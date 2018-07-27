package client

type Socks struct {
	ListenAddr string `toml:"listen_addr"`
	ListenPort int    `toml:"listen_port"`
}
