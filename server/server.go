package server

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "io"
    "log"
    "net"
    "net/http"

    "github.com/Sherlock-Holo/camouflage/config"
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

        go handle(l)
    }
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

    server := &Server{
        server: http.Server{
            Addr:    fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.BindPort),
            Handler: new(handler),
            TLSConfig: &tls.Config{
                ClientCAs:  pool,
                ClientAuth: tls.RequireAndVerifyClientCert,
            },
        },
        config: cfg,
    }

    return server, nil
}
