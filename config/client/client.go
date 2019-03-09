package client

import (
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/xerrors"
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
	DebugCA    string   `toml:"debug_ca"`
	ListenAddr string   `toml:"listen_addr"`
	Timeout    Duration `toml:"timeout"`
	Secret     string   `toml:"secret"`
	Period     uint     `toml:"period"`
}

type tomlConfig struct {
	Client Config `toml:"client"`
}

func New(path string) (Config, error) {
	config := new(tomlConfig)
	if _, err := toml.DecodeFile(path, config); err != nil {
		return Config{}, xerrors.Errorf("new client config failed: %w", err)
	}

	return config.Client, nil
}
