package client

import "net"

type ListenerInfo struct {
	Key      []byte
	Listener net.Listener
}
