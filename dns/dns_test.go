package dns

import (
	"testing"
	"time"
)

func TestResolver_Query(t *testing.T) {
	resolver := NewResolver("114.114.114.114:53", "udp")

	if result, err := resolver.Query("www.qq.com", true, 10*time.Second); err != nil {
		t.Error(err)
	} else {
		t.Log(result)
	}
}
