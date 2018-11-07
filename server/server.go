package server

import (
	"context"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/Sherlock-Holo/camouflage/config/server"
	"github.com/Sherlock-Holo/camouflage/dns"
	websocket2 "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/libsocks"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
)

type Server struct {
	config   server.Server
	upgrader websocket.Upgrader
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("token") != s.config.Token || !websocket.IsWebSocketUpgrade(r) {
		s.invalidHandle(w, r)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
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

		go handle(s, l)
	}
}

func (s *Server) invalidHandle(w http.ResponseWriter, r *http.Request) {
	log.Println("an invalid request is detected from", r.RemoteAddr)
	if s.config.WebPage != "" {
		http.ServeFile(w, r, filepath.Join(s.config.WebPage, r.URL.Path))
	} else {
		http.NotFound(w, r)
	}
	return
}

func handle(handler *Server, l *link.Link) {
	address, err := libsocks.DecodeFrom(l)
	if err != nil {
		log.Println(err)
		l.Close()
		return
	}

	if handler.config.DNS != "" {
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
	mux := http.NewServeMux()

	mux.HandleFunc(s.config.Path, s.ServeHTTP)
	mux.HandleFunc("/", s.invalidHandle)

	log.Println(http.ListenAndServeTLS(
		net.JoinHostPort(s.config.ListenAddr, strconv.Itoa(s.config.ListenPort)),
		s.config.Crt,
		s.config.Key,
		mux,
	),
	)
}

func New(cfg *server.Server) (*Server, error) {
	s := &Server{
		config: *cfg,
	}

	if cfg.DNS != "" {
		dns.Resolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			return net.Dial(cfg.DNSType, cfg.DNS)
		}
	}

	return s, nil
}
