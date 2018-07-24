package client

import "github.com/Sherlock-Holo/link"

type managerStatus struct {
	manager *link.Manager
	count   int32

	// closed chan struct{}
	closed int32

	index int
}
