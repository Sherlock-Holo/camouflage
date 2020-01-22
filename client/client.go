package client

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"time"

	"github.com/Sherlock-Holo/camouflage/config/client"
	"github.com/Sherlock-Holo/camouflage/session"
	quic "github.com/Sherlock-Holo/camouflage/session/quic/client"
	wsslink "github.com/Sherlock-Holo/camouflage/session/wsslink/client"
	"github.com/Sherlock-Holo/libsocks"
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
			Host:   cfg.Host,
			Path:   cfg.Path,
		}).String()

		cl.session = wsslink.NewClient(wsURL, cfg.Secret, cfg.Period, opts...)

	case client.TypeQuic:
		var opts []quic.Option

		if cfg.DebugCA != "" {
			ca, err := ioutil.ReadFile(cfg.DebugCA)
			if err != nil {
				err = errors.Errorf("read ca cert failed: %w", err)
				log.Fatalf("%+v", err)
			}

			opts = append(opts, quic.WithDebugCA(ca))
		}

		cl.session = quic.NewClient(cfg.Host, cfg.Secret, cfg.Period, opts...)
	}

	if cfg.Pprof != "" {
		go func() {
			if err := http.ListenAndServe(cfg.Pprof, nil); err != nil {
				err := errors.Errorf("enable pprof failed: %w", err)
				log.Warnf("%+v", err)
			}
		}()
	}

	return cl, nil
}

func (c *Client) Run() {
	go c.acceptConnReq()

	for {
		socksConn, err := c.listener.Accept()
		if err != nil {
			err = errors.Errorf("accept socks failed: %w", err)
			log.Errorf("%v", err)
			continue
		}

		log.Debugf("accept from %s", socksConn.RemoteAddr())

		go c.handle(socksConn)
	}
}

func (c *Client) acceptConnReq() {
	for connReq := range c.connReqChan {
		ctx := context.WithValue(context.Background(), "pre-data", connReq.Socks.Target())

		conn, err := c.session.OpenConn(ctx)
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
		_ = socks.Close()
		return

	case c.connReqChan <- connReq:
	}

	var sessionConn net.Conn

	select {
	case <-time.After(30 * time.Second):
		log.Error("client handle timeout")

		_ = socks.Handshake(libsocks.TTLExpired)
		_ = socks.Close()

		return

	case err := <-connReq.Err:
		log.Errorf("client handle error: %+v", err)

		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			_ = socks.Handshake(libsocks.TTLExpired)
		} else {
			_ = socks.Handshake(libsocks.ServerFailed)
		}

		_ = socks.Close()
		return

	case sessionConn = <-connReq.Conn:
		log.Debug("start socks handshake")

		if err := socks.Handshake(libsocks.Success); err != nil {
			err := errors.Errorf("client handle error: %w", err)
			log.Errorf("%+v", err)
			_ = socks.Close()
			_ = sessionConn.Close()
			return
		}

		log.Debug("socks handshake success")
	}

	go func() {
		_, _ = io.Copy(sessionConn, socks)
		_ = socks.Close()
		_ = sessionConn.Close()
	}()

	go func() {
		_, _ = io.Copy(socks, sessionConn)
		_ = socks.Close()
		_ = sessionConn.Close()
	}()
}
