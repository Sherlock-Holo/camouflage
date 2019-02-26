package client

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"sync"

	"github.com/Sherlock-Holo/camouflage/config/client"
	wsWrapper "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

type Client struct {
	listener    net.Listener
	config      *client.Config
	wsURL       string
	wsDialer    websocket.Dialer
	manager     *link.Manager
	managerLock sync.RWMutex
}

func New(cfg *client.Config) (*Client, error) {
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		log.Fatalf("local listen failed: %+v", errors.WithStack(err))
	}

	wsURL := (&url.URL{
		Scheme: "wss",
		Host:   cfg.RemoteAddr,
		Path:   cfg.Path,
	}).String()

	certificate, err := tls.LoadX509KeyPair(cfg.Crt, cfg.Key)
	if err != nil {
		log.Fatalf("read key pair failed: %+v", errors.WithStack(err))
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
	}

	if cfg.DebugCA != "" {
		certPool := x509.NewCertPool()

		caBytes, err := ioutil.ReadFile(cfg.DebugCA)
		if err != nil {
			log.Fatalf("read ca cert failed: %+v", errors.WithStack(err))
		}

		certPool.AppendCertsFromPEM(caBytes)

		tlsConfig.RootCAs = certPool
	}

	dialer := websocket.Dialer{
		TLSClientConfig: tlsConfig,
	}

	return &Client{
		listener: listener,
		wsURL:    wsURL,
		wsDialer: dialer,
		config:   cfg,
	}, nil
}

func (c *Client) Run() {
	for {
		if err := c.reconnect(); err != nil {
			log.Printf("connect to server failed: %v", errors.WithStack(err))
			continue
		}
		break
	}

	for {
		conn, err := c.listener.Accept()
		if err != nil {
			log.Printf("accept failed: %s", errors.WithStack(err))
			continue
		}

		go c.handle(conn)
	}
}

func (c *Client) reconnect() error {
	if c.manager != nil {
		c.manager.Close()
	}

	conn, _, err := c.wsDialer.Dial(c.wsURL, nil)
	if err != nil {
		return errors.WithStack(err)
	}
	c.manager = link.NewManager(wsWrapper.NewWrapper(conn), link.KeepaliveConfig())
	return nil
}

func (c *Client) handle(conn net.Conn) {
	socks, err := NewSocks(conn)
	if err != nil {
		log.Printf("new socks failed: %v", err)
		return
	}

	var l io.ReadWriteCloser

	c.managerLock.RLock()
	if c.manager.IsClosed() {
		c.managerLock.RUnlock()

		c.managerLock.Lock()
		if c.manager.IsClosed() {
			if err := c.reconnect(); err != nil {
				log.Printf("reconnect failed: %v", err)
				c.managerLock.Unlock()
				socks.Close()
				return
			}

			l, err = c.manager.Dial()
			if err != nil {
				log.Printf("dial failed: %v", errors.WithStack(err))
				c.managerLock.Unlock()
				socks.Close()
				return
			}
			c.managerLock.Unlock()
		} else {
			l, err = c.manager.Dial()
			if err != nil {
				log.Printf("dial failed: %v", errors.WithStack(err))
				c.managerLock.Unlock()
				socks.Close()
				return
			}
			c.managerLock.Unlock()
		}

	} else {
		l, err = c.manager.Dial()
		if err != nil {
			log.Printf("dial failed: %v", errors.WithStack(err))
			c.managerLock.RUnlock()
			socks.Close()
			return
		}
		c.managerLock.RUnlock()
	}

	if _, err := l.Write(socks.Target()); err != nil {
		log.Printf("send target failed: %v", err)
		socks.Handshake(false)
		socks.Close()
		l.Close()
		return
	}

	if err := socks.Handshake(true); err != nil {
		log.Printf("handshake failed: %v", err)
		socks.Close()
		l.Close()
		return
	}

	go func() {
		if _, err := io.Copy(l, socks); err != nil {
			log.Println(err)
		}
		socks.Close()
		l.Close()
	}()

	go func() {
		if _, err := io.Copy(socks, l); err != nil {
			log.Println(err)
		}
		socks.Close()
		l.Close()
	}()
}
