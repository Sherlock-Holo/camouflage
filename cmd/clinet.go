package cmd

import (
	"log"

	"github.com/Sherlock-Holo/camouflage/client"
	config "github.com/Sherlock-Holo/camouflage/config/client"
	"github.com/spf13/cobra"
)

var clientConfig string

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "client mode",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		cfg, err := config.New(clientConfig)
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
