package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.3.0"

var rootCmd = &cobra.Command{
	Use:     "camouflage",
	Short:   "camouflage is a websocket over TLS proxy",
	Version: version,
}

func Execute() {
	rootCmd.AddCommand(clientCmd, serverCmd)

	rootCmd.InitDefaultVersionFlag()

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
