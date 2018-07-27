package server

import (
	"github.com/pelletier/go-toml"
)

type Server struct {
	ListenAddr string `toml:"listen_addr"`
	ListenPort int    `toml:"listen_port"`
	Path       string `toml:"path"`

	CaCrt string `toml:"ca_crt"`
	Crt   string `toml:"crt"`
	Key   string `toml:"key"`

	DNS     string `toml:"dns"`
	DNSType string `toml:"dns_type"`
}

func New(path string) (*Server, error) {
	tree, err := toml.LoadFile(path)
	if err != nil {
		return nil, err
	}

	serverTree := tree.Get("server").(*toml.Tree)

	server := new(Server)
	if err = serverTree.Unmarshal(server); err != nil {
		return nil, err
	}

	return server, nil
}
