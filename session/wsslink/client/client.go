package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Sherlock-Holo/camouflage/utils"
	wsWrapper "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	errors "golang.org/x/xerrors"
)

type Option interface {
	apply(link *wssLink)
}

type debugCA []byte

func (d debugCA) apply(link *wssLink) {
	if link.wsDialer.TLSClientConfig.RootCAs == nil {
		link.wsDialer.TLSClientConfig.RootCAs = x509.NewCertPool()
	}

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
	closed       atomic.Bool
}

func (w *wssLink) Name() string {
	return "wsslink"
}

func NewClient(wsURL, totpSecret string, totpPeriod uint, opts ...Option) *wssLink {
	wl := &wssLink{
		wsURL: wsURL,
		wsDialer: websocket.Dialer{
			TLSClientConfig: new(tls.Config),
		},

		secret: totpSecret,
		period: totpPeriod,
	}

	for _, opt := range opts {
		opt.apply(wl)
	}

	return wl
}

func (w *wssLink) Close() error {
	if w.closed.CAS(false, true) {
		return w.manager.Close()
	}

	return nil
}

func (w *wssLink) OpenConn(ctx context.Context) (net.Conn, error) {
	if w.closed.Load() {
		return nil, &net.OpError{
			Op:  "open",
			Net: w.Name(),
			Err: errors.New("session is closed"),
		}
	}

	w.connectMutex.Lock()
	defer w.connectMutex.Unlock()

	if w.manager == nil {
		for {
			err := w.reconnect(ctx)
			switch {
			case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
				netErr := &net.OpError{
					Op:  "open",
					Net: w.Name(),
					Err: err,
				}

				// when connect timeout, manager may can't recover
				_ = w.manager.Close()
				w.manager = nil

				return nil, errors.Errorf("connect wss link failed: %w", netErr)

			default:
				continue

			case err == nil:
			}

			log.Debug("wss link connect success")
			break
		}
	}

	if w.manager.IsClosed() {
		if err := w.reconnect(ctx); err != nil {
			return nil, errors.Errorf("reconnect wss link failed: %w", err)
		}
	}

	if raw := ctx.Value("pre-data"); raw != nil {
		preData, ok := raw.([]byte)
		if !ok {
			return nil, &net.OpError{
				Op:  "open",
				Net: w.Name(),
				Err: errors.New("invalid pre-data"),
			}
		}

		log.Debug("dial data")
		return w.manager.DialData(ctx, preData)
	}

	return w.manager.Dial(ctx)
}

// lazy init, until OpenConn called, won't dial websocket
func (w *wssLink) reconnect(ctx context.Context) error {
	if w.manager != nil {
		_ = w.manager.Close()
	}

	for i := 0; i < 2; i++ {
		code, err := utils.GenCode(w.secret, w.period)
		if err != nil {
			return errors.Errorf("generate TOTP code failed: %w", err)
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

		return nil
	}

	panic("unreachable")
}
