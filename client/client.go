package client

import (
	"container/heap"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"

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

	token string

	pool     *baseHeap
	poolLock sync.Mutex

	maxLinks int

	monitor *Monitor
}

func New(cfg *client.Client) (*Client, error) {
	var (
		shadowsocksListeners []ListenerInfo
		socksListeners       []ListenerInfo
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

		socksListeners = append(socksListeners, ListenerInfo{
			Key:      nil,
			Listener: listener,
		})
	}

	wsURL := (&url.URL{
		Scheme: "wss",
		Host:   net.JoinHostPort(cfg.RemoteAddr, strconv.Itoa(cfg.RemotePort)),
		Path:   cfg.Path,
	}).String()

	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	if cfg.SelfCA != "" {
		selfCA, err := ioutil.ReadFile(cfg.SelfCA)
		if err != nil {
			return nil, err
		}
		pool, err = ca.InitCAPool(selfCA)
		if err != nil {
			return nil, err
		}
	}

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}

	var monitor *Monitor

	if cfg.MonitorPort != 0 && cfg.MonitorAddr != "" {
		log.Printf("start monitor on %s", net.JoinHostPort(cfg.MonitorAddr, strconv.Itoa(cfg.MonitorPort)))
		monitor = new(Monitor)
		monitor.start(cfg.MonitorAddr, cfg.MonitorPort)
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

		token: cfg.Token,

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

func (c *Client) Dial() (*link.Link, *base, error) {
	return c.realDial(0)
}

func (c *Client) realDial(count int) (*link.Link, *base, error) {
	if count > 10 {
		return nil, nil, errors.New("new Link failed")
	}

	c.poolLock.Lock()
	for c.pool.Len() > 0 {
		base := heap.Pop(c.pool).(*base)

		// base is closed
		if base.manager.IsClosed() {
			// report to Monitor
			if c.monitor != nil {
				c.monitor.updateMonitor(0, -1)
			}
			continue
		}

		if base.count >= int32(c.maxLinks) {
			heap.Push(c.pool, base)
			break
		}

		l, err := base.manager.Dial()
		if err != nil {
			go base.manager.Close()
			c.poolLock.Unlock()

			// report to Monitor
			if c.monitor != nil {
				c.monitor.updateMonitor(0, -1)
			}

			return c.realDial(count + 1)
		}

		base.count++
		heap.Push(c.pool, base)
		c.poolLock.Unlock()

		return l, base, nil
	}
	c.poolLock.Unlock()

	httpHeader := http.Header{}
	httpHeader.Add("token", c.token)
	conn, _, err := c.wsDialer.Dial(c.wsURL, httpHeader)
	if err != nil {
		return c.realDial(count + 1)
	}

	base := &base{
		manager: link.NewManager(websocket2.NewWrapper(conn), link.KeepaliveConfig()),
		count:   1,
	}

	l, err := base.manager.Dial()
	if err != nil {
		go base.manager.Close()
		return c.realDial(count + 1)
	}

	c.poolLock.Lock()
	heap.Push(c.pool, base)
	c.poolLock.Unlock()

	// report to Monitor
	if c.monitor != nil {
		c.monitor.updateMonitor(0, 1)
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

	l, base, err := c.Dial()
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

	// report to Monitor
	if c.monitor != nil {
		c.monitor.updateMonitor(1, 0)
	}

	go func() {
		if _, err := io.Copy(l, fe); err != nil {
			log.Println(err)
		}
		fe.Close()
		l.Close()
	}()

	go func() {
		if _, err := io.Copy(fe, l); err != nil {
			log.Println(err)
		}
		fe.Close()
		l.Close()
	}()

	// report to Monitor
	if c.monitor != nil {
		c.monitor.updateMonitor(-1, 0)
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

			// report to Monitor
			if c.monitor != nil {
				c.monitor.updateMonitor(0, -1)
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

				// report to Monitor
				if c.monitor != nil {
					c.monitor.updateMonitor(0, -1)
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
