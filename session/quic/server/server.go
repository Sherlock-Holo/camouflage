package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"math"
	"net"
	"sync"
	"time"

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
		KeepAlive:                             true,
		MaxIncomingStreams:                    math.MaxInt32,
		MaxReceiveConnectionFlowControlWindow: 50 * 1024 * 1024,
		MaxReceiveStreamFlowControlWindow:     10 * 1024 * 1024,
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

	timeout, timeoutFunc := context.WithTimeout(context.Background(), 30*time.Second)
	defer timeoutFunc()

	success, err := q.sessionHandshake(timeout, session)
	if err != nil {
		err = errors.Errorf("session handshake failed: %w", err)
		log.Errorf("%+v", err)
		return
	}

	if !success {
		return
	}

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

func (q *quicServer) sessionHandshake(ctx context.Context, session quic.Session) (success bool, err error) {
	stream, err := session.AcceptStream(ctx)
	if err != nil {
		// hack
		if quicSession.NoRecentNetwork(err) {
			log.Debug("session has no recent network, close it")
			return false, nil
		}

		err = errors.Errorf("accept handshake quic stream failed: %w", err)

		return false, err
	}

	defer func() {
		_ = stream.Close()
	}()

	// reduce udp read syscall
	bufStream := bufio.NewReader(stream)

	buf := make([]byte, 1)

	if err := stream.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return false, errors.Errorf("set handshake quic stream read deadline failed: %w", err)
	}

	if _, err := bufStream.Read(buf); err != nil {
		return false, errors.Errorf("read handshake message length failed: %w", err)
	}

	length := int(buf[0])

	log.Debugf("handshake length %d", length)

	buf = make([]byte, length)

	if err := stream.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return false, errors.Errorf("set handshake quic stream read deadline failed: %w", err)
	}

	if _, err := io.ReadFull(bufStream, buf); err != nil {
		return false, errors.Errorf("read handshake message failed: %w", err)
	}

	code := string(buf)

	ok, err := utils.VerifyCode(code, q.secret, q.period)
	if err != nil {
		return false, errors.Errorf("verify code error: %w", err)
	}

	if ok {
		if _, err := stream.Write([]byte{quicSession.HandshakeSuccess}); err != nil {
			return false, errors.Errorf("write handshake success response failed: %w", err)
		}
	} else {
		if _, err := stream.Write([]byte{quicSession.HandshakeFailed}); err != nil {
			return false, errors.Errorf("write handshake failed response failed: %w", err)
		}

		return false, nil
	}

	return true, nil
}
