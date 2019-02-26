package client

import (
	"net"

	"github.com/Sherlock-Holo/libsocks"
	"github.com/pkg/errors"
)

type Socks struct {
	socks  libsocks.Socks
	target []byte
}

func NewSocks(conn net.Conn) (socks *Socks, err error) {
	s, err := libsocks.NewSocks(conn, nil)
	if err != nil {
		err = errors.Wrap(err, "new socks failed")
		conn.Close()
		return
	}

	return &Socks{
		socks: s,
	}, nil
}

func (s *Socks) Handshake(ok bool) error {
	if ok {
		return errors.WithStack(s.socks.Reply(s.socks.LocalAddr().(*net.TCPAddr).IP, uint16(s.socks.LocalAddr().(*net.TCPAddr).Port), libsocks.Success))
	} else {
		return errors.WithStack(s.socks.Reply(s.socks.LocalAddr().(*net.TCPAddr).IP, uint16(s.socks.LocalAddr().(*net.TCPAddr).Port), libsocks.ServerFailed))
	}
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
