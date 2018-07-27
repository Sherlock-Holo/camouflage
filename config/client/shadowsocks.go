package client

type Shadowsocks struct {
	ListenAddr string `toml:"listen_addr"`
	ListenPort int    `toml:"listen_port"`
	Key        string `toml:"key"`
}
