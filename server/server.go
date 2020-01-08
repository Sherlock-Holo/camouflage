package server

import (
	"context"
	"crypto/tls"
	"io"
	"net"

	config "github.com/Sherlock-Holo/camouflage/config/server"
	"github.com/Sherlock-Holo/camouflage/session"
	wsslink "github.com/Sherlock-Holo/camouflage/session/wsslink/server"
	"github.com/Sherlock-Holo/libsocks"
	log "github.com/sirupsen/logrus"
	errors "golang.org/x/xerrors"
)

type Server struct {
	session session.Server
}

func New(cfg *config.Config) (*Server, error) {
	// load server certificate
	serverCert, err := tls.LoadX509KeyPair(cfg.Crt, cfg.Key)
	if err != nil {
		return nil, errors.Errorf("read server key pair failed: %w", err)
	}

	var sess session.Server

	switch cfg.Type {
	case config.TypeWebsocket:
		var opts []wsslink.Option

		opts = append(opts, wsslink.WithHandshakeTimeout(cfg.Timeout.Duration))

		var err error
		sess, err = wsslink.NewWssLink(cfg.ListenAddr, cfg.Host, cfg.Path, cfg.Secret, cfg.Period, serverCert, opts...)
		if err != nil {
			return nil, errors.Errorf("new wss link server failed: %w", err)
		}

	case config.TypeQuic:
		panic("TODO")
	}

	server := &Server{
		session: sess,
	}

	return server, nil

	/*if server.webCertificateIsEnabled() {
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

	return*/
}

/*func (s *Server) checkRequest(w http.ResponseWriter, r *http.Request) {
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
}*/

func handle(conn net.Conn) {
	address, err := libsocks.UnmarshalAddressFrom(conn)
	if err != nil {
		err = errors.Errorf("server unmarshal address failed: %w", err)
		log.Errorf("%+v", err)
		conn.Close()

		return
	}

	remote, err := net.Dial("tcp", address.String())
	if err != nil {
		err = errors.Errorf("server connect target failed: %w", err)
		log.Errorf("%+v", err)
		conn.Close()

		return
	}

	go func() {
		_, _ = io.Copy(remote, conn)
		conn.Close()
		remote.Close()
	}()

	go func() {
		_, _ = io.Copy(conn, remote)
		conn.Close()
		remote.Close()
	}()
}

func (s *Server) Run() {
	for {
		conn, err := s.session.AcceptConn(context.TODO())
		if err != nil {
			err = errors.Errorf("accept connection failed: %w", err)
			log.Errorf("%+v", err)

			continue
		}

		go handle(conn)
	}
}

func (s *Server) Close() error {
	return s.session.Close()
}

/*func (s *Server) webCertificateIsEnabled() bool {
	return s.config.WebCrt != "" && s.config.WebKey != "" && s.config.WebRoot != "" && s.config.WebHost != ""
}

func (s *Server) reverseProxyIsEnabled() bool {
	return s.config.ReverseProxyHost != "" && s.config.ReverseProxyCrt != "" && s.config.ReverseProxyKey != "" && s.config.ReverseProxyAddr != ""
}*/
