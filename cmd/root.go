package cmd

import (
	"github.com/Sherlock-Holo/camouflage/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	errors "golang.org/x/xerrors"
)

var rootCmd = &cobra.Command{
	Use:     "camouflage",
	Short:   "camouflage is a mux websocket over TLS proxy",
	Version: version,
}

var debug bool

func Execute() {
	rootCmd.AddCommand(
		clientCmd,
		serverCmd,
		genSecret,
	)

	clientCmd.Flags().StringVarP(&clientConfig, "file", "f", "", "client config file")
	serverCmd.Flags().StringVarP(&serverConfig, "file", "f", "", "server config file")

	genSecret.Flags().UintVarP(&period, "period", "p", utils.DefaultPeriod, "TOTP period")

	clientCmd.Flags().BoolVar(&debug, "debug", false, "debug log")
	serverCmd.Flags().BoolVar(&debug, "debug", false, "debug log")

	rootCmd.InitDefaultVersionFlag()

	if err := rootCmd.Execute(); err != nil {
		err = errors.Errorf("root cmd execute failed: %w", err)
		log.Fatalf("%+v", err)
	}
}
