package frontend

import "io"

const (
	SOCKS = iota
)

type Frontend interface {
	io.ReadWriteCloser

	Handshake(bool) error

	Target() string

	CloseWrite() error
}
