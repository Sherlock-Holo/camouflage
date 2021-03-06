package client

import (
	"net"

	"github.com/Sherlock-Holo/libsocks"
	errors "golang.org/x/xerrors"
)

type Socks struct {
	socks *libsocks.SocksServer
}

func NewSocks(conn net.Conn) (socks *Socks, err error) {
	s, err := libsocks.NewSocks(conn, nil)
	if err != nil {
		err = errors.Errorf("NewSocks failed: %w", err)
		conn.Close()
		return
	}

	return &Socks{
		socks: s,
	}, nil
}

func (s *Socks) Handshake(respType libsocks.ResponseType) error {
	if err := s.socks.Reply(s.socks.LocalAddr().(*net.TCPAddr).IP, uint16(s.socks.LocalAddr().(*net.TCPAddr).Port), respType); err != nil {
		return errors.Errorf("socks handshake failed: %w", err)
	}
	return nil
}

func (s *Socks) Target() []byte {
	return s.socks.Target.Bytes()
}

func (s *Socks) Read(p []byte) (n int, err error) {
	if n, err = s.socks.Read(p); err != nil {
		err = errors.Errorf("socks read failed: %w", err)
	}
	return
}

func (s *Socks) Write(p []byte) (n int, err error) {
	if n, err = s.socks.Write(p); err != nil {
		err = errors.Errorf("socks write failed: %w", err)
	}
	return
}

func (s *Socks) Close() error {
	return s.socks.Close()
}
