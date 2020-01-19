package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
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

func newQuicClient(quicAddr, totpSecret string, totpPeriod uint, opts ...Option) *quicClient {
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
		return q.quicSession.Close()
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
	defer q.connectMutex.Unlock()

	if q.quicSession == nil {
		for {
			err := q.reconnect(ctx)
			switch {
			case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
				netErr := &net.OpError{
					Op:  "open",
					Net: q.Name(),
					Err: err,
				}

				return nil, errors.Errorf("connect quic failed: %w", netErr)

			default:
				continue

			case err == nil:
			}

			log.Debug("quic connect success")
			break
		}
	}

	select {
	case <-q.quicSession.Context().Done():
		if err := q.reconnect(ctx); err != nil {
			return nil, errors.Errorf("reconnect quic failed: %w", err)
		}

	default:
	}

	var (
		stream  quic.Stream
		err     error
		codeBuf bytes.Buffer
	)

	for i := 0; i < 2; i++ {
		stream, err = q.quicSession.OpenStreamSync(ctx)
		if err != nil {
			return nil, errors.Errorf("open quic stream failed: %w", err)
		}

		code, err := utils.GenCode(q.secret, q.period)
		if err != nil {
			return nil, errors.Errorf("generate TOTP code failed: %w", err)
		}

		length := len(code)

		codeBuf.WriteByte(byte(length))
		codeBuf.WriteString(code)

		codeBytes := codeBuf.Bytes()
		codeBuf.Reset()

		if _, err := stream.Write(codeBytes); err != nil {
			return nil, errors.Errorf("send TOTP code failed: %w", err)
		}

		if err := stream.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return nil, errors.Errorf("set read deadline failed: %w", err)
		}

		handshakeResp := make([]byte, 1)
		if _, err := stream.Read(handshakeResp); err != nil {
			return nil, errors.Errorf("get TOTP handshake response failed: %w", err)
		}

		switch handshakeResp[0] {
		case quicSession.HandshakeFailed:
			if i == 1 {
				return nil, errors.New("connect failed: maybe TOTP secret is wrong")
			}

			continue

		case quicSession.HandshakeSuccess:
		}

		break
	}

	if raw := ctx.Value("pre-data"); raw != nil {
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

	return connection{
		Stream:     stream,
		localAddr:  q.quicSession.LocalAddr(),
		remoteAddr: q.quicSession.RemoteAddr(),
	}, nil

}

func (q *quicClient) reconnect(ctx context.Context) error {
	if q.quicSession != nil {
		_ = q.quicSession.Close()
	}

	session, err := quic.DialAddrContext(ctx, q.addr, q.tlsConfig, nil)
	if err != nil {
		return errors.Errorf("dial quic failed: %w", err)
	}

	q.quicSession = session

	return nil
}
