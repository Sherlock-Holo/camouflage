package config

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

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

	MonitorAddr string
	MonitorPort int
}

type Server struct {
	BindAddr string
	BindPort int

	Path string

	CA []byte

	CrtFile string
	KeyFile string

	Net string
	DNS string
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
		if !strings.HasPrefix(caFile, "/") {
			caFile = filepath.Dir(cfgFile) + "/" + caFile
		}

		ca, err := ioutil.ReadFile(caFile)
		if err != nil {
			return Client{}, err
		}

		client.CA = ca
	} else {
		return Client{}, errors.New("need ca_crt")
	}

	if crtFile, ok := section["crt"]; ok {
		if !strings.HasPrefix(crtFile, "/") {
			crtFile = filepath.Dir(cfgFile) + "/" + crtFile
		}
		client.CrtFile = crtFile
	} else {
		return Client{}, errors.New("need crt")
	}

	if keyFile, ok := section["key"]; ok {
		if !strings.HasPrefix(keyFile, "/") {
			keyFile = filepath.Dir(cfgFile) + "/" + keyFile
		}
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

	if monitorAddr, ok := section["monitor_addr"]; ok {
		if monitorPortString, ok := section["monitor_port"]; ok {
			if monitorPort, err := strconv.Atoi(monitorPortString); err == nil {
				client.MonitorAddr = monitorAddr
				client.MonitorPort = monitorPort
			}
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
		if !strings.HasPrefix(caFile, "/") {
			caFile = filepath.Dir(cfgFile) + "/" + caFile
		}
		ca, err := ioutil.ReadFile(caFile)
		if err != nil {
			return Server{}, err
		}

		server.CA = ca
	} else {
		return Server{}, errors.New("need ca_crt")
	}

	if crtFile, ok := section["crt"]; ok {
		if !strings.HasPrefix(crtFile, "/") {
			crtFile = filepath.Dir(cfgFile) + "/" + crtFile
		}
		server.CrtFile = crtFile
	} else {
		return Server{}, errors.New("need crt")
	}

	if keyFile, ok := section["key"]; ok {
		if !strings.HasPrefix(keyFile, "/") {
			keyFile = filepath.Dir(cfgFile) + "/" + keyFile
		}
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

	return server, nil
}
