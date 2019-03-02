package server

import (
	"time"

	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) (err error) {
	d.Duration, err = time.ParseDuration(string(text))
	return
}

type Config struct {
	Host        string   `toml:"host"`
	ListenAddr  string   `toml:"listen_addr"`
	ClientCAKey string   `toml:"client_ca_key"`
	ClientCACrt string   `toml:"client_ca_crt"`
	Key         string   `toml:"key"`
	Crt         string   `toml:"crt"`
	Path        string   `toml:"path"`
	WebRoot     string   `toml:"web_root"`
	WebKey      string   `toml:"web_key"`
	WebCrt      string   `toml:"web_crt"`
	WebHost     string   `toml:"web_host"`
	Crl         string   `toml:"crl"`
	TLS13       bool     `toml:"TLS13"`
	Timeout     Duration `toml:"timeout"`
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
