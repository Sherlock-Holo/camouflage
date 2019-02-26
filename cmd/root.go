package cmd

import (
	"log"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "camouflage",
	Short:   "camouflage is a mux websocket over TLS proxy",
	Version: version,
}

func Execute() {
	rootCmd.AddCommand(clientCmd, serverCmd)

	rootCmd.InitDefaultVersionFlag()

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("%v", errors.WithStack(err))
	}
}
