package client

import (
	"time"

	"github.com/BurntSushi/toml"
	errors "golang.org/x/xerrors"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) (err error) {
	d.Duration, err = time.ParseDuration(string(text))
	return
}

const (
	TypeWebsocket = "websocket"
	TypeQuic      = "quic"
)

type Config struct {
	Type       string   `toml:"type"` // support websocket and quic
	Host       string   `toml:"host"`
	Path       string   `toml:"path"`
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
		return Config{}, errors.Errorf("new client config failed: %w", err)
	}

	switch config.Client.Type {
	default:
		return Config{}, errors.Errorf("unknown type %s", config.Client.Type)

	case TypeWebsocket, TypeQuic:
	}

	return config.Client, nil
}
