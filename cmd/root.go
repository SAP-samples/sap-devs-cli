package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sap-devs",
	Short: "AI-first SAP developer toolkit",
	Long:  `sap-devs injects up-to-date SAP developer knowledge into your AI tools.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
