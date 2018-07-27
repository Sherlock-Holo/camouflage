package server

import (
	"testing"
)

func TestNew(t *testing.T) {
	server, err := New("/home/sherlock/go/src/github.com/Sherlock-Holo/camouflage/config/example.toml")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(server)
}
