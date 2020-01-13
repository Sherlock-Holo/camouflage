package cmd

import (
	"github.com/Sherlock-Holo/camouflage/client"
	config "github.com/Sherlock-Holo/camouflage/config/client"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var clientConfig string

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "client mode",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		if debug {
			log.SetLevel(log.DebugLevel)
		}

		cfg, err := config.New(clientConfig)
		if err != nil {
			return err
		}

		c, err := client.New(&cfg)
		if err != nil {
			return err
		}

		c.Run()

		return nil
	},
}
