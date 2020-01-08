package client

import (
	"net"
)

type connRequest struct {
	Socks *Socks
	Conn  chan net.Conn
	Err   chan error
}
