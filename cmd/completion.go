package cmd

import (
	"log"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generates bash/zsh completion scripts",
	Run: func(cmd *cobra.Command, args []string) {
		if err := rootCmd.GenBashCompletionFile("bash_completion"); err != nil {
			log.Printf("%+v", errors.WithStack(err))
		}

		if err := rootCmd.GenZshCompletionFile("zsh_completion"); err != nil {
			log.Printf("%+v", errors.WithStack(err))
		}
	},
}
