package frontend

import (
	"net"

	"github.com/Sherlock-Holo/libsocks"
)

type Socks struct {
	socks libsocks.Socks
}

func (s *Socks) Handshake(b bool) error {
	if b {
		return s.socks.Reply(s.socks.LocalAddr().(*net.TCPAddr).IP, uint16(s.socks.LocalAddr().(*net.TCPAddr).Port), libsocks.Success)
	} else {
		return s.socks.Reply(s.socks.LocalAddr().(*net.TCPAddr).IP, uint16(s.socks.LocalAddr().(*net.TCPAddr).Port), libsocks.ServerFailed)
	}
}

func (s *Socks) Target() string {
	return s.socks.Target.String()
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
