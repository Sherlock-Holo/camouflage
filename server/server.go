package server

import (
	"crypto/tls"
	"crypto/x509/pkix"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/Sherlock-Holo/camouflage/config/server"
	"github.com/Sherlock-Holo/camouflage/utils"
	wsWrapper "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/libsocks"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
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
		log.Printf("verify code failed: %+v", err)
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
		log.Printf("websocket upgrade failed: %s", errors.WithStack(err))
		return
	}

	linkCfg := link.DefaultConfig(link.ClientMode)
	linkCfg.KeepaliveInterval = 5 * time.Second

	manager := link.NewManager(wsWrapper.NewWrapper(conn), linkCfg)
	for {
		l, err := manager.Accept()
		if err != nil {
			log.Printf("manager accept failed: %v", err)
			manager.Close()
			return
		}

		go handle(l)
	}
}

func handle(l link.Link) {
	address, err := libsocks.DecodeFrom(l)
	if err != nil {
		log.Printf("decode socks failed: %+v", errors.WithStack(err))
		l.Close()
		return
	}

	remote, err := net.Dial("tcp", address.String())
	if err != nil {
		log.Printf("connect target failed: %+v", errors.WithStack(err))
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

func (s *Server) Run() http.Server {
	go s.httpServer.Serve(s.tlsListener)

	if s.enableWebCertificate() {
		log.Println("sni enable")
	}
	if s.config.WebRoot != "" {
		log.Println("web service enable, web root:", s.config.WebRoot)
	}
	log.Println("camouflage started")

	return s.httpServer
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
	}

	// load server certificate
	serverCertificate, err := tls.LoadX509KeyPair(server.config.Crt, server.config.Key)
	if err != nil {
		log.Fatalf("read server key pair failed: %+v", errors.WithStack(err))
	}

	tlsConfig.Certificates = append(tlsConfig.Certificates, serverCertificate)

	if server.enableWebCertificate() {
		// load web certificate
		webCertificate, err := tls.LoadX509KeyPair(server.config.WebCrt, server.config.WebKey)
		if err != nil {
			log.Fatalf("read web key pair failed: %+v", errors.WithStack(err))
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, webCertificate)
	}

	tlsConfig.BuildNameToCertificate()

	server.tlsListener, err = tls.Listen("tcp", server.config.ListenAddr, tlsConfig)
	if err != nil {
		log.Fatalf("listen %s failed: %+v", server.config.ListenAddr, errors.WithStack(err))
	}

	mux := http.NewServeMux()

	if server.enableWebCertificate() {
		mux.HandleFunc(server.config.Host+"/", server.proxyHandle)
		mux.HandleFunc(server.config.WebHost+"/", server.webHandle)
	} else {
		mux.HandleFunc("/", server.checkRequest)
	}

	server.httpServer = http.Server{Handler: mux}

	return
}

func (s *Server) enableWebCertificate() bool {
	return s.config.WebCrt != "" && s.config.WebKey != "" && s.config.WebRoot != "" && s.config.WebHost != ""
}
