package client

import "github.com/Sherlock-Holo/link"

type connRequest struct {
	Socks   *Socks
	Success chan link.Link
	Err     chan error
}
