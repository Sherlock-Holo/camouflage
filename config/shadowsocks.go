package config

type Shadowsocks struct {
	MaxLinks   int    `toml:"max_links"`
	ListenAddr string `toml:"listen_addr"`
	ListenPort int    `toml:"listen_port"`
	Key        string `toml:"key"`
}
