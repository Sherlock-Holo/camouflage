package server

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"github.com/Sherlock-Holo/camouflage/config/server"
	wsWrapper "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/libsocks"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

type Server struct {
	config   server.Config
	upgrader websocket.Upgrader
}

func (s *Server) checkRequest(w http.ResponseWriter, r *http.Request) {
	if len(r.TLS.PeerCertificates) == 0 {
		s.webHandle(w, r)
		return
	}

	if r.URL.Path != s.config.Path || !websocket.IsWebSocketUpgrade(r) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %s", err)
		return
	}

	manager := link.NewManager(wsWrapper.NewWrapper(conn), link.KeepaliveConfig())
	for {
		l, err := manager.Accept()
		if err != nil {
			log.Println(err)
			manager.Close()
			return
		}

		go handle(l)
	}
}

func (s *Server) webHandle(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, s.config.WebRoot)
}

func handle(l *link.Link) {
	address, err := libsocks.DecodeFrom(l)
	if err != nil {
		log.Println(err)
		l.Close()
		return
	}

	remote, err := net.Dial("tcp", address.String())
	if err != nil {
		log.Println(err)
		l.Close()
		return
	}

	go func() {
		if _, err := io.Copy(remote, l); err != nil {
			log.Println(err)
		}
		l.Close()
		remote.Close()
	}()

	go func() {
		if _, err := io.Copy(l, remote); err != nil {
			log.Println(err)
		}
		l.Close()
		remote.Close()
	}()
}

func (s *Server) Run() *http.Server {
	tlsConfig := new(tls.Config)
	tlsConfig.PreferServerCipherSuites = true

	// load client ca
	clientCAPool := x509.NewCertPool()

	clientCA, err := ioutil.ReadFile(s.config.ClientCACrt)
	if err != nil {
		log.Fatalf("read client ca crt failed: %+v", errors.WithStack(err))
	}

	clientCAPool.AppendCertsFromPEM(clientCA)
	tlsConfig.ClientCAs = clientCAPool

	// load server certificate
	serverCertificate, err := tls.LoadX509KeyPair(s.config.Crt, s.config.Key)
	if err != nil {
		log.Fatalf("read server key pair failed: %+v", errors.WithStack(err))
	}

	tlsConfig.Certificates = append(tlsConfig.Certificates, serverCertificate)

	tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven

	tcpListener, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		log.Fatalf("listen %s failed: %+v", s.config.ListenAddr, errors.WithStack(err))
	}

	tlsListener := tls.NewListener(tcpListener, tlsConfig)

	mux := http.NewServeMux()

	mux.HandleFunc("/", s.checkRequest)

	httpServer := &http.Server{Handler: mux}

	go httpServer.Serve(tlsListener)

	return httpServer
}

func New(cfg *server.Config) (servers *Server) {
	net.DefaultResolver.PreferGo = true
	return &Server{
		config: *cfg,
	}
}
