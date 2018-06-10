package config

import (
    "testing"
)

func TestReadClient(t *testing.T) {
    client, err := ReadClient("example.conf")
    if err != nil {
        t.Error(err)
    }

    if client.RemoteAddr != "127.0.0.1" {
        t.Error("remote_addr wrong")
    }

    if client.RemotePort != 9876 {
        t.Error("remote_port wrong")
    }

    if client.SocksAddr != "127.0.0.1" {
        t.Error("socks_addr wrong")
    }

    if client.SocksPort != 9875 {
        t.Error("socks_port wrong")
    }

    if client.CrtFile != "/home/sherlock/go/src/go-learn/tls/tls/client/client.crt" {
        t.Error("crt wrong")
    }

    if client.KeyFile != "/home/sherlock/go/src/go-learn/tls/tls/client/client.key" {
        t.Error("key wrong")
    }
}

func TestReadServer(t *testing.T) {
    server, err := ReadServer("example.conf")
    if err != nil {
        t.Error(err)
    }

    if server.BindAddr != "127.0.0.1" {
        t.Error("bind_addr wrong")
    }

    if server.BindPort != 9876 {
        t.Error("bind_port wrong")
    }

    if server.CrtFile != "/home/sherlock/go/src/go-learn/tls/tls/server/server.crt" {
        t.Error("crt wrong")
    }

    if server.KeyFile != "/home/sherlock/go/src/go-learn/tls/tls/server/server.key" {
        t.Error("key wrong")
    }
}
