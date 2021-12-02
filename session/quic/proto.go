package quic

import (
	"github.com/lucas-clemente/quic-go"
)

const Proto = "quic"

const ErrorNoError quic.ApplicationErrorCode = 0x100

const (
	HandshakeSuccess = 1
	HandshakeFailed  = 2
)
