package cmd

import (
	"log"
	"os"

	config "github.com/Sherlock-Holo/camouflage/config/server"
	"github.com/Sherlock-Holo/camouflage/server"
	"github.com/pkg/errors"
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

		if cfg.TLS13 {
			if err := os.Setenv("GODEBUG", "tls13=1"); err != nil {
				log.Fatal(errors.WithStack(err))
			}
		}

		server := server.New(&cfg)
		server.Run()
		<-make(chan struct{})
	},
}
