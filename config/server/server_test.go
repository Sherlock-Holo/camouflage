package server

import (
	"testing"
)

func TestNew(t *testing.T) {
	server, err := New("/home/sherlock/git/camouflage/config/example.toml")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%+v\n", server)
}
