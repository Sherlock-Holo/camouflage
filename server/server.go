package server

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

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
	if s.enableWebCertificate() {
		switch r.Host {
		case s.config.WebHost:
			s.webHandle(w, r)
			return

		case s.config.Host:
			exist, revoke := s.checkCertificate(r)
			if !exist {
				s.redirect(w, r)
				return
			}

			if revoke {
				return
			}

			s.proxyHandle(w, r)
			return

		default:
			s.redirect(w, r)
			return
		}
	}

	exist, revoke := s.checkCertificate(r)
	if !exist || revoke {
		s.webHandle(w, r)
		return
	}

	s.proxyHandle(w, r)
}

func (s *Server) webHandle(w http.ResponseWriter, r *http.Request) {
	http.FileServer(http.Dir(s.config.WebRoot)).ServeHTTP(w, r)
}

func (s *Server) proxyHandle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != s.config.Path || !websocket.IsWebSocketUpgrade(r) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %s", errors.WithStack(err))
		return
	}

	manager := link.NewManager(wsWrapper.NewWrapper(conn), link.KeepaliveConfig(link.ServerMode))
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

func (s *Server) checkCertificate(r *http.Request) (exist, revoke bool) {
	if len(r.TLS.PeerCertificates) == 0 {
		return false, false
	}
	// check client certificate in crl or not
	if s.crl != nil {
		for _, certificate := range r.TLS.PeerCertificates {
			if utils.IsRevokedCertificate(certificate, s.crl) {
				return true, true
			}
		}
	}

	return true, false
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	webUrl := url.URL{
		Scheme: "https",
		Path:   r.URL.Path,
	}
	if strings.HasSuffix(s.config.WebHost, ":443") {
		webUrl.Host = strings.Split(s.config.WebHost, ":")[0]
	} else {
		webUrl.Host = s.config.WebHost
	}

	http.Redirect(w, r, webUrl.String(), http.StatusFound)
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
		/*if _, err := io.Copy(remote, l); err != nil {
			// log.Printf("forward link data to remote server failed: %v", err)
		}*/
		io.Copy(remote, l)
		l.Close()
		remote.Close()
	}()

	go func() {
		/*if _, err := io.Copy(l, remote); err != nil {
			// log.Printf("forward remote server data to link failed: %v", err)
		}*/
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
	net.DefaultResolver.PreferGo = true
	server = &Server{
		config: *cfg,
	}

	tlsConfig := &tls.Config{
		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2"},
		MinVersion:               tls.VersionTLS12,
	}

	// load client ca
	clientCAPool := x509.NewCertPool()

	clientCA, err := ioutil.ReadFile(server.config.ClientCACrt)
	if err != nil {
		log.Fatalf("read client ca crt failed: %+v", errors.WithStack(err))
	}

	clientCAPool.AppendCertsFromPEM(clientCA)
	tlsConfig.ClientCAs = clientCAPool

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

	// set client auth mode, use tls.VerifyClientCertIfGiven
	tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven

	tlsConfig.BuildNameToCertificate()

	// read crl
	if cfg.Crl != "" {
		crlBytes, err := ioutil.ReadFile(cfg.Crl)
		if err != nil {
			log.Fatalf("read crl file failed: %+v", errors.WithStack(err))
		}
		crl, err := x509.ParseCRL(crlBytes)
		if err != nil {
			log.Fatalf("parse crl failed: %+v", errors.WithStack(err))
		}
		server.crl = crl
	}

	tcpListener, err := net.Listen("tcp", server.config.ListenAddr)
	if err != nil {
		log.Fatalf("listen %s failed: %+v", server.config.ListenAddr, errors.WithStack(err))
	}

	server.tlsListener = tls.NewListener(tcpListener, tlsConfig)

	mux := http.NewServeMux()

	mux.HandleFunc("/", server.checkRequest)

	server.httpServer = http.Server{Handler: mux}

	return
}

func (s *Server) enableWebCertificate() bool {
	return s.config.WebCrt != "" && s.config.WebKey != "" && s.config.WebRoot != "" && s.config.WebHost != ""
}
