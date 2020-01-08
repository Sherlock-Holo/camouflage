package session

import (
	"context"
	"io"
	"net"
)

type Client interface {
	io.Closer

	Name() string

	OpenConn(ctx context.Context) (net.Conn, error)
}

type Server interface {
	io.Closer

	Name() string

	AcceptConn(ctx context.Context) (net.Conn, error)
}
