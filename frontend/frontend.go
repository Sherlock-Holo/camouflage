package frontend

import (
	"io"
	"net"
)

const (
	SOCKS = iota
	SHADOWSOCKS_CHACHA20_IETF
)

type Frontend interface {
	io.ReadWriteCloser

	Handshake(bool) error

	Target() []byte

	CloseWrite() error

	CloseRead() error
}

var Frontends = map[int]func(conn net.Conn) (Frontend, error){
	SOCKS: NewSocks,
}
