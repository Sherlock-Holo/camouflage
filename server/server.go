package server

import (
	"crypto/tls"
	"crypto/x509/pkix"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/Sherlock-Holo/camouflage/config/server"
	"github.com/Sherlock-Holo/camouflage/log"
	"github.com/Sherlock-Holo/camouflage/utils"
	wsWrapper "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/libsocks"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
	"golang.org/x/xerrors"
)

type Server struct {
	config      server.Config
	upgrader    websocket.Upgrader
	httpServer  http.Server
	tlsListener net.Listener
	crl         *pkix.CertificateList
}

func (s *Server) checkRequest(w http.ResponseWriter, r *http.Request) {
	code := r.Header.Get("totp-code")
	ok, err := utils.VerifyCode(code, s.config.Secret, s.config.Period)
	if err != nil {
		http.Error(w, "server internal error", http.StatusInternalServerError)
		log.Warnf("%+v", xerrors.Errorf("verify code error: %w", err))
		return
	}

	if !ok || !websocket.IsWebSocketUpgrade(r) {
		s.webHandle(w, r)
		return
	}

	s.proxyHandle(w, r)
}

func (s *Server) webHandle(w http.ResponseWriter, r *http.Request) {
	if s.config.WebRoot == "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	http.FileServer(http.Dir(s.config.WebRoot)).ServeHTTP(w, r)
}

func (s *Server) proxyHandle(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warnf("%+v", xerrors.Errorf("websocket upgrade failed: %w", err))
		return
	}

	linkCfg := link.DefaultConfig(link.ClientMode)
	linkCfg.KeepaliveInterval = 5 * time.Second

	manager := link.NewManager(wsWrapper.NewWrapper(conn), linkCfg)
	for {
		l, err := manager.Accept()
		if err != nil {
			log.Errorf("manager accept failed: %v", err)
			manager.Close()
			return
		}

		go handle(l)
	}
}

func handle(l link.Link) {
	address, err := libsocks.UnmarshalAddressFrom(l)
	if err != nil {
		log.Errorf("%+v", xerrors.Errorf("server unmarshal address failed: %w", err))
		l.Close()
		return
	}

	remote, err := net.Dial("tcp", address.String())
	if err != nil {
		log.Errorf("%+v", xerrors.Errorf("server connect target failed: %w", err))
		l.Close()
		return
	}

	go func() {
		io.Copy(remote, l)
		l.Close()
		remote.Close()
	}()

	go func() {
		io.Copy(l, remote)
		l.Close()
		remote.Close()
	}()
}

func (s *Server) Run() *http.Server {
	go s.httpServer.Serve(s.tlsListener)

	if s.webCertificateIsEnabled() {
		log.Info("sni enable")
	}

	if s.reverseProxyIsEnabled() {
		log.Info("reverse proxy enable")
	}

	if s.config.WebRoot != "" {
		log.Info("web service enable, web root:", s.config.WebRoot)
	}
	log.Info("camouflage started")

	return &s.httpServer
}

func New(cfg *server.Config) (server *Server) {
	server = &Server{
		config: *cfg,
	}

	if cfg.Timeout.Duration > 0 {
		server.upgrader.HandshakeTimeout = cfg.Timeout.Duration
	}

	tlsConfig := &tls.Config{
		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2"},
		MinVersion:               tls.VersionTLS12,
	}

	// load server certificate
	serverCertificate, err := tls.LoadX509KeyPair(server.config.Crt, server.config.Key)
	if err != nil {
		log.Fatalf("%+v", xerrors.Errorf("read server key pair failed: %w", err))
	}

	tlsConfig.Certificates = append(tlsConfig.Certificates, serverCertificate)

	if server.webCertificateIsEnabled() {
		// load web certificate
		webCertificate, err := tls.LoadX509KeyPair(server.config.WebCrt, server.config.WebKey)
		if err != nil {
			log.Fatalf("%+v", xerrors.Errorf("read web key pair failed: %w", err))
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, webCertificate)
	}

	if server.reverseProxyIsEnabled() {
		// load read reverse certificate
		reverseProxyCertificate, err := tls.LoadX509KeyPair(server.config.ReverseProxyCrt, server.config.ReverseProxyKey)
		if err != nil {
			log.Fatalf("%+v", xerrors.Errorf("read reverse proxy key pair failed: %w", err))
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, reverseProxyCertificate)
	}

	tlsConfig.BuildNameToCertificate()

	server.tlsListener, err = tls.Listen("tcp", server.config.ListenAddr, tlsConfig)
	if err != nil {
		log.Fatalf("%+v", xerrors.Errorf("listen %s failed: %w", server.config.ListenAddr, err))
	}

	mux := http.NewServeMux()

	if server.webCertificateIsEnabled() {
		mux.HandleFunc(server.config.Host+"/", server.proxyHandle)
		mux.Handle(server.config.WebHost+"/", enableGzip(http.HandlerFunc(server.webHandle)))
	} else {
		mux.HandleFunc(server.config.Host+"/", server.checkRequest)
	}

	// enable reverse proxy
	if server.reverseProxyIsEnabled() {
		if !strings.HasPrefix(server.config.ReverseProxyAddr, "http") && !strings.HasPrefix(server.config.ReverseProxyAddr, "https") {
			server.config.ReverseProxyAddr = "http://" + server.config.ReverseProxyAddr
		}

		u, err := url.Parse(server.config.ReverseProxyAddr)
		if err != nil {
			log.Fatalf("%+v", xerrors.Errorf("parse reverse proxy addr failed: %w", err))
		}

		proxy := httputil.NewSingleHostReverseProxy(u)
		originDirector := proxy.Director
		proxy.Director = func(r *http.Request) {
			originDirector(r)
			// delete origin field to avoid websocket upgrade check failed
			r.Header.Del("origin")
		}

		mux.Handle(server.config.ReverseProxyHost+"/", proxy)
	}

	server.httpServer = http.Server{Handler: mux}

	return
}

func (s *Server) webCertificateIsEnabled() bool {
	return s.config.WebCrt != "" && s.config.WebKey != "" && s.config.WebRoot != "" && s.config.WebHost != ""
}

func (s *Server) reverseProxyIsEnabled() bool {
	return s.config.ReverseProxyHost != "" && s.config.ReverseProxyCrt != "" && s.config.ReverseProxyKey != "" && s.config.ReverseProxyAddr != ""
}
