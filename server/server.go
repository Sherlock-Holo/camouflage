package server

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"strings"

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
	var sess session.Server

	switch cfg.Type {
	case config.TypeWebsocket:
		var opts []wsslink.Option

		if cfg.Timeout.Duration > 0 {
			opts = append(opts, wsslink.WithHandshakeTimeout(cfg.Timeout.Duration))
		}

		if cfg.WebCrt != "" && cfg.WebKey != "" && cfg.WebHost != "" && cfg.WebRoot != "" {
			if _, err := os.Stat(cfg.WebKey); err != nil {
				return nil, errors.Errorf("get web root stat failed: %w", err)
			}

			webCrt, err := tls.LoadX509KeyPair(cfg.WebCrt, cfg.WebKey)
			if err != nil {
				return nil, errors.Errorf("load web certificate failed: %w", err)
			}

			opts = append(opts, wsslink.WithWeb(cfg.WebRoot, cfg.WebHost, webCrt))

			log.Info("enable web")
		}

		if cfg.ReverseProxyCrt != "" && cfg.ReverseProxyKey != "" && cfg.ReverseProxyHost != "" && cfg.ReverseProxyAddr != "" {
			if !strings.HasPrefix(cfg.ReverseProxyAddr, "http") {
				cfg.ReverseProxyAddr = "http://" + cfg.ReverseProxyAddr
			}

			host, err := url.Parse(cfg.ReverseProxyAddr)
			if err != nil {
				return nil, errors.Errorf("parse reverser proxy host failed: %w", err)
			}

			log.Debugf("reverse proxy host: %s", host)

			reverseProxyCrt, err := tls.LoadX509KeyPair(cfg.ReverseProxyCrt, cfg.ReverseProxyKey)
			if err != nil {
				return nil, errors.Errorf("load reverse proxy certificate failed: %w", err)
			}

			opts = append(opts, wsslink.WithReverseProxy(host, cfg.ReverseProxyAddr, reverseProxyCrt))

			log.Info("enable reverse proxy")
		}

		// load server certificate
		serverCert, err := tls.LoadX509KeyPair(cfg.Crt, cfg.Key)
		if err != nil {
			return nil, errors.Errorf("read server key pair failed: %w", err)
		}

		sess, err = wsslink.NewServer(cfg.ListenAddr, cfg.Host, cfg.Path, cfg.Secret, cfg.Period, serverCert, opts...)
		if err != nil {
			return nil, errors.Errorf("new wss link server failed: %w", err)
		}

		/*case config.TypeQuic:
		var opts []quic.Option

		// load server certificate
		serverCert, err := tls.LoadX509KeyPair(cfg.Crt, cfg.Key)
		if err != nil {
			return nil, errors.Errorf("read server key pair failed: %w", err)
		}

		sess, err = quic.NewServer(cfg.ListenAddr, cfg.Secret, cfg.Period, serverCert, opts...)
		if err != nil {
			return nil, errors.Errorf("new quic server failed: %w", err)
		}*/
	}

	server := &Server{
		session: sess,
	}

	if cfg.Pprof != "" {
		go func() {
			if err := http.ListenAndServe(cfg.Pprof, nil); err != nil {
				err := errors.Errorf("enable pprof failed: %w", err)
				log.Warnf("%+v", err)
			}
		}()
	}

	return server, nil
}

func handle(conn net.Conn) {
	address, err := libsocks.UnmarshalAddressFrom(conn)
	if err != nil {
		err = errors.Errorf("server unmarshal address failed: %w", err)
		log.Errorf("%+v", err)
		_ = conn.Close()

		return
	}

	remote, err := net.Dial("tcp", address.String())
	if err != nil {
		err = errors.Errorf("server connect target failed: %w", err)
		log.Errorf("%+v", err)
		_ = conn.Close()

		return
	}

	log.Debug("start proxy")

	go func() {
		_, _ = io.Copy(remote, conn)
		_ = conn.Close()
		_ = remote.Close()
	}()

	go func() {
		_, _ = io.Copy(conn, remote)
		_ = conn.Close()
		_ = remote.Close()
	}()
}

func (s *Server) Run() {
	for {
		conn, err := s.session.AcceptConn(context.Background())
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
