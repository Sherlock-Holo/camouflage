package cmd

import (
	"log"

	"github.com/Sherlock-Holo/camouflage/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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
		genTOTPSecret,
	)

	rootCmd.AddCommand(completionCmd)

	clientCmd.Flags().StringVarP(&clientConfig, "file", "f", "", "client config file")
	serverCmd.Flags().StringVarP(&serverConfig, "file", "f", "", "server config file")
	genTOTPSecret.Flags().UintVarP(&period, "period", "p", utils.DefaultPeriod, "TOTP period")

	rootCmd.InitDefaultVersionFlag()

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("%v", errors.WithStack(err))
	}
}
