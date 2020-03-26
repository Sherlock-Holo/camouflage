package client

import (
	"context"
	"net"
)

type connRequest struct {
	Socks *Socks
	Conn  chan net.Conn
	Err   chan error
	Ctx   context.Context
}
