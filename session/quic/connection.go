package quic

import (
	"net"

	"github.com/lucas-clemente/quic-go"
)

type Connection struct {
	quic.Stream

	localAddr  net.Addr
	remoteAddr net.Addr
}

func NewConnection(stream quic.Stream, localAddr, remoteAddr net.Addr) *Connection {
	return &Connection{
		Stream:     stream,
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}
}

func (c Connection) LocalAddr() net.Addr {
	return c.localAddr
}

func (c Connection) RemoteAddr() net.Addr {
	return c.remoteAddr
}
