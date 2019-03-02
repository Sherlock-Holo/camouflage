package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/url"

	"github.com/Sherlock-Holo/camouflage/config/client"
	"github.com/Sherlock-Holo/camouflage/utils"
	wsWrapper "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/libsocks"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

type Client struct {
	listener    net.Listener
	config      *client.Config
	wsURL       string
	wsDialer    websocket.Dialer
	manager     link.Manager
	connReqChan chan *connRequest
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
		MinVersion:   tls.VersionTLS12,
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

	netDialer := net.Dialer{}
	dialer := websocket.Dialer{
		TLSClientConfig: tlsConfig,
		NetDialContext: func(_ context.Context, network, addr string) (conn net.Conn, err error) {
			return netDialer.DialContext(utils.TimeoutCtx(cfg.Timeout.Duration), network, addr)
		},
	}

	cl := &Client{
		listener:    listener,
		wsURL:       wsURL,
		wsDialer:    dialer,
		config:      cfg,
		connReqChan: make(chan *connRequest, 100),
	}

	go cl.managerLoop()

	return cl, nil
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

func (c *Client) managerLoop() {
	for connReq := range c.connReqChan {
		socks := connReq.Socks

		if c.manager.IsClosed() {
			if err := c.reconnect(); err != nil {
				connReq.Err <- err
				continue
			}
		}

		l, err := c.manager.DialData(utils.TimeoutCtx(c.config.Timeout.Duration), socks.Target())
		if err != nil {
			connReq.Err <- err
			continue
		}

		connReq.Success <- l
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
	c.manager = link.NewManager(wsWrapper.NewWrapper(conn), link.KeepaliveConfig(link.ClientMode))
	return nil
}

func (c *Client) handle(conn net.Conn) {
	socks, err := NewSocks(conn)
	if err != nil {
		log.Printf("new socks failed: %v", err)
		conn.Close()
		return
	}

	connReq := &connRequest{
		Socks:   socks,
		Success: make(chan link.Link, 1),
		Err:     make(chan error, 1),
	}

	c.connReqChan <- connReq

	var l link.Link

	select {
	case err := <-connReq.Err:
		log.Printf("connect failed: %+v", err)

		switch errors.Cause(err) {
		case link.ErrTimeout:
			socks.Handshake(libsocks.TTLExpired)

		case link.ErrManagerClosed:
			socks.Handshake(libsocks.NetworkUnreachable)

		default:
			socks.Handshake(libsocks.ServerFailed)
		}

		socks.Close()
		return

	case l = <-connReq.Success:
		if err := socks.Handshake(libsocks.Success); err != nil {
			log.Printf("socks handshake failed: %+v", err)
			socks.Close()
			l.Close()
			return
		}
	}

	go func() {
		/*if _, err := io.Copy(l, socks); err != nil {
			// log.Println(err)
		}*/
		io.Copy(l, socks)
		socks.Close()
		l.Close()
	}()

	go func() {
		/*if _, err := io.Copy(socks, l); err != nil {
			// log.Println(err)
		}*/
		io.Copy(socks, l)
		socks.Close()
		l.Close()
	}()
}
