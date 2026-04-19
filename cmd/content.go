package cmd

import "github.com/spf13/cobra"

var contentCmd = &cobra.Command{
	Use:   "content",
	Short: "Manage content YAML files",
	Long:  "Browse, edit, and validate content YAML files across all content layers.",
}

func init() {
	rootCmd.AddCommand(contentCmd)
}
