package server

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"path"
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

type wssLink struct {
	upgrader websocket.Upgrader

	host       string
	httpMux    *http.ServeMux
	httpServer http.Server

	tlsConfig   *tls.Config
	tlsListener net.Listener

	//crl *pkix.CertificateList

	secret string
	period uint

	linkManagerIdGen *atomic.Uint64
	linkManagerMap   sync.Map

	acceptChan chan net.Conn
}

func NewWssLink(listenAddr, host, wsPath, secret string, period uint, serverCert tls.Certificate, opts ...Option) (*wssLink, error) {
	wl := &wssLink{
		host: host,

		tlsConfig: &tls.Config{
			PreferServerCipherSuites: true,
			NextProtos:               []string{"h2"},
			MinVersion:               tls.VersionTLS12,
		},

		secret: secret,
		period: period,

		linkManagerIdGen: atomic.NewUint64(0),

		acceptChan: make(chan net.Conn, 100),
	}

	wl.tlsConfig.Certificates = append(wl.tlsConfig.Certificates, serverCert)

	wl.httpMux = http.NewServeMux()

	wl.httpMux.Handle(path.Join(host, wsPath), http.HandlerFunc(wl.wsHandle))

	for _, opt := range opts {
		opt.apply(wl)
	}

	wl.tlsConfig.BuildNameToCertificate()

	wl.httpServer = http.Server{Handler: wl.httpMux}

	tlsListener, err := tls.Listen("tcp", listenAddr, wl.tlsConfig)
	if err != nil {
		return nil, errors.Errorf("listen %s failed: %w", listenAddr, err)
	}

	wl.tlsListener = tlsListener

	go func() {
		_ = wl.httpServer.Serve(wl.tlsListener)
	}()

	return wl, nil
}

func (w *wssLink) Name() string {
	return "wsslink"
}

func (w *wssLink) Close() error {
	timeout, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()

	_ = w.httpServer.Shutdown(timeout)

	w.linkManagerMap.Range(func(_, value interface{}) bool {
		_ = value.(link.Manager).Close()

		return true
	})

	_ = w.httpServer.Close()

	return nil
}

func (w *wssLink) AcceptConn(ctx context.Context) (net.Conn, error) {
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
		//w.webHandle(w, request) TODO
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

		linkConn, err := manager.Accept()
		if err != nil {
			err = errors.Errorf("accept wss link failed: %w", err)
			log.Errorf("%+v", err)
		}

		select {
		default:
			log.Warn("accept queue is full")

			_ = linkConn.Close()

		case w.acceptChan <- linkConn:
		}
	}()
}
