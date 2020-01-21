package server

import (
	"context"
	"crypto/tls"
	"io"
	"math"
	"net"
	"sync"

	quicSession "github.com/Sherlock-Holo/camouflage/session/quic"
	"github.com/Sherlock-Holo/camouflage/utils"
	"github.com/lucas-clemente/quic-go"
	log "github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	errors "golang.org/x/xerrors"
)

type Option interface {
	apply(link *quicServer)
}

type quicServer struct {
	tlsConfig *tls.Config

	listener quic.Listener

	secret string
	period uint

	sessionIdGen *atomic.Uint64
	sessionMap   sync.Map

	acceptChan chan net.Conn

	startOnce sync.Once
	closed    atomic.Bool
}

func NewServer(listenAddr, secret string, period uint, serverCert tls.Certificate, opts ...Option) (*quicServer, error) {
	server := &quicServer{
		tlsConfig: &tls.Config{
			PreferServerCipherSuites: true,
			NextProtos:               []string{quicSession.Proto},
			Certificates:             []tls.Certificate{serverCert},
		},

		secret: secret,
		period: period,

		sessionIdGen: atomic.NewUint64(0),

		acceptChan: make(chan net.Conn, 100),
	}

	for _, opt := range opts {
		opt.apply(server)
	}

	server.tlsConfig.BuildNameToCertificate()

	listener, err := quic.ListenAddr(listenAddr, server.tlsConfig, &quic.Config{
		KeepAlive:          true,
		MaxIncomingStreams: math.MaxInt32,
	})

	if err != nil {
		return nil, errors.Errorf("listen quic on %s failed: %w", listenAddr, err)
	}

	server.listener = listener

	log.Debug("quic server init")

	return server, nil
}

func (q *quicServer) Name() string {
	return "quic"
}

func (q *quicServer) Close() error {
	if q.closed.CAS(false, true) {
		_ = q.listener.Close()

		q.sessionMap.Range(func(_, value interface{}) bool {
			_ = value.(quic.Session).Close()

			return true
		})
	}

	return nil
}

func (q *quicServer) AcceptConn(ctx context.Context) (net.Conn, error) {
	log.Debug("accept connection")

	q.startOnce.Do(func() {
		go q.run()
	})

	if q.closed.Load() {
		return nil, &net.OpError{
			Op:  "open",
			Net: q.Name(),
			Err: errors.New("quic is closed"),
		}
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	case conn := <-q.acceptChan:
		log.Debug("start stream handshake")

		buf := make([]byte, 1)

		_, err := conn.Read(buf)
		if err != nil {
			_ = conn.Close()
			return nil, errors.Errorf("read handshake message length failed: %w", err)
		}

		length := int(buf[0])

		log.Debugf("handshake length %d", length)

		buf = make([]byte, length)

		if _, err := io.ReadFull(conn, buf); err != nil {
			_ = conn.Close()
			return nil, errors.Errorf("read handshake message failed: %w", err)
		}

		code := string(buf)

		ok, err := utils.VerifyCode(code, q.secret, q.period)
		if err != nil {
			_ = conn.Close()

			return nil, errors.Errorf("verify code error: %w", err)
		}

		if ok {
			if _, err := conn.Write([]byte{quicSession.HandshakeSuccess}); err != nil {
				_ = conn.Close()

				return nil, errors.Errorf("write handshake success response failed: %w", err)
			}
		} else {
			if _, err := conn.Write([]byte{quicSession.HandshakeFailed}); err != nil {
				_ = conn.Close()

				return nil, errors.Errorf("write handshake failed response failed: %w", err)
			}

			// don't return error, keep accept connection
			return q.AcceptConn(ctx)
		}

		log.Debug("stream accepted")

		return conn, nil
	}
}

func (q *quicServer) run() {
	log.Debug("run quic server")

	for {
		session, err := q.listener.Accept(context.Background())
		if err != nil {
			err = errors.Errorf("accept quic session failed: %w", err)
			log.Errorf("%+v", err)

			continue
		}

		log.Debug("accept quic session")

		id := q.sessionIdGen.Add(1) - 1
		q.sessionMap.Store(id, session)

		go q.handleSession(id, session)
	}
}

func (q *quicServer) handleSession(id uint64, session quic.Session) {
	defer func() {
		_ = session.Close()

		q.sessionMap.Delete(id)
	}()

	for {
		stream, err := session.AcceptStream(context.Background())
		if err != nil {
			// hack
			if quicSession.NoRecentNetwork(err) {
				log.Debug("session has no recent network, close it")
				return
			}

			err = errors.Errorf("accept quic stream failed: %w", err)
			log.Errorf("%+v", err)

			return
		}

		/*var netErr net.Error

		switch {
		case errors.As(err, &netErr) && (netErr.Temporary() || netErr.Timeout()):
			log.Debugf("ignore error: %v", netErr)
			continue

		default:
			err = errors.Errorf("accept quic stream failed: %w", err)
			log.Errorf("%+v", err)

			return

		case err == nil:
		}*/

		log.Debug("accept stream")

		select {
		default:
			log.Warn("accept queue is full")

			_ = stream.Close()

		case q.acceptChan <- quicSession.NewConnection(stream, session.LocalAddr(), session.RemoteAddr()):
		}

		log.Debug("stream enter chan")
	}
}
