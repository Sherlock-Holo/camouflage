package client

import (
	"net"

	"github.com/lucas-clemente/quic-go"
)

type connection struct {
	quic.Stream

	localAddr  net.Addr
	remoteAddr net.Addr
}

func (c connection) LocalAddr() net.Addr {
	return c.localAddr
}

func (c connection) RemoteAddr() net.Addr {
	return c.remoteAddr
}
