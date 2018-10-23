package client

import (
	"log"
	"testing"
)

func TestNew(t *testing.T) {
	client, err := New("/home/sherlock/git/camouflage/config/example.toml")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	t.Logf("%+v\n", client)
}
