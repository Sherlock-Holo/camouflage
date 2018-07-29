package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/Sherlock-Holo/camouflage/config/server"
	"github.com/Sherlock-Holo/camouflage/dns"
	websocket2 "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/libsocks"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
)

type Server struct {
	server http.Server
	config server.Server
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

	manager := link.NewManager(websocket2.NewWrapper(conn), link.KeepaliveConfig)

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
			if addrs, err := dns.Resolver.LookupIPAddr(context.Background(), address.Host); err == nil {
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
				log.Println("resolve DNS", err)
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
	log.Println(s.server.ListenAndServeTLS(s.config.Crt, s.config.Key))
}

func New(cfg *server.Server) (*Server, error) {
	pool := x509.NewCertPool()
	caFile, err := os.Open(cfg.CaCrt)
	if err != nil {
		return nil, err
	}

	CA, err := ioutil.ReadAll(caFile)
	if err != nil {
		return nil, err
	}

	pool.AppendCertsFromPEM(CA)

	handler := new(handler)

	server := &Server{
		server: http.Server{
			Addr:    net.JoinHostPort(cfg.ListenAddr, strconv.Itoa(cfg.ListenPort)),
			Handler: handler,
			TLSConfig: &tls.Config{
				ClientCAs:  pool,
				ClientAuth: tls.RequireAndVerifyClientCert,
			},
		},

		config: *cfg,
	}

	handler.server = server

	if cfg.DNS != "" {
		dns.Resolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			return net.Dial(cfg.DNSType, cfg.DNS)
		}
	}

	return server, nil
}
