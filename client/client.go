package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/Sherlock-Holo/camouflage/config/client"
	"github.com/Sherlock-Holo/camouflage/log"
	"github.com/Sherlock-Holo/camouflage/utils"
	wsWrapper "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/libsocks"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
	"golang.org/x/xerrors"
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
		log.Fatalf("%+v", xerrors.Errorf("local listen failed: %w", err))
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
			log.Fatalf("%+v", xerrors.Errorf("read ca cert failed: %w", err))
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
			log.Warnf("%+v", xerrors.Errorf("connect to server failed: %w", err))
			continue
		}
		break
	}

	for {
		conn, err := c.listener.Accept()
		if err != nil {
			log.Errorf("%v", xerrors.Errorf("accept failed: %w", err))
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
			return xerrors.Errorf("reconnect failed: %w", err)
		}

		httpHeader := http.Header{}
		httpHeader.Set("totp-code", code)

		conn, resp, err = c.wsDialer.Dial(c.wsURL, httpHeader)

		switch {
		case xerrors.Is(err, websocket.ErrBadHandshake):
			resp.Body.Close()

			if resp.StatusCode == http.StatusForbidden {
				if i == 1 {
					return xerrors.New("reconnect failed: maybe TOTP secret is wrong")
				} else {
					continue
				}
			}
			return xerrors.Errorf("reconnect failed: %w", err)

		default:
			return xerrors.Errorf("reconnect failed: %w", err)

		case err == nil:
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
		log.Errorf("%v", xerrors.Errorf("client handle error: %w", err))
		return
	}

	connReq := &connRequest{
		Socks:   socks,
		Success: make(chan link.Link, 1),
		Err:     make(chan error, 1),
	}

	timeoutCtx := context.Background()
	if c.config.Timeout.Duration > 0 {
		var cancel context.CancelFunc
		timeoutCtx, cancel = context.WithTimeout(context.Background(), c.config.Timeout.Duration)
		defer cancel()
	}

	select {
	case <-timeoutCtx.Done():
		log.Warnf("dial queue is full")
		socks.Handshake(libsocks.TTLExpired)
		socks.Close()
		return

	case c.connReqChan <- connReq:
	}

	var l link.Link

	select {
	case err := <-connReq.Err:
		log.Errorf("client handle error: %+v", err)

		switch {
		case xerrors.Is(err, link.ErrTimeout):
			socks.Handshake(libsocks.TTLExpired)

		case xerrors.Is(err, link.ErrManagerClosed):
			socks.Handshake(libsocks.NetworkUnreachable)

		default:
			socks.Handshake(libsocks.ServerFailed)
		}

		socks.Close()
		return

	case l = <-connReq.Success:
		if err := socks.Handshake(libsocks.Success); err != nil {
			log.Errorf("%+v", xerrors.Errorf("client handle error: %w", err))
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
