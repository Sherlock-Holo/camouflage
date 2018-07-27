package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Sherlock-Holo/camouflage/client"
	configClient "github.com/Sherlock-Holo/camouflage/config/client"
	configServer "github.com/Sherlock-Holo/camouflage/config/server"
	"github.com/Sherlock-Holo/camouflage/server"
)

func main() {
	clientCfg := flag.String("client", "", "client config file, run on client mode")
	serverCfg := flag.String("server", "", "server config file, run on server mode")

	flag.Parse()

	if flag.NFlag() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	switch {
	case *clientCfg != "" && *serverCfg != "":
		fmt.Fprintln(os.Stderr, "can't use client and server at the same time")
		os.Exit(2)

	case *clientCfg != "":
		cfg, err := configClient.New(*clientCfg)
		if err != nil {
			log.Fatal(err)
		}
		c, err := client.New(cfg)
		if err != nil {
			log.Fatal(err)
		}
		c.Run()

	case *serverCfg != "":
		cfg, err := configServer.New(*serverCfg)
		if err != nil {
			log.Fatal(err)
		}

		s, err := server.NewServer(cfg)
		if err != nil {
			log.Fatal(err)
		}
		s.Run()
	}
}
