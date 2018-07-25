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

	"github.com/Sherlock-Holo/camouflage/ca"
	"github.com/Sherlock-Holo/camouflage/config"
	websocket2 "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/libsocks"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
)

const poolCachedSize = 1

type Client struct {
	listener net.Listener

	wsURL    string
	wsDialer websocket.Dialer

	pool     *baseHeap
	poolLock sync.Mutex

	maxLinks int
}

func New(cfg config.Client) (*Client, error) {
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

		wsURL:    wsURL,
		wsDialer: dialer,

		pool: new(baseHeap),

		maxLinks: cfg.MaxLinks,
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

func (c *Client) newLink() (*link.Link, *base, error) {
	return c.realNewLink(0)
}

func (c *Client) realNewLink(count int) (*link.Link, *base, error) {
	if count > 10 {
		return nil, nil, errors.New("new Link failed")
	}

	c.poolLock.Lock()
	for c.pool.Len() > 0 {
		base := heap.Pop(c.pool).(*base)

		// base is closed
		if base.manager.IsClosed() {
			continue
		}

		if base.count >= int32(c.maxLinks) {
			heap.Push(c.pool, base)
			break
		}

		l, err := base.manager.NewLink()
		if err != nil {
			go base.manager.Close()
			c.poolLock.Unlock()

			return c.realNewLink(count + 1)
		}

		base.count++
		heap.Push(c.pool, base)
		c.poolLock.Unlock()

		return l, base, nil
	}
	c.poolLock.Unlock()

	conn, _, err := c.wsDialer.Dial(c.wsURL, nil)
	if err != nil {
		return c.realNewLink(count + 1)
	}

	base := &base{
		manager: link.NewManager(websocket2.NewWrapper(conn), link.KeepaliveConfig),
		count:   1,
	}

	l, err := base.manager.NewLink()
	if err != nil {
		go base.manager.Close()
		return c.realNewLink(count + 1)
	}

	c.poolLock.Lock()
	heap.Push(c.pool, base)
	c.poolLock.Unlock()

	return l, base, nil
}

func (c *Client) handle(conn net.Conn) {
	socks, err := libsocks.NewSocks(conn, nil)
	if err != nil {
		log.Println("new socks:", err)
		conn.Close()
		return
	}

	socksLocalAddr := socks.LocalAddr().(*net.TCPAddr)

	l, base, err := c.newLink()
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
		if base.index != -1 {
			// base is closed
			if base.manager.IsClosed() {
				heap.Remove(c.pool, base.index)

			} else {
				base.count--

				// check it should remove base or not
				if base.count == 0 {
					if c.pool.Len() > poolCachedSize {
						go base.manager.Close()
						heap.Remove(c.pool, base.index)
					} else {
						heap.Fix(c.pool, base.index)
					}

				} else {
					heap.Fix(c.pool, base.index)
				}
			}
		}
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
		if base.index != -1 {
			// base is closed
			if base.manager.IsClosed() {
				heap.Remove(c.pool, base.index)

			} else {
				base.count--

				// check it should remove base or not
				if base.count == 0 {
					if c.pool.Len() > poolCachedSize {
						go base.manager.Close()
						heap.Remove(c.pool, base.index)
					} else {
						heap.Fix(c.pool, base.index)
					}

				} else {
					heap.Fix(c.pool, base.index)
				}
			}
		}
		c.poolLock.Unlock()
		return
	}

	var (
		closeCount = make(chan struct{}, 2)
		die        = make(chan struct{})
		once       = sync.Once{}
	)

	go func() {
		if _, err := io.Copy(l, socks); err != nil {
			log.Println(err)

			select {
			case <-die:
			default:
				once.Do(func() {
					close(die)
				})
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
				once.Do(func() {
					close(die)
				})
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
	if base.index != -1 {
		// base is closed
		if base.manager.IsClosed() {
			heap.Remove(c.pool, base.index)

		} else {
			base.count--

			// check it should remove base or not
			if base.count == 0 {
				if c.pool.Len() > poolCachedSize {
					go base.manager.Close()
					heap.Remove(c.pool, base.index)
				} else {
					heap.Fix(c.pool, base.index)
				}

			} else {
				heap.Fix(c.pool, base.index)
			}
		}
	}
	c.poolLock.Unlock()
	return
}
