package config

import (
	"errors"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/Unknwon/goconfig"
)

type Client struct {
	SocksAddr string
	SocksPort int

	RemoteAddr string
	RemotePort int

	Path string

	CA []byte

	CrtFile string
	KeyFile string

	MaxLinks int
}

type Server struct {
	BindAddr string
	BindPort int

	Path string

	CA []byte

	CrtFile string
	KeyFile string

	Net        string
	DNS        string
	DNSTimeout time.Duration
}

func ReadClient(cfgFile string) (Client, error) {
	cfg, err := goconfig.LoadConfigFile(cfgFile)
	if err != nil {
		return Client{}, err
	}

	section, err := cfg.GetSection("client")
	if err != nil {
		return Client{}, err
	}

	client := Client{}

	if remoteAddr, ok := section["remote_addr"]; ok {
		client.RemoteAddr = remoteAddr
	} else {
		return Client{}, errors.New("need remote_addr")
	}

	if remotePort, ok := section["remote_port"]; ok {
		client.RemotePort, err = strconv.Atoi(remotePort)
		if err != nil {
			return Client{}, err
		}
	} else {
		return Client{}, errors.New("need remote_port")
	}

	if path, ok := section["path"]; ok {
		client.Path = path
	} else {
		return Client{}, errors.New("need path")
	}

	if socksAddr, ok := section["socks_addr"]; ok {
		client.SocksAddr = socksAddr
	} else {
		return Client{}, errors.New("need socks_addr")
	}

	if socksPort, ok := section["socks_port"]; ok {
		client.SocksPort, err = strconv.Atoi(socksPort)
		if err != nil {
			return Client{}, err
		}
	} else {
		return Client{}, errors.New("need socks_port")
	}

	if caFile, ok := section["ca_crt"]; ok {
		ca, err := ioutil.ReadFile(caFile)
		if err != nil {
			return Client{}, err
		}

		client.CA = ca
	} else {
		return Client{}, errors.New("need ca_crt")
	}

	if crtFile, ok := section["crt"]; ok {
		client.CrtFile = crtFile
	} else {
		return Client{}, errors.New("need crt")
	}

	if keyFile, ok := section["key"]; ok {
		client.KeyFile = keyFile
	} else {
		return Client{}, errors.New("need key")
	}

	if maxLinksString, ok := section["max_links"]; ok {
		if maxLinks, err := strconv.Atoi(maxLinksString); err != nil {
			client.MaxLinks = 100
		} else {
			client.MaxLinks = maxLinks
		}
	}

	return client, nil
}

func ReadServer(cfgFile string) (Server, error) {
	cfg, err := goconfig.LoadConfigFile(cfgFile)
	if err != nil {
		return Server{}, err
	}

	section, err := cfg.GetSection("server")
	if err != nil {
		return Server{}, err
	}

	server := Server{}

	if bindAddr, ok := section["bind_addr"]; ok {
		server.BindAddr = bindAddr
	} else {
		return Server{}, errors.New("need bind_addr")
	}

	if bindPort, ok := section["bind_port"]; ok {
		server.BindPort, err = strconv.Atoi(bindPort)
		if err != nil {
			return Server{}, err
		}
	} else {
		return Server{}, errors.New("need bind_port")
	}

	if path, ok := section["path"]; ok {
		server.Path = path
	} else {
		return Server{}, errors.New("need path")
	}

	if caFile, ok := section["ca_crt"]; ok {
		ca, err := ioutil.ReadFile(caFile)
		if err != nil {
			return Server{}, err
		}

		server.CA = ca
	} else {
		return Server{}, errors.New("need ca_crt")
	}

	if crtFile, ok := section["crt"]; ok {
		server.CrtFile = crtFile
	} else {
		return Server{}, errors.New("need crt")
	}

	if keyFile, ok := section["key"]; ok {
		server.KeyFile = keyFile
	} else {
		return Server{}, errors.New("need key")
	}

	if dnsString, ok := section["dns"]; ok {
		split := strings.Split(dnsString, "#")

		switch strings.ToLower(split[0]) {
		case "udp":
			server.Net = "udp"
		case "tcp":
			server.Net = "tcp"
		default:
		}

		if server.Net != "" {
			server.DNS = split[1]
		}
	}

	if dnsTimeout, ok := section["dns_timeout"]; ok {
		if duration, err := time.ParseDuration(dnsTimeout + "s"); err != nil {
			server.DNSTimeout = 10 * time.Second
		} else {
			server.DNSTimeout = duration
		}
	}

	return server, nil
}
