package client

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

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
	}).String()

	tlsConfig := new(tls.Config)

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

	if cfg.Timeout.Duration > 0 {
		dialer.HandshakeTimeout = cfg.Timeout.Duration
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
			log.Printf("accept failed: %v", errors.WithStack(err))
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

func (c *Client) reconnect() (err error) {
	if c.manager != nil {
		c.manager.Close()
	}

	var (
		conn *websocket.Conn
		resp *http.Response
	)
	for i := 0; i < 2; i++ {
		code, err := utils.GenCode(c.config.Secret, c.config.Period)
		if err != nil {
			return errors.Wrap(err, "connect server failed")
		}

		httpHeader := http.Header{}
		httpHeader.Set("totp-code", code)

		conn, resp, err = c.wsDialer.Dial(c.wsURL, httpHeader)

		switch errors.Cause(err) {
		case websocket.ErrBadHandshake:
			resp.Body.Close()

			if resp.StatusCode == http.StatusForbidden {
				if i == 1 {
					return errors.New("maybe TOTP secret is wrong")
				} else {
					continue
				}
			}
			return errors.Wrap(err, "connect server failed")

		default:
			return errors.Wrap(err, "connect server failed")

		case nil:
		}
	}

	linkCfg := link.DefaultConfig(link.ClientMode)
	linkCfg.KeepaliveInterval = 5 * time.Second

	c.manager = link.NewManager(wsWrapper.NewWrapper(conn), linkCfg)
	return nil
}

func (c *Client) handle(conn net.Conn) {
	socks, err := NewSocks(conn)
	if err != nil {
		log.Printf("%v", err)
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
		log.Printf("dial link failed: %+v", err)

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
		io.Copy(l, socks)
		socks.Close()
		l.Close()
	}()

	go func() {
		io.Copy(socks, l)
		socks.Close()
		l.Close()
	}()
}
