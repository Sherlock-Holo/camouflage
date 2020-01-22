package server

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

const (
	TypeWebsocket = "websocket"
	TypeQuic      = "quic"
)

type Config struct {
	Type             string   `toml:"type"` // support websocket and quic
	Host             string   `toml:"host"`
	Path             string   `toml:"path"`
	ListenAddr       string   `toml:"listen_addr"`
	Key              string   `toml:"key"`
	Crt              string   `toml:"crt"`
	WebRoot          string   `toml:"web_root"`
	WebKey           string   `toml:"web_key"`
	WebCrt           string   `toml:"web_crt"`
	WebHost          string   `toml:"web_host"`
	Timeout          Duration `toml:"timeout"`
	Secret           string   `toml:"secret"`
	Period           uint     `toml:"period"`
	ReverseProxyHost string   `toml:"reverse_proxy_host"`
	ReverseProxyKey  string   `toml:"reverse_proxy_key"`
	ReverseProxyCrt  string   `toml:"reverse_proxy_crt"`
	ReverseProxyAddr string   `toml:"reverse_proxy_addr"`
	Pprof            string   `toml:"pprof"`
}

type tomlConfig struct {
	Server Config `toml:"server"`
}

func New(path string) (Config, error) {
	config := new(tomlConfig)
	if _, err := toml.DecodeFile(path, config); err != nil {
		return Config{}, xerrors.Errorf("new server config failed: %w", err)
	}

	return config.Server, nil
}
