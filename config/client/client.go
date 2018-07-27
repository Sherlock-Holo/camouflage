package client

import (
	"github.com/Sherlock-Holo/camouflage/frontend"
	"github.com/pelletier/go-toml"
)

type Client struct {
	RemoteAddr string `toml:"remote_addr"`
	RemotePort int    `toml:"remote_port"`
	Path       string `toml:"path"`

	MaxLinks int `toml:"max_links"`

	CaCrt string `toml:"ca_crt"`

	Crt string `toml:"crt"`
	Key string `toml:"key"`

	MonitorAddr string `toml:"monitor_addr"`
	MonitorPort int    `toml:"monitor_port"`

	Shadowsocks []Shadowsocks `toml:"ignore"`
	Socks       []Socks       `toml:"ignore"`
}

func New(path string) (*Client, error) {
	tree, err := toml.LoadFile(path)
	if err != nil {
		return nil, err
	}

	clientTree := tree.Get("client").(*toml.Tree)
	client := new(Client)
	if err = clientTree.Unmarshal(client); err != nil {
		return nil, err
	}

	clientMap := clientTree.ToMap()
	treeMap := make(map[frontend.Type][]*toml.Tree)

	for _, v := range clientMap {
		switch m := v.(type) {
		case map[string]interface{}:
			littleTree, err := toml.TreeFromMap(m)
			if err != nil {
				return nil, err
			}

			switch littleTree.Get("type").(string) {
			case "socks":
				treeMap[frontend.SOCKS] = append(treeMap[frontend.SOCKS], littleTree)

			case "ss":
				treeMap[frontend.SHADOWSOCKS_CHACHA20_IETF] = append(treeMap[frontend.SHADOWSOCKS_CHACHA20_IETF], littleTree)
			}
		}
	}

	socksTrees := treeMap[frontend.SOCKS]
	for _, socksTree := range socksTrees {
		socks := new(Socks)
		if err = socksTree.Unmarshal(socks); err != nil {
			return nil, err
		}

		client.Socks = append(client.Socks, *socks)
	}

	ssTrees := treeMap[frontend.SHADOWSOCKS_CHACHA20_IETF]
	for _, ssTree := range ssTrees {
		ss := new(Shadowsocks)
		if err = ssTree.Unmarshal(ss); err != nil {
			return nil, err
		}

		client.Shadowsocks = append(client.Shadowsocks, *ss)
	}

	return client, nil
}
