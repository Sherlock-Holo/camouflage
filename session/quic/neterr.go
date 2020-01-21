package quic

import (
	"strings"
)

func NoRecentNetwork(err error) bool {
	return strings.Contains(err.Error(), "NO_ERROR: No recent network activity")
}
