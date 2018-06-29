package client

import (
	"container/heap"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/Sherlock-Holo/camouflage/ca"
	"github.com/Sherlock-Holo/camouflage/config"
	websocket2 "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/libsocks"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
)

type managerStatus struct {
	manager *link.Manager
	count   int32

	closed chan struct{}

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

	maxLinks int

	dialer websocket.Dialer
}

func NewClient(cfg config.Client) (*Client, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.SocksAddr, cfg.SocksPort))
	if err != nil {
		return nil, err
	}

	wsURL := (&url.URL{
		Scheme: "wss",
		Host:   net.JoinHostPort(cfg.RemoteAddr, strconv.Itoa(cfg.RemotePort)),
		Path:   cfg.Path,
	}).String()

	certificate, err := tls.LoadX509KeyPair(cfg.CrtFile, cfg.KeyFile)
	if err != nil {
		return nil, err
	}

	pool, err := ca.InitCAPool(cfg.CA)
	if err != nil {
		return nil, fmt.Errorf("Init CA Pool: %s\n", err)
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

		maxLinks: cfg.MaxLinks,

		dialer: dialer,
	}, nil
}

func (c *Client) Run() {
	go c.clean()

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
		conn.Close()
		return
	}

	socksLocalAddr := socks.LocalAddr().(*net.TCPAddr)

	l, status, err := c.newLink()
	if err != nil {
		log.Println(err)
		socks.Reply(socksLocalAddr.IP, uint16(socksLocalAddr.Port), libsocks.ServerFailed)
		socks.Close()
		return
	}

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

	localAddr := conn.LocalAddr().(*net.TCPAddr)

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
			break

		case <-closeCount:
		}
	}

	socks.Close()
	l.Close()

	c.poolLock.Lock()
	if status.manager.IsClosed() {
		select {
		case <-status.closed:
		default:
			close(status.closed)
		}

		heap.Remove(c.managerPool, status.index)
	}

	status.count--
	heap.Fix(c.managerPool, status.index)

	c.poolLock.Unlock()
}

func (c *Client) clean() {
	ticker := time.NewTicker(15 * time.Second)

	for {
		<-ticker.C

		var tmp []*managerStatus

		c.poolLock.Lock()
		var cleaned int

		for {
			if c.managerPool.Len() <= 1 {
				break
			}

			status := heap.Pop(c.managerPool).(*managerStatus)

			switch {
			case status.count == 0:
				cleaned++
				go status.manager.Close()

			case status.manager.IsClosed():
				cleaned++
				go status.manager.Close()

			default:
				tmp = append(tmp, status)
			}
		}

		if tmp != nil {
			pool := managerHeap(tmp)
			c.managerPool = &pool

			heap.Init(c.managerPool)
		}

		c.poolLock.Unlock()

		if cleaned > 0 {
			log.Printf("clean %d useless manager(s)", cleaned)
		}
	}
}

func (c *Client) newLink() (*link.Link, *managerStatus, error) {
	return c.realNewLink(1)
}

func (c *Client) realNewLink(count int) (*link.Link, *managerStatus, error) {
	if count >= 10 {
		return nil, nil, errors.New("new Link failed")
	}

	c.poolLock.Lock()
	for c.managerPool.Len() > 0 {
		status := heap.Pop(c.managerPool).(*managerStatus)

		select {
		case <-status.closed:
			continue
		default:
		}

		if status.count >= int32(c.maxLinks) {
			heap.Push(c.managerPool, status)
			break
		}

		status.count++
		heap.Push(c.managerPool, status)
		c.poolLock.Unlock()

		l, err := status.manager.NewLink()
		if err != nil {
			c.poolLock.Lock()
			go status.manager.Close()

			select {
			case <-status.closed:
			default:
				close(status.closed)
			}

			heap.Remove(c.managerPool, status.index)
			return c.realNewLink(count + 1)
		}

		return l, status, nil
	}
	c.poolLock.Unlock()

	conn, _, err := c.dialer.Dial(c.wsURL, nil)
	if err != nil {
		return c.realNewLink(count + 1)
	}

	status := &managerStatus{
		manager: link.NewManager(websocket2.NewWrapper(conn), link.KeepaliveConfig),
		count:   1,
		closed:  make(chan struct{}),
	}

	l, err := status.manager.NewLink()
	if err != nil {
		go status.manager.Close()
		close(status.closed)
		return c.realNewLink(count + 1)
	}

	c.poolLock.Lock()
	heap.Push(c.managerPool, status)
	c.poolLock.Unlock()

	return l, status, nil
}
