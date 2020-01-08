package client

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"net/url"

	"github.com/Sherlock-Holo/camouflage/config/client"
	"github.com/Sherlock-Holo/camouflage/session"
	wsslink "github.com/Sherlock-Holo/camouflage/session/wsslink/client"
	"github.com/Sherlock-Holo/libsocks"
	"github.com/Sherlock-Holo/link"
	log "github.com/sirupsen/logrus"
	errors "golang.org/x/xerrors"
)

type Client struct {
	listener    net.Listener
	session     session.Client
	connReqChan chan *connRequest
}

func New(cfg *client.Config) (*Client, error) {
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		err = errors.Errorf("local listen failed: %w", err)
		log.Fatalf("%+v", err)
	}

	cl := &Client{
		listener:    listener,
		connReqChan: make(chan *connRequest, 100),
	}

	switch cfg.Type {
	case client.TypeWebsocket:
		var opts []wsslink.Option

		if cfg.DebugCA != "" {
			ca, err := ioutil.ReadFile(cfg.DebugCA)
			if err != nil {
				err = errors.Errorf("read ca cert failed: %w", err)
				log.Fatalf("%+v", err)
			}

			opts = append(opts, wsslink.WithDebugCA(ca))
		}

		if cfg.Timeout.Duration > 0 {
			opts = append(opts, wsslink.WithHandshakeTimeout(cfg.Timeout.Duration))
		}

		wsURL := (&url.URL{
			Scheme: "wss",
			Host:   cfg.RemoteAddr,
		}).String()

		cl.session = wsslink.NewWssLink(wsURL, cfg.Secret, cfg.Period, opts...)

	case client.TypeQuic:
		panic("TODO")
	}

	go cl.managerLoop()

	return cl, nil
}

func (c *Client) Run() {
	for {
		conn, err := c.listener.Accept()
		if err != nil {
			err = errors.Errorf("accept failed: %w", err)
			log.Errorf("%v", err)
			continue
		}

		go c.handle(conn)
	}
}

func (c *Client) managerLoop() {
	for connReq := range c.connReqChan {
		conn, err := c.session.OpenConn(context.Background())
		if err != nil {
			err = errors.Errorf("session open connection failed: %w", err)
			connReq.Err <- err
			continue
		}

		connReq.Conn <- conn
	}
}

func (c *Client) handle(socksConn net.Conn) {
	socks, err := NewSocks(socksConn)
	if err != nil {
		err = errors.Errorf("client handle error: %w", err)
		log.Errorf("%v", err)
		return
	}

	connReq := &connRequest{
		Socks: socks,
		Conn:  make(chan net.Conn, 1),
		Err:   make(chan error, 1),
	}

	select {
	default:
		log.Warn("dial queue is full")
		_ = socks.Handshake(libsocks.TTLExpired)
		socks.Close()
		return

	case c.connReqChan <- connReq:
	}

	var sessionConn net.Conn

	select {
	case err := <-connReq.Err:
		log.Errorf("client handle error: %+v", err)

		switch {
		case errors.Is(err, link.ErrTimeout):
			_ = socks.Handshake(libsocks.TTLExpired)

		case errors.Is(err, link.ErrManagerClosed):
			_ = socks.Handshake(libsocks.NetworkUnreachable)

		default:
			_ = socks.Handshake(libsocks.ServerFailed)
		}

		socks.Close()
		return

	case sessionConn = <-connReq.Conn:
		if err := socks.Handshake(libsocks.Success); err != nil {
			err := errors.Errorf("client handle error: %w", err)
			log.Errorf("%+v", err)
			socks.Close()
			sessionConn.Close()
			return
		}
	}

	go func() {
		_, _ = io.Copy(sessionConn, socks)
		socks.Close()
		sessionConn.Close()
	}()

	go func() {
		_, _ = io.Copy(socks, sessionConn)
		socks.Close()
		sessionConn.Close()
	}()
}
