package cmd

import (
	config "github.com/Sherlock-Holo/camouflage/config/server"
	"github.com/Sherlock-Holo/camouflage/server"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var serverConfig string

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "server mode",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		if debug {
			log.SetLevel(log.DebugLevel)
		}

		cfg, err := config.New(serverConfig)
		if err != nil {
			return err
		}

		server, err := server.New(&cfg)
		if err != nil {
			return err
		}

		server.Run()

		return nil
	},
}
