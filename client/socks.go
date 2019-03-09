package client

import (
	"net"

	"github.com/Sherlock-Holo/libsocks"
	"golang.org/x/xerrors"
)

type Socks struct {
	socks  libsocks.Socks
	target []byte
}

func NewSocks(conn net.Conn) (socks *Socks, err error) {
	s, err := libsocks.NewSocks(conn, nil)
	if err != nil {
		err = xerrors.Errorf("NewSocks failed: %w", err)
		conn.Close()
		return
	}

	return &Socks{
		socks: s,
	}, nil
}

func (s *Socks) Handshake(respType libsocks.ResponseType) error {
	return xerrors.Errorf(
		"socks handshake failed: %w",
		s.socks.Reply(s.socks.LocalAddr().(*net.TCPAddr).IP, uint16(s.socks.LocalAddr().(*net.TCPAddr).Port), respType),
	)
}

func (s *Socks) Target() []byte {
	return s.socks.Target.Bytes()
}

func (s *Socks) Read(p []byte) (n int, err error) {
	return s.socks.Read(p)
}

func (s *Socks) Write(p []byte) (n int, err error) {
	return s.socks.Write(p)
}

func (s *Socks) Close() error {
	return s.socks.Close()
}
