package client

import (
    "container/heap"
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "io"
    "log"
    "net"
    "net/url"
    "sync"

    "github.com/Sherlock-Holo/camouflage/config"
    websocket2 "github.com/Sherlock-Holo/goutils/websocket"
    "github.com/Sherlock-Holo/libsocks"
    "github.com/Sherlock-Holo/link"
    "github.com/gorilla/websocket"
)

const maxLinks = 100

type managerStatus struct {
    manager *link.Manager
    count   int32
    usable  bool

    index int
}

type managerHeap []*managerStatus

func (h *managerHeap) Len() int {
    return len(*h)
}

func (h *managerHeap) Less(i, j int) bool {
    return (*h)[i].count < (*h)[j].count
}

func (h *managerHeap) Swap(i, j int) {
    (*h)[i], (*h)[j] = (*h)[j], (*h)[i]
    (*h)[i].index, (*h)[j].index = i, j
}

func (h *managerHeap) Push(x interface{}) {
    st := x.(*managerStatus)
    st.index = len(*h)
    *h = append(*h, st)
}

func (h *managerHeap) Pop() interface{} {
    old := *h

    status := old[len(old)-1]
    status.index = -1

    *h = old[:len(old)-1]

    return status
}

type Client struct {
    listener net.Listener

    wsURL string

    managerPool *managerHeap
    poolLock    sync.Mutex

    dialer websocket.Dialer
}

func NewClient(cfg config.Client) (*Client, error) {
    listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.SocksAddr, cfg.SocksPort))
    if err != nil {
        return nil, err
    }

    wsURL := (&url.URL{
        Scheme: "wss",
        Host:   fmt.Sprintf("%s:%d", cfg.RemoteAddr, cfg.RemotePort),
        Path:   cfg.Path,
    }).String()

    pool := x509.NewCertPool()

    pool.AppendCertsFromPEM(cfg.CA)

    certificate, err := tls.LoadX509KeyPair(cfg.CrtFile, cfg.KeyFile)
    if err != nil {
        return nil, err
    }

    dialer := websocket.Dialer{
        TLSClientConfig: &tls.Config{
            RootCAs:      pool,
            Certificates: []tls.Certificate{certificate},
        },
    }

    return &Client{
        listener: listener,

        wsURL: wsURL,

        managerPool: new(managerHeap),

        dialer: dialer,
    }, nil
}

func (c *Client) Run() {
    for {
        conn, err := c.listener.Accept()
        if err != nil {
            log.Println(err)
            continue
        }

        go c.handle(conn)
    }
}

func (c *Client) handle(conn net.Conn) {
    socks, err := libsocks.NewSocks(conn, nil)
    if err != nil {
        log.Println("new socks:", err)
        socks.Close()
        return
    }

    socksLocalAddr := socks.LocalAddr().(*net.TCPAddr)

    var (
        status *managerStatus
        inPool bool
    )

    c.poolLock.Lock()
    for c.managerPool.Len() > 0 {
        st := heap.Pop(c.managerPool).(*managerStatus)
        if !st.usable {
            continue
        }

        if st.count >= maxLinks {
            heap.Push(c.managerPool, st)
            break
        }

        status = st
        status.count++
        heap.Push(c.managerPool, status)
        inPool = true
        break
    }
    c.poolLock.Unlock()

    if !inPool {
        conn, _, err := c.dialer.Dial(c.wsURL, nil)
        if err != nil {
            log.Println("dial websocket:", err)
            socks.Reply(socksLocalAddr.IP, uint16(socksLocalAddr.Port), libsocks.NetworkUnreachable)
            socks.Close()
            return
        }

        status = &managerStatus{
            manager: link.NewManager(websocket2.NewWrapper(conn)),
            count:   1,
            usable:  true,
        }
    }

    localAddr := conn.LocalAddr().(*net.TCPAddr)

    l, err := status.manager.NewLink()
    if err != nil {
        log.Println("newLink:", err)
        socks.Reply(socksLocalAddr.IP, uint16(socksLocalAddr.Port), libsocks.ServerFailed)
        socks.Close()
        status.manager.Close()

        if inPool {
            c.poolLock.Lock()
            heap.Remove(c.managerPool, status.index)
            c.poolLock.Unlock()
        }

        return
    }

    c.poolLock.Lock()
    heap.Push(c.managerPool, status)
    c.poolLock.Unlock()

    if _, err := l.Write(socks.Target.Bytes()); err != nil {
        log.Println("send target:", err)
        socks.Reply(socksLocalAddr.IP, uint16(socksLocalAddr.Port), libsocks.ServerFailed)
        socks.Close()
        l.Close()

        c.poolLock.Lock()
        status.count--
        heap.Fix(c.managerPool, status.index)
        c.poolLock.Unlock()

        return
    }

    // socks reply
    if err := socks.Reply(localAddr.IP, uint16(localAddr.Port), libsocks.Success); err != nil {
        log.Println("socks reply", err)
        socks.Close()
        l.Close()

        c.poolLock.Lock()
        status.count--
        heap.Fix(c.managerPool, status.index)
        c.poolLock.Unlock()

        return
    }

    var (
        closeCount = make(chan struct{}, 2)
        die        = make(chan struct{})
    )

    go func() {
        if _, err := io.Copy(l, socks); err != nil {
            log.Println(err)
            socks.Close()
            l.Close()
            select {
            case <-die:
            default:
                close(die)
            }
            return
        }

        l.CloseWrite()
        socks.CloseRead()
        closeCount <- struct{}{}
    }()

    go func() {
        if _, err := io.Copy(socks, l); err != nil {
            log.Println(err)
            socks.Close()
            l.Close()
            select {
            case <-die:
            default:
                close(die)
            }
            return
        }

        socks.CloseWrite()
        closeCount <- struct{}{}
    }()

    for i := 0; i < 2; i++ {
        select {
        case <-die:
            c.poolLock.Lock()
            status.count--
            heap.Fix(c.managerPool, status.index)
            c.poolLock.Unlock()

            return

        case <-closeCount:
        }
    }
}
