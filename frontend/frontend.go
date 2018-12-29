package frontend

import (
	"io"
	"net"
)

type Type int

const (
	SOCKS Type = iota
	SHADOWSOCKS_CHACHA20_IETF
)

type Frontend interface {
	io.ReadWriteCloser

	Handshake(bool) error

	Target() []byte

	CloseWrite() error

	CloseRead() error
}

var Frontends = map[Type]func(conn net.Conn, key []byte) (Frontend, error){
	SOCKS:                     NewSocks,
	SHADOWSOCKS_CHACHA20_IETF: NewShadowsocks,
}
