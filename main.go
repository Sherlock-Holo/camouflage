package main

import (
	"net"

	"github.com/Sherlock-Holo/camouflage/cmd"
)

func main() {
	// prefer go resolver
	net.DefaultResolver.PreferGo = true

	cmd.Execute()
}
