package client

import (
	"log"
	"testing"
)

func TestNew(t *testing.T) {
	client, err := New("/home/sherlock/go/src/github.com/Sherlock-Holo/camouflage/config/example.toml")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	t.Log(client)
}
