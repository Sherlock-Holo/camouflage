package dns

import (
	net2 "github.com/Sherlock-Holo/goutils/net"
	"net"
)

var Resolver = net.Resolver{
	PreferGo: true,
}

func HasPublicIPv6() bool {
	addrs, _ := net.InterfaceAddrs()

	if addrs == nil {
		return false
	}

	var v6Addrs []net.IP

	for _, addr := range addrs {
		if v6 := addr.(*net.IPNet).IP.To16(); v6 != nil {
			v6Addrs = append(v6Addrs, v6)
		}
	}

	if v6Addrs == nil {
		return false
	}

	for _, v6 := range v6Addrs {
		if net2.IsPublicIP(v6) {
			return true
		}
	}

	return false
}
