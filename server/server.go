package server

import (
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"path/filepath"

	"github.com/Sherlock-Holo/camouflage/config/server"
	"github.com/Sherlock-Holo/camouflage/nic"
	websocket2 "github.com/Sherlock-Holo/goutils/websocket"
	"github.com/Sherlock-Holo/libsocks"
	"github.com/Sherlock-Holo/link"
	"github.com/gorilla/websocket"
)

var dialer = net.Dialer{}

type Server struct {
	noTLS    bool
	addr     string
	services []*server.Service
}

type camouflageService struct {
	token             string
	webRoot           string
	upgrader          websocket.Upgrader
	disableInvalidLog bool
}

func (cs *camouflageService) serviceProxy(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("token") != cs.token || !websocket.IsWebSocketUpgrade(r) {
		cs.serviceInvalid(w, r)
		return
	}

	conn, err := cs.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		conn.Close()
		return
	}

	manager := link.NewManager(websocket2.NewWrapper(conn), link.KeepaliveConfig())

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

func (cs *camouflageService) serviceInvalid(w http.ResponseWriter, r *http.Request) {
	log.Println("an invalid request is detected from", r.RemoteAddr)
	cs.serviceWeb(w, r)
}

func (cs *camouflageService) serviceWeb(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(cs.webRoot, r.URL.Path))
}

func handle(l *link.Link) {
	address, err := libsocks.DecodeFrom(l)
	if err != nil {
		log.Println(err)
		l.Close()
		return
	}

	switch address.Type {
	case 3:
		if addrs, err := net.DefaultResolver.LookupIPAddr(context.Background(), address.Host); err == nil {
			if nic.HasPublicIPv6() {
				ip := addrs[rand.Intn(len(addrs))].IP
				if ipv4 := ip.To4(); ipv4 != nil {
					address.Type = 1
					address.IP = ipv4
				} else {
					address.Type = 4
					address.IP = ip
				}
			} else {
				var ipv4s []net.IP
				for _, ip := range addrs {
					if ipv4 := ip.IP.To4(); ipv4 != nil {
						ipv4s = append(ipv4s, ipv4)
					}
				}

				if ipv4s == nil {
					log.Println("interfaces only have IPv4 address but DNS resolve result doesn't have IPv4")
					l.Close()
					return
				}

				ip := ipv4s[rand.Intn(len(ipv4s))]
				address.Type = 1
				address.IP = ip
			}
		} else {
			log.Println("resolve DNS", err)
			l.Close()
			return
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
	if s.noTLS {
		return s.noTLSRun()
	}

	return s.tlsRun()
}

func (s *Server) tlsRun() *http.Server {
	tlsConfig := new(tls.Config)
	tlsConfig.PreferServerCipherSuites = true
	tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h2")

	for _, service := range s.services {
		crtBytes, err := ioutil.ReadFile(service.Crt)
		if err != nil {
			log.Fatalf("service name: %s, read crt file error: %s", service.ServiceName, err)
		}
		keyBytes, err := ioutil.ReadFile(service.Key)
		if err != nil {
			log.Fatalf("service name: %s, read private key file error: %s", service.ServiceName, err)
		}
		certificate, err := tls.X509KeyPair(crtBytes, keyBytes)
		if err != nil {
			log.Fatalf("service name: %s, X509 key pair error: %s", service.ServiceName, err)
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, certificate)
	}

	tcpListener, err := net.Listen("tcp", s.addr)
	if err != nil {
		log.Fatalf("addr: %s, tcp listen error: %s", s.addr, err)
	}

	tlsListener := tls.NewListener(tcpListener, tlsConfig)

	mux := http.NewServeMux()

	for _, service := range s.services {
		cs := camouflageService{}

		cs.token = service.Token
		host := service.Host
		mux.HandleFunc(host+service.Path, cs.serviceProxy)

		if service.WebRoot != "" {
			cs.webRoot = service.WebRoot
			if !service.DisableInvalidLog {
				mux.HandleFunc(host+"/", cs.serviceInvalid)
			} else {
				mux.HandleFunc(host+"/", cs.serviceWeb)
			}
		}

		log.Println("service", service.ServiceName, "inited")
	}

	httpServer := &http.Server{Handler: mux}

	go func() {
		if err := httpServer.Serve(tlsListener); err != http.ErrServerClosed {
			log.Println(err)
		}
	}()

	return httpServer
}

func (s *Server) noTLSRun() *http.Server {
	tcpListener, err := net.Listen("tcp", s.addr)
	if err != nil {
		log.Fatalf("addr: %s, tcp listen error: %s", s.addr, err)
	}
	mux := http.NewServeMux()

	for _, service := range s.services {
		cs := camouflageService{}

		cs.token = service.Token
		host := service.Host
		mux.HandleFunc(host+service.Path, cs.serviceProxy)

		if service.WebRoot != "" {
			cs.webRoot = service.WebRoot
			if !service.DisableInvalidLog {
				mux.HandleFunc(host+"/", cs.serviceInvalid)
			} else {
				mux.HandleFunc(host+"/", cs.serviceWeb)
			}
		}

		log.Println("service", service.ServiceName, "inited")
	}

	httpServer := &http.Server{Handler: mux}

	go func() {
		if err := httpServer.Serve(tcpListener); err != http.ErrServerClosed {
			log.Println(err)
		}
	}()

	return httpServer
}

func New(cfg *server.Config) (servers []*Server) {
	net.DefaultResolver.PreferGo = true
	if cfg.DNS != "" {
		net.DefaultResolver.Dial = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, cfg.DNSType, cfg.DNS)
		}
	}

	for addr, services := range cfg.TLSServices {
		s := new(Server)
		s.addr = addr
		s.services = services

		servers = append(servers, s)
	}

	for addr, services := range cfg.NoTLSServices {
		s := new(Server)
		s.noTLS = true
		s.addr = addr
		s.services = services

		servers = append(servers, s)
	}

	return
}
