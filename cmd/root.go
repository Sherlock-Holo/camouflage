package cmd

import (
	"log"

	"github.com/Sherlock-Holo/camouflage/utils"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var rootCmd = &cobra.Command{
	Use:     "camouflage",
	Short:   "camouflage is a mux websocket over TLS proxy",
	Version: version,
}

func Execute() {
	rootCmd.AddCommand(
		clientCmd,
		serverCmd,
		genSecret,
	)

	clientCmd.Flags().StringVarP(&clientConfig, "file", "f", "", "client config file")
	serverCmd.Flags().StringVarP(&serverConfig, "file", "f", "", "server config file")
	genSecret.Flags().UintVarP(&period, "period", "p", utils.DefaultPeriod, "TOTP period")

	rootCmd.InitDefaultVersionFlag()

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("%+v", xerrors.Errorf("root cmd execute failed: %w", err))
	}
}
