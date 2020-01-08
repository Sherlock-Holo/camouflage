package client

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Sherlock-Holo/camouflage/utils"
	wsWrapper "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
	"go.uber.org/atomic"
	errors "golang.org/x/xerrors"
)

type Option interface {
	apply(link *wssLink)
}

type debugCA []byte

func (d debugCA) apply(link *wssLink) {
	link.wsDialer.TLSClientConfig.RootCAs.AppendCertsFromPEM(d)
}

func WithDebugCA(ca []byte) Option {
	return debugCA(ca)
}

type handshakeTimeout time.Duration

func (h handshakeTimeout) apply(link *wssLink) {
	link.wsDialer.HandshakeTimeout = time.Duration(h)
}

func WithHandshakeTimeout(timeout time.Duration) Option {
	return handshakeTimeout(timeout)
}

type wssLink struct {
	wsURL    string
	wsDialer websocket.Dialer

	secret string
	period uint

	manager      link.Manager
	connectMutex sync.Mutex
	errHappened  *atomic.Bool
	closed       *atomic.Bool
}

func (w *wssLink) Name() string {
	return "wsslink"
}

func NewWssLink(wsURL, totpSecret string, totpPeriod uint, opts ...Option) *wssLink {
	wl := &wssLink{
		wsURL: wsURL,
		wsDialer: websocket.Dialer{
			TLSClientConfig: new(tls.Config),
		},

		secret: totpSecret,
		period: totpPeriod,

		errHappened: atomic.NewBool(true), // link manager not init, need init in connect()
		closed:      atomic.NewBool(false),
	}

	for _, opt := range opts {
		opt.apply(wl)
	}

	return wl
}

func (w *wssLink) Close() error {
	w.closed.Store(true)

	return nil
}

func (w *wssLink) OpenConn(ctx context.Context) (net.Conn, error) {
	if !w.closed.Load() {
		return nil, &net.OpError{
			Op:  "open",
			Net: w.Name(),
			Err: errors.New("session is closed"),
		}
	}

	if err := w.connect(ctx); err != nil {
		return nil, errors.Errorf("connect %s server failed: %w", w.Name(), err)
	}

	switch preData := ctx.Value("pre-data").(type) {
	case nil:
		conn, err := w.manager.Dial(ctx)
		if err != nil {
			return nil, errors.Errorf("dial wsslink failed: %w", err)
		}

		return &connection{
			Conn:        conn,
			errHappened: w.errHappened,
		}, nil

	case []byte:
		conn, err := w.manager.DialData(ctx, preData)
		if err != nil {
			return nil, errors.Errorf("dial wsslink failed: %w", err)
		}

		return &connection{
			Conn:        conn,
			errHappened: w.errHappened,
		}, nil

	default:
		return nil, errors.New("invalid pre-data")
	}
}

// lazy init, until OpenConn called, won't dial websocket
func (w *wssLink) connect(ctx context.Context) error {
	// quick path
	if !w.errHappened.Load() {
		return nil
	}

	w.connectMutex.Lock()
	defer w.connectMutex.Unlock()

	if !w.errHappened.Load() {
		return nil
	}

	if w.manager != nil {
		_ = w.manager.Close()
	}

	for i := 0; i < 2; i++ {
		code, err := utils.GenCode(w.secret, w.period)
		if err != nil {
			return errors.Errorf("connect failed: %w", err)
		}

		httpHeader := http.Header{}
		httpHeader.Set("totp-code", code)

		conn, response, err := w.wsDialer.DialContext(ctx, w.wsURL, httpHeader)
		switch {
		case errors.Is(err, websocket.ErrBadHandshake):
			response.Body.Close()

			if response.StatusCode == http.StatusForbidden {
				if i == 1 {
					return errors.New("connect failed: maybe TOTP secret is wrong")
				} else {
					continue
				}
			}

			fallthrough

		default:
			return errors.Errorf("connect failed: %w", err)

		case err == nil:
		}

		linkCfg := link.DefaultConfig(link.ClientMode)
		linkCfg.KeepaliveInterval = 5 * time.Second

		w.manager = link.NewManager(wsWrapper.NewWrapper(conn), linkCfg)

		w.errHappened.Store(false)

		return nil
	}

	panic("unreachable")
}
