package client

import (
	"container/heap"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/Sherlock-Holo/camouflage/ca"
	"github.com/Sherlock-Holo/camouflage/config/client"
	"github.com/Sherlock-Holo/camouflage/frontend"
	websocket2 "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/link"
	"github.com/Sherlock-Holo/streamencrypt"
	"github.com/gorilla/websocket"
)

const poolCachedSize = 1

type Client struct {
	listeners map[frontend.Type][]ListenerInfo

	wsURL    string
	wsDialer websocket.Dialer

	pool     *baseHeap
	poolLock sync.Mutex

	maxLinks int

	monitor *Monitor
}

func New(cfg *client.Client) (*Client, error) {
	var (
		shadowsocksListeners []ListenerInfo

		socksListeners []ListenerInfo
	)

	cipherInfo := streamencrypt.Ciphers[streamencrypt.CHACHA20_IETF]

	for _, ssCfg := range cfg.Shadowsocks {
		listener, err := net.Listen("tcp", net.JoinHostPort(ssCfg.ListenAddr, strconv.Itoa(ssCfg.ListenPort)))
		if err != nil {
			return nil, err
		}

		shadowsocksListeners = append(shadowsocksListeners, ListenerInfo{
			Key:      streamencrypt.EvpBytesToKey(ssCfg.Key, cipherInfo.KeyLen),
			Listener: listener,
		})
	}

	for _, socksCfg := range cfg.Socks {
		listener, err := net.Listen("tcp", net.JoinHostPort(socksCfg.ListenAddr, strconv.Itoa(socksCfg.ListenPort)))
		if err != nil {
			return nil, err
		}

		socksListeners = append(socksListeners, ListenerInfo{Key: nil, Listener: listener})
	}

	wsURL := (&url.URL{
		Scheme: "wss",
		Host:   net.JoinHostPort(cfg.RemoteAddr, strconv.Itoa(cfg.RemotePort)),
		Path:   cfg.Path,
	}).String()

	certificate, err := tls.LoadX509KeyPair(cfg.Crt, cfg.Key)
	if err != nil {
		return nil, err
	}

	caFile, err := os.Open(cfg.CaCrt)
	if err != nil {
		return nil, err
	}
	defer caFile.Close()

	CA, err := ioutil.ReadAll(caFile)
	if err != nil {
		return nil, err
	}

	pool, err := ca.InitCAPool(CA)
	if err != nil {
		return nil, fmt.Errorf("Init CA Pool: %s\n", err)
	}

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			RootCAs:      pool,
			Certificates: []tls.Certificate{certificate},
		},
	}

	var monitor *Monitor

	if cfg.MonitorPort != 0 && cfg.MonitorAddr != "" {
		monitor = new(Monitor)
		if err = monitor.start(cfg.MonitorAddr, cfg.MonitorPort); err != nil {
			log.Println(err)
			monitor = nil
		}
	}

	listeners := make(map[frontend.Type][]ListenerInfo)

	for _, ssListener := range shadowsocksListeners {
		listeners[frontend.SHADOWSOCKS_CHACHA20_IETF] = append(listeners[frontend.SHADOWSOCKS_CHACHA20_IETF], ssListener)
	}

	for _, socksListener := range socksListeners {
		listeners[frontend.SOCKS] = append(listeners[frontend.SOCKS], socksListener)
	}

	return &Client{
		listeners: listeners,

		wsURL:    wsURL,
		wsDialer: dialer,

		pool: new(baseHeap),

		maxLinks: cfg.MaxLinks,

		monitor: monitor,
	}, nil
}

func (c *Client) Run() {
	for frontendType := range c.listeners {
		for _, listener := range c.listeners[frontendType] {
			go func(frontendType frontend.Type, listener ListenerInfo) {
				for {
					conn, err := listener.Listener.Accept()
					if err != nil {
						log.Println(err)
						continue
					}

					go c.handle(conn, frontendType, listener.Key)
				}
			}(frontendType, listener)
		}
	}
	<-make(chan struct{})
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
			// report to monitor
			if c.monitor != nil {
				atomic.AddInt32(&c.monitor.baseConnections, -1)
			}
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

			// report to monitor
			if c.monitor != nil {
				atomic.AddInt32(&c.monitor.baseConnections, -1)
			}

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

	// report to monitor
	if c.monitor != nil {
		atomic.AddInt32(&c.monitor.baseConnections, 1)
	}

	return l, base, nil
}

func (c *Client) handle(conn net.Conn, frontendType frontend.Type, key []byte) {
	fe, err := frontend.Frontends[frontendType](conn, key)
	if err != nil {
		log.Println(err)
		conn.Close()
		return
	}

	l, base, err := c.newLink()
	if err != nil {
		log.Println(err)
		fe.Handshake(false)
		fe.Close()
		return
	}

	if _, err := l.Write(fe.Target()); err != nil {
		log.Println("send target:", err)
		fe.Handshake(false)
		fe.Close()
		l.Close()

		c.errorHandle(fe, l, base)
		return
	}

	if fe.Handshake(true) != nil {
		log.Println("handshake failed")

		fe.Close()
		l.Close()

		c.errorHandle(fe, l, base)
		return
	}

	var (
		closeCount = make(chan struct{}, 2)
		die        = make(chan struct{})
		once       = sync.Once{}
	)

	// report to monitor
	if c.monitor != nil {
		atomic.AddInt32(&c.monitor.tcpConnections, 1)
	}

	go func() {
		if _, err := io.Copy(l, fe); err != nil {
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
		fe.CloseRead()
		closeCount <- struct{}{}
	}()

	go func() {
		if _, err := io.Copy(fe, l); err != nil {
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

		fe.CloseWrite()
		closeCount <- struct{}{}
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-die:
			break

		case <-closeCount:
		}
	}

	fe.Close()
	l.Close()

	// report to monitor
	if c.monitor != nil {
		atomic.AddInt32(&c.monitor.tcpConnections, -1)
	}

	c.errorHandle(fe, l, base)
	return
}

func (c *Client) errorHandle(socks, link io.ReadWriteCloser, base *base) {
	c.poolLock.Lock()
	if base.index != -1 {
		// base is closed
		if base.manager.IsClosed() {
			heap.Remove(c.pool, base.index)

			// report to monitor
			if c.monitor != nil {
				atomic.AddInt32(&c.monitor.baseConnections, -1)
			}

			c.poolLock.Unlock()
			return
		}

		base.count--

		// check it should remove base or not
		if base.count == 0 {
			if c.pool.Len() > poolCachedSize {
				go base.manager.Close()
				heap.Remove(c.pool, base.index)

				// report to monitor
				if c.monitor != nil {
					atomic.AddInt32(&c.monitor.baseConnections, -1)
				}

			} else {
				heap.Fix(c.pool, base.index)
			}

		} else {
			heap.Fix(c.pool, base.index)
		}
	}
	c.poolLock.Unlock()
}
