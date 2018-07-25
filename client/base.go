package client

import "github.com/Sherlock-Holo/link"

type base struct {
	manager *link.Manager
	count   int32

	closed bool

	index int
}
