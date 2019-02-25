package client

import (
	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
)

type Config struct {
	RemoteAddr string `toml:"remote_addr"`
	Path       string `toml:"path"`
	Key        string `toml:"key"`
	Crt        string `toml:"crt"`
	ListenAddr string `toml:"listen_addr"`
}

type tomlConfig struct {
	Client Config `toml:"client"`
}

func New(path string) (Config, error) {
	config := new(tomlConfig)
	if _, err := toml.DecodeFile(path, config); err != nil {
		return Config{}, errors.WithStack(err)
	}

	return config.Client, nil
}
