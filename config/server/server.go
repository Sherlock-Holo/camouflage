package server

import (
	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
)

type Config struct {
	Host        string `toml:"host"`
	ListenAddr  string `toml:"listen_addr"`
	ClientCAKey string `toml:"client_ca_key"`
	ClientCACrt string `toml:"client_ca_crt"`
	Key         string `toml:"key"`
	Crt         string `toml:"crt"`
	Path        string `toml:"path"`
	WebRoot     string `toml:"web_root"`

	Crl string `toml:"crl"`
}

type tomlConfig struct {
	Server Config `toml:"server"`
}

func New(path string) (Config, error) {
	config := new(tomlConfig)
	if _, err := toml.DecodeFile(path, config); err != nil {
		return Config{}, errors.WithStack(err)
	}

	return config.Server, nil
}
