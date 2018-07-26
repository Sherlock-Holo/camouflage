package config

type Socks struct {
	MaxLinks   int    `toml:"max_links"`
	ListenAddr string `toml:"listen_addr"`
	ListenPort string `toml:"listen_port"`
}
