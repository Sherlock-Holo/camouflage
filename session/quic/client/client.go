package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"math"
	"net"
	"sync"
	"time"

	"github.com/Sherlock-Holo/camouflage/session"
	quicSession "github.com/Sherlock-Holo/camouflage/session/quic"
	"github.com/Sherlock-Holo/camouflage/utils"
	"github.com/lucas-clemente/quic-go"
	log "github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	errors "golang.org/x/xerrors"
)

type Option interface {
	apply(client *quicClient)
}

type debugCA []byte

func (d debugCA) apply(client *quicClient) {
	if client.tlsConfig.RootCAs == nil {
		client.tlsConfig.RootCAs = x509.NewCertPool()
	}

	client.tlsConfig.RootCAs.AppendCertsFromPEM(d)
}

func WithDebugCA(ca []byte) Option {
	return debugCA(ca)
}

type quicClient struct {
	addr string

	tlsConfig *tls.Config

	secret string
	period uint

	quicSession  quic.Session
	connectMutex sync.Mutex
	closed       atomic.Bool
}

func NewClient(quicAddr, totpSecret string, totpPeriod uint, opts ...Option) *quicClient {
	client := &quicClient{
		addr: quicAddr,

		tlsConfig: &tls.Config{
			NextProtos: []string{quicSession.Proto},
		},

		secret: totpSecret,
		period: totpPeriod,
	}

	for _, opt := range opts {
		opt.apply(client)
	}

	return client
}

func (q *quicClient) Name() string {
	return "quic"
}

func (q *quicClient) Close() error {
	if q.closed.CAS(false, true) {
		return q.quicSession.CloseWithError(quicSession.ErrorNoError, "")
	}

	return nil
}

func (q *quicClient) OpenConn(ctx context.Context) (net.Conn, error) {
	if q.closed.Load() {
		return nil, &net.OpError{
			Op:  "open",
			Net: q.Name(),
			Err: errors.New("session is closed"),
		}
	}

	q.connectMutex.Lock()

	for q.quicSession == nil {
		log.Debug("start quic connect")

		err := q.reconnect(ctx)
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			netErr := &net.OpError{
				Op:  "open",
				Net: q.Name(),
				Err: err,
			}

			// when connect timeout, quic session may can't recover
			_ = q.quicSession.CloseWithError(quicSession.ErrorNoError, "")
			q.quicSession = nil

			q.connectMutex.Unlock()

			return nil, errors.Errorf("connect quic failed: %w", netErr)

		default:
			err = errors.Errorf("quic connect failed: %w", err)
			log.Warnf("%+v", err)

			continue

		case err == nil:
		}

		log.Debug("quic connect success")
		break
	}

	select {
	case <-q.quicSession.Context().Done():
		if err := q.reconnect(ctx); err != nil {
			q.connectMutex.Unlock()

			return nil, errors.Errorf("reconnect quic failed: %w", err)
		}

	default:
	}

	q.connectMutex.Unlock()

	stream, err := q.quicSession.OpenStreamSync(ctx)
	if err != nil {
		if !quicSession.NoRecentNetwork(err) {
			return nil, errors.Errorf("open quic stream failed: %w", err)
		}

		log.Debug("session has no recent network, need reconnect")

		if err := q.reconnect(ctx); err != nil {
			return nil, errors.Errorf("reconnect quic failed: %w", err)
		}

		stream, err = q.quicSession.OpenStreamSync(ctx)
		if err != nil {
			return nil, errors.Errorf("open quic stream failed: %w", err)
		}
	}

	if raw := ctx.Value(session.PreData{}); raw != nil {
		preData, ok := raw.([]byte)
		if !ok {
			return nil, &net.OpError{
				Op:  "open",
				Net: q.Name(),
				Err: errors.New("invalid pre-data"),
			}
		}

		if _, err := stream.Write(preData); err != nil {
			return nil, errors.Errorf("write pre-data failed: %w", err)
		}
	}

	return quicSession.NewConnection(stream, q.quicSession.LocalAddr(), q.quicSession.RemoteAddr()), nil
}

func (q *quicClient) reconnect(ctx context.Context) error {
	if q.quicSession != nil {
		_ = q.quicSession.CloseWithError(quicSession.ErrorNoError, "")
	}

	q.quicSession = nil

	var (
		codeBuf bytes.Buffer
		errRet  error
	)

	for i := 0; i < 2; i++ {
		errRet = nil

		sess, err := quic.DialAddrContext(ctx, q.addr, q.tlsConfig, &quic.Config{
			KeepAlive:                             true,
			MaxIncomingStreams:                    math.MaxInt32,
			MaxReceiveConnectionFlowControlWindow: 300 * 1024 * 1024,
			MaxReceiveStreamFlowControlWindow:     10 * 1024 * 1024,
		})

		if err != nil {
			errRet = errors.Errorf("dial quic failed: %w", err)
			continue
		}

		code, err := utils.GenCode(q.secret, q.period)
		if err != nil {
			return errors.Errorf("generate TOTP code failed: %w", err)
		}

		length := len(code)

		codeBuf.WriteByte(byte(length))
		codeBuf.WriteString(code)

		codeBytes := codeBuf.Bytes()
		codeBuf.Reset()

		stream, err := sess.OpenStreamSync(ctx)
		if err != nil {
			_ = sess.CloseWithError(quicSession.ErrorNoError, "")
			errRet = errors.Errorf("open handshake stream failed: %w", err)

			continue
		}

		if _, err := stream.Write(codeBytes); err != nil {
			_ = stream.Close()
			_ = sess.CloseWithError(quicSession.ErrorNoError, "")

			errRet = errors.Errorf("send TOTP code failed: %w", err)
			continue
		}

		log.Debug("write handshake success")

		if err := stream.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
			_ = stream.Close()
			_ = sess.CloseWithError(quicSession.ErrorNoError, "")

			errRet = errors.Errorf("set read deadline failed: %w", err)
			continue
		}

		handshakeResp := make([]byte, 1)
		if _, err := stream.Read(handshakeResp); err != nil {
			_ = stream.Close()
			_ = sess.CloseWithError(quicSession.ErrorNoError, "")

			errRet = errors.Errorf("get TOTP handshake response failed: %w", err)
			continue
		}

		_ = stream.Close()

		switch handshakeResp[0] {
		case quicSession.HandshakeFailed:
			_ = sess.CloseWithError(quicSession.ErrorNoError, "")
			errRet = errors.New("connect failed: maybe TOTP secret is wrong")
			log.Debug("handshake failed")

			continue

		case quicSession.HandshakeSuccess:
		}

		q.quicSession = sess
		log.Debug("handshake success")

		break
	}

	return errRet
}
