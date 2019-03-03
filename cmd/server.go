package cmd

import (
	"log"

	config "github.com/Sherlock-Holo/camouflage/config/server"
	"github.com/Sherlock-Holo/camouflage/server"
	"github.com/spf13/cobra"
)

var serverConfig string

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "server mode",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		cfg, err := config.New(serverConfig)
		if err != nil {
			log.Fatalf("%v", err)
		}

		server := server.New(&cfg)
		server.Run()
		<-make(chan struct{})
	},
}
