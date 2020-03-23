package server

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
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

type handshakeTimeout time.Duration

func (h handshakeTimeout) apply(link *wssLink) {
	link.upgrader.HandshakeTimeout = time.Duration(h)
}

func WithHandshakeTimeout(timeout time.Duration) Option {
	return handshakeTimeout(timeout)
}

type webConfig struct {
	root string
	host string
	crt  tls.Certificate
}

func (w webConfig) apply(link *wssLink) {
	link.tlsConfig.Certificates = append(link.tlsConfig.Certificates, w.crt)
	link.httpMux.Handle(w.host+"/", enableGzip(http.FileServer(http.Dir(w.root))))
}

func WithWeb(root, host string, crt tls.Certificate) Option {
	return webConfig{
		root: root,
		host: host,
		crt:  crt,
	}
}

type reverseProxyConfig struct {
	host       *url.URL
	realserver string
	crt        tls.Certificate
}

func (r reverseProxyConfig) apply(link *wssLink) {
	proxy := httputil.NewSingleHostReverseProxy(r.host)
	originDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		originDirector(r)
		// delete origin field to avoid websocket upgrade check failed
		r.Header.Del("origin")
	}

	link.httpMux.Handle(r.host.Host+"/", proxy)
}

func WithReverseProxy(host *url.URL, realserver string, crt tls.Certificate) Option {
	return reverseProxyConfig{
		host:       host,
		realserver: realserver,
		crt:        crt,
	}
}

type wssLink struct {
	upgrader websocket.Upgrader

	host       string
	httpMux    *http.ServeMux
	httpServer http.Server

	tlsConfig   *tls.Config
	tlsListener net.Listener

	secret string
	period uint

	linkManagerIdGen *atomic.Uint64
	linkManagerMap   sync.Map

	acceptChan chan net.Conn

	startOnce sync.Once
	closed    atomic.Bool
}

func NewServer(listenAddr, host, wsPath, secret string, period uint, serverCert tls.Certificate, opts ...Option) (*wssLink, error) {
	wl := &wssLink{
		host: host,

		tlsConfig: &tls.Config{
			PreferServerCipherSuites: true,
			NextProtos:               []string{"h2"},
			MinVersion:               tls.VersionTLS12,
			Certificates:             []tls.Certificate{serverCert},
		},

		secret: secret,
		period: period,

		linkManagerIdGen: atomic.NewUint64(0),

		acceptChan: make(chan net.Conn, 100),
	}

	wl.httpMux = http.NewServeMux()

	wl.httpMux.Handle(host+wsPath, http.HandlerFunc(wl.wsHandle))

	for _, opt := range opts {
		opt.apply(wl)
	}

	wl.httpServer = http.Server{Handler: wl.httpMux}

	tlsListener, err := tls.Listen("tcp", listenAddr, wl.tlsConfig)
	if err != nil {
		return nil, errors.Errorf("listen %s failed: %w", listenAddr, err)
	}

	wl.tlsListener = tlsListener

	return wl, nil
}

func (w *wssLink) Name() string {
	return "wsslink"
}

func (w *wssLink) Close() error {
	if w.closed.CAS(false, true) {
		timeout, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelFunc()

		_ = w.httpServer.Shutdown(timeout)

		w.linkManagerMap.Range(func(_, value interface{}) bool {
			_ = value.(link.Manager).Close()

			return true
		})

		_ = w.httpServer.Close()
	}

	return nil
}

func (w *wssLink) AcceptConn(ctx context.Context) (net.Conn, error) {
	// lazy start
	w.startOnce.Do(func() {
		go func() {
			_ = w.httpServer.Serve(w.tlsListener)
		}()
	})

	if w.closed.Load() {
		return nil, &net.OpError{
			Op:  "open",
			Net: w.Name(),
			Err: errors.New("wss link is closed"),
		}
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	case conn := <-w.acceptChan:
		return conn, nil
	}
}

func (w *wssLink) wsHandle(writer http.ResponseWriter, request *http.Request) {
	code := request.Header.Get("totp-code")

	ok, err := utils.VerifyCode(code, w.secret, w.period)
	if err != nil {
		err = errors.Errorf("verify code error: %w", err)
		log.Warnf("%+v", err)

		http.Error(writer, "server internal error", http.StatusInternalServerError)

		return
	}

	if !ok || !websocket.IsWebSocketUpgrade(request) {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	conn, err := w.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		err = errors.Errorf("websocket upgrade failed: %w", err)
		log.Warnf("%+v", err)
		return
	}

	linkCfg := link.DefaultConfig(link.ClientMode)
	linkCfg.KeepaliveInterval = 5 * time.Second

	manager := link.NewManager(wsWrapper.NewWrapper(conn), linkCfg)

	linkManagerId := w.linkManagerIdGen.Add(1) - 1

	w.linkManagerMap.Store(linkManagerId, manager)

	go func() {
		defer func() {
			_ = manager.Close()

			w.linkManagerMap.Delete(linkManagerId)
		}()

		for {
			linkConn, err := manager.Accept()
			if err != nil {
				err = errors.Errorf("accept wss link failed: %w", err)
				log.Errorf("%+v", err)
				return
			}

			select {
			default:
				log.Warn("accept queue is full")

				_ = linkConn.Close()

			case w.acceptChan <- linkConn:
			}
		}
	}()
}
