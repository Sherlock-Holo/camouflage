package server

import (
	"context"
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
	server http.Server
	config config.Server
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

	if handler.server.config.DNS != "" {
		if address.Type == 3 {
			ctx, _ := context.WithTimeout(context.Background(), handler.server.config.DNSTimeout)
			if addrs, err := dns.Resolver.LookupIPAddr(ctx, address.Host); err == nil {
				if dns.HasPublicIPv6() {
					ip := addrs[rand.Intn(len(addrs))].IP
					if ip.To4() != nil {
						address.Type = 1
						address.IP = ip.To4()
					} else {
						address.Type = 4
						address.IP = ip
					}

				} else {
					var v4s []net.IP
					for _, ip := range addrs {
						if v4 := ip.IP.To4(); v4 != nil {
							v4s = append(v4s, v4)
						}
					}

					if v4s == nil {
						log.Println("interfaces only have IPv4 address but DNS resolve result doesn't have IPv4")
						l.Close()
						return
					}

					ip := v4s[rand.Intn(len(v4s))]
					address.Type = 1
					address.IP = ip
				}
			} else {
				log.Println(err)
				l.Close()
				return
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
		dns.Resolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			return net.Dial(cfg.Net, cfg.DNS)
		}
	}

	return server, nil
}
