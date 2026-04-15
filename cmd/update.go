package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/credentials"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/update"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

// repoURL is the canonical repository URL used for update checks and downloads.
// Accessible to all files in package cmd (e.g. root.go background check).
const repoURL = "https://github.tools.sap/developer-relations/sap-devs-cli"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update sap-devs to the latest release",
	Long:  `Check for a newer release on GitHub and install it if found.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if Version == "dev" {
			fmt.Fprintln(os.Stderr, i18n.T(i18n.ActiveLang, "update.no_dev"))
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "update.checking"))

		paths, err := xdg.New()
		if err != nil {
			return err
		}
		token := credentials.Resolve(paths.ConfigDir)

		rel, newer, err := update.CheckLatest(repoURL, Version, token)
		if err != nil {
			return err
		}

		if rel == nil || !newer {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "update.up_to_date", map[string]any{"Version": Version}))
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "update.installing", map[string]any{"From": Version, "To": rel.TagName}))
		if err := update.Install(repoURL, rel, token); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "update.done", map[string]any{"TagName": rel.TagName}))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
