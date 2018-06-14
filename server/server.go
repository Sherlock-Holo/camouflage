package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"

	"github.com/Sherlock-Holo/camouflage/config"
	"github.com/Sherlock-Holo/camouflage/dns"
	websocket2 "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/libsocks"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
)

type Server struct {
	server   http.Server
	config   config.Server
	resolver *dns.Resolver
}

type handler struct {
	server   *Server
	upgrader websocket.Upgrader
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !websocket.IsWebSocketUpgrade(r) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		conn.Close()
		return
	}

	manager := link.NewManager(websocket2.NewWrapper(conn))

	for {
		l, err := manager.Accept()
		if err != nil {
			log.Println(err)
			manager.Close()
			return
		}

		go handle(h, l)
	}
}

func handle(handler *handler, l *link.Link) {
	address, err := libsocks.DecodeFrom(l)
	if err != nil {
		log.Println(err)
		l.Close()
		return
	}

	if handler.server.resolver != nil {
		if address.Type == 3 {
			result, err := handler.server.resolver.Query(address.Host, handler.server.config.PreferIPv6, handler.server.config.DNSTimeout)
			if err != nil {
				log.Println(err)
				l.Close()
				return
			}

			if handler.server.config.PreferIPv6 {
				if len(result.AAAAIP) > 0 {
					address.IP = result.AAAAIP[rand.Intn(len(result.AAAAIP))]
					address.Type = 4
				} else {
					address.IP = result.AIP[rand.Intn(len(result.AIP))]
					address.Type = 1
				}

			} else {
				if len(result.AIP) > 0 {
					address.IP = result.AIP[rand.Intn(len(result.AIP))]
					address.Type = 1

				} else {
					address.IP = result.AAAAIP[rand.Intn(len(result.AAAAIP))]
					address.Type = 4
				}
			}

		}
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
			remote.Close()
			l.Close()
			return
		}

		remote.(*net.TCPConn).CloseWrite()
	}()

	go func() {
		if _, err := io.Copy(l, remote); err != nil {
			log.Println(err)
			remote.Close()
			l.Close()
			return
		}

		l.CloseWrite()
	}()
}

func (s *Server) Run() {
	log.Println(s.server.ListenAndServeTLS(s.config.CrtFile, s.config.KeyFile))
}

func NewServer(cfg config.Server) (*Server, error) {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(cfg.CA)

	handler := new(handler)

	server := &Server{
		server: http.Server{
			Addr:    fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.BindPort),
			Handler: handler,
			TLSConfig: &tls.Config{
				ClientCAs:  pool,
				ClientAuth: tls.RequireAndVerifyClientCert,
			},
		},

		config: cfg,
	}

	handler.server = server

	if cfg.DNS != "" {
		resolver := dns.NewResolver(cfg.DNS, cfg.Net)
		server.resolver = &resolver
	}

	return server, nil
}
