package server

import (
	"net"
	"path/filepath"
	"strconv"

	"github.com/pelletier/go-toml"
)

type Config struct {
	DNS     string `toml:"dns"`
	DNSType string `toml:"dns_type"`

	TLSServices   map[string][]*Service `toml:"ignore"`
	NoTLSServices map[string][]*Service `toml:"ignore"`
}

type Service struct {
	ServiceName string `toml:"ignore"`

	ListenAddr string `toml:"listen_addr"`
	ListenPort int    `toml:"listen_port"`

	DisableInvalidLog bool `toml:"disable_invalid_request_log"`

	NoTLS bool `toml:"no_tls"`

	Token string
	Path  string

	WebRoot string `toml:"web_root"`

	Host string

	Crt string
	Key string
}

func New(path string) (*Config, error) {
	tree, err := toml.LoadFile(path)
	if err != nil {
		return nil, err
	}

	serverTree := tree.Get("server").(*toml.Tree)

	config := new(Config)
	if err = serverTree.Unmarshal(config); err != nil {
		return nil, err
	}
	config.TLSServices = make(map[string][]*Service)
	config.NoTLSServices = make(map[string][]*Service)

	for name, s := range serverTree.ToMap() {
		switch value := s.(type) {
		case map[string]interface{}:
			serviceTree, err := toml.TreeFromMap(value)
			if err != nil {
				return nil, err
			}
			service := new(Service)
			if err := serviceTree.Unmarshal(service); err != nil {
				return nil, err
			}
			service.ServiceName = name
			addr := net.JoinHostPort(service.ListenAddr, strconv.Itoa(service.ListenPort))

			if !service.NoTLS {
				config.TLSServices[addr] = append(config.TLSServices[addr], service)
			} else {
				config.NoTLSServices[addr] = append(config.NoTLSServices[addr], service)
			}
		}
	}

	for _, services := range config.TLSServices {
		for i := range services {
			if !filepath.IsAbs(services[i].Crt) {
				services[i].Crt = filepath.Join(filepath.Dir(path), services[i].Crt)
				services[i].Key = filepath.Join(filepath.Dir(path), services[i].Key)

				if services[i].WebRoot != "" && !filepath.IsAbs(services[i].WebRoot) {
					services[i].WebRoot = filepath.Join(filepath.Dir(path), services[i].WebRoot)
				}
			}
		}
	}

	for _, services := range config.NoTLSServices {
		for i := range services {
			if !filepath.IsAbs(services[i].Crt) {
				services[i].Crt = filepath.Join(filepath.Dir(path), services[i].Crt)
				services[i].Key = filepath.Join(filepath.Dir(path), services[i].Key)

				if services[i].WebRoot != "" && !filepath.IsAbs(services[i].WebRoot) {
					services[i].WebRoot = filepath.Join(filepath.Dir(path), services[i].WebRoot)
				}
			}
		}
	}

	return config, nil
}
