package client

import (
	"io"
	"net"

	"go.uber.org/atomic"
	errors "golang.org/x/xerrors"
)

type connection struct {
	net.Conn

	errHappened *atomic.Bool
}

func (c *connection) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	switch {
	case err == nil, errors.Is(err, io.EOF):

	default:
		c.errHappened.Store(true)
	}

	return
}

func (c *connection) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	if err != nil {
		c.errHappened.Store(true)
	}

	return
}
