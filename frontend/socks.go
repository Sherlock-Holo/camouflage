package frontend

import (
	"fmt"
	"net"

	"github.com/Sherlock-Holo/libsocks"
)

type Socks struct {
	socks libsocks.Socks
}

func NewSocks(conn net.Conn) (socks Frontend, err error) {
	s, err := libsocks.NewSocks(conn, nil)
	if err != nil {
		err = fmt.Errorf("new socks: %s", err)
		conn.Close()
		return
	}

	return &Socks{s}, nil
}

func (s *Socks) Handshake(b bool) error {
	if b {
		return s.socks.Reply(s.socks.LocalAddr().(*net.TCPAddr).IP, uint16(s.socks.LocalAddr().(*net.TCPAddr).Port), libsocks.Success)
	} else {
		return s.socks.Reply(s.socks.LocalAddr().(*net.TCPAddr).IP, uint16(s.socks.LocalAddr().(*net.TCPAddr).Port), libsocks.ServerFailed)
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

func (s *Socks) CloseWrite() error {
	return s.socks.CloseWrite()
}

func (s *Socks) CloseRead() error {
	return s.socks.CloseRead()
}
