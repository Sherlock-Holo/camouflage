package client

import (
	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
	"time"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) (err error) {
	d.Duration, err = time.ParseDuration(string(text))
	return
}

type Config struct {
	RemoteAddr string   `toml:"remote_addr"`
	Path       string   `toml:"path"`
	Key        string   `toml:"key"`
	Crt        string   `toml:"crt"`
	DebugCA    string   `toml:"debug_ca"`
	ListenAddr string   `toml:"listen_addr"`
	Timeout    Duration `toml:"timeout"`
	TLS13      bool     `toml:"TLS13"`
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
