package main

import (
    "flag"
    "fmt"
    "log"
    "os"

    "github.com/Sherlock-Holo/camouflage/client"
    "github.com/Sherlock-Holo/camouflage/config"
    "github.com/Sherlock-Holo/camouflage/server"
)

func main() {
    clientCfg := flag.String("client", "", "client config file, run on client mode")
    serverCfg := flag.String("server", "", "server config file, run on server mode")

    flag.Parse()

    if flag.NFlag() == 0 {
        flag.Usage()
        os.Exit(1)
    }

    switch {
    case *clientCfg != "" && *serverCfg != "":
        fmt.Fprintln(os.Stderr, "can't use client and server at the same time")
        os.Exit(1)

    case *clientCfg != "":
        cfg, err := config.ReadClient(*clientCfg)
        if err != nil {
            log.Fatal(err)
        }
        c, err := client.NewClient(cfg)
        if err != nil {
            log.Fatal(err)
        }
        c.Run()

    case *serverCfg != "":
        cfg, err := config.ReadServer(*serverCfg)
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
