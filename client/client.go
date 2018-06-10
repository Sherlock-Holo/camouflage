package client

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "io"
    "log"
    "net"
    "net/url"
    "sync/atomic"

    "github.com/Sherlock-Holo/camouflage/config"
    websocket2 "github.com/Sherlock-Holo/goutils/websocket"
    "github.com/Sherlock-Holo/libsocks"
    "github.com/Sherlock-Holo/link"
    "github.com/gorilla/websocket"
)

const maxLinks = 100

type managerStatus struct {
    count  int32
    usable bool
}

type Client struct {
    listener net.Listener
    wsURL    string
    managers map[*link.Manager]*managerStatus
    dialer   websocket.Dialer
}

func NewClient(cfg config.Client) (*Client, error) {
    listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.SocksAddr, cfg.SocksPort))
    if err != nil {
        return nil, err
    }

    wsURL := url.URL{
        Scheme: "wss",
        Host:   fmt.Sprintf("%s:%d", cfg.RemoteAddr, cfg.RemotePort),
        Path:   cfg.Path,
    }.String()

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
        wsURL:    wsURL,
        managers: make(map[*link.Manager]*managerStatus),
        dialer:   dialer,
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
        manager *link.Manager
        links   = len(c.managers)
    )

RANDOM:
    for i := 0; i < links; i++ {
        for m, status := range c.managers {
            if !status.usable {
                // delete usable manager
                c.deleteManager(m)
                break
            }

            if atomic.LoadInt32(&status.count) < maxLinks {
                manager = m
                atomic.AddInt32(&status.count, 1)
                break RANDOM
            }
            break
        }
    }

    if manager == nil {
        conn, _, err := c.dialer.Dial(c.wsURL, nil)
        if err != nil {
            log.Println("dial websocket:", err)
            socks.Reply(socksLocalAddr.IP, uint16(socksLocalAddr.Port), libsocks.NetworkUnreachable)
            socks.Close()
            return
        }

        manager = link.NewManager(websocket2.NewWrapper(conn))

        c.managers[manager] = &managerStatus{
            count:  0,
            usable: true,
        }
    }

    localAddr := conn.LocalAddr().(*net.TCPAddr)

    l, err := manager.NewLink()
    if err != nil {
        log.Println("newLink:", err)
        socks.Reply(socksLocalAddr.IP, uint16(socksLocalAddr.Port), libsocks.ServerFailed)
        socks.Close()
        manager.Close()
        // delete broken manager
        c.deleteManager(manager)
        return
    }

    atomic.AddInt32(&c.managers[manager].count, 1)

    if _, err := l.Write(socks.Target.Bytes()); err != nil {
        log.Println("send target:", err)
        socks.Reply(socksLocalAddr.IP, uint16(socksLocalAddr.Port), libsocks.ServerFailed)
        socks.Close()
        l.Close()
        atomic.AddInt32(&c.managers[manager].count, -1)
        return
    }

    // socks reply
    if err := socks.Reply(localAddr.IP, uint16(localAddr.Port), libsocks.Success); err != nil {
        log.Println("socks reply", err)
        socks.Close()
        l.Close()
        atomic.AddInt32(&c.managers[manager].count, -1)
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
            atomic.AddInt32(&c.managers[manager].count, -1)
            return

        case <-closeCount:
        }
    }
}

func (c *Client) deleteManager(m *link.Manager) {
    if status, ok := c.managers[m]; ok {
        status.usable = false
        delete(c.managers, m)
    }
}
