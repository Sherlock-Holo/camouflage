package cmd

import (
	"log"

	config "github.com/Sherlock-Holo/camouflage/config/server"
	"github.com/Sherlock-Holo/camouflage/server"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "server mode",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.New(args[0])
		if err != nil {
			log.Fatalf("%v", err)
		}
		server := server.New(&cfg)
		server.Run()
		<-make(chan struct{})
	},
}
