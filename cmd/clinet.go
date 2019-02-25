package cmd

import (
	"log"

	"github.com/Sherlock-Holo/camouflage/client"
	config "github.com/Sherlock-Holo/camouflage/config/client"
	"github.com/spf13/cobra"
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "client mode",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.New(args[0])
		if err != nil {
			log.Fatalf("%v", err)
		}
		c, err := client.New(&cfg)
		if err != nil {
			log.Fatalf("%v", err)
		}
		c.Run()
	},
}
