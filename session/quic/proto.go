package quic

import (
	"github.com/lucas-clemente/quic-go"
)

const Proto = "quic"

const ErrorNoError quic.ErrorCode = 0x100

const (
	HandshakeSuccess = 1
	HandshakeFailed  = 2
)
