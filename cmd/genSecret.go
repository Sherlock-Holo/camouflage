package cmd

import (
	"fmt"
	"github.com/Sherlock-Holo/camouflage/utils"
	"github.com/spf13/cobra"
)

var period uint

var genSecret = &cobra.Command{
	Use:   "genSecret",
	Short: fmt.Sprintf("generate TOTP secret, default period is %d", utils.DefaultPeriod),
	Args:  cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, _ []string) {
		secret := utils.GenTOTPSecret(uint(period))
		fmt.Printf("Your secret is %s\nYour period is %d\n", secret, period)
	},
}
