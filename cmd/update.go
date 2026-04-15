package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/credentials"
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
			fmt.Fprintln(os.Stderr, "cannot update a dev build")
			return nil
		}

		fmt.Println("Checking for updates...")

		paths, err := xdg.New()
		if err != nil {
			return err
		}
		token := credentials.Resolve(paths.ConfigDir)

		rel, newer, err := update.CheckLatest(repoURL, Version, token)
		if err != nil {
			return err
		}

		if !newer {
			fmt.Printf("sap-devs v%s is already up to date.\n", Version)
			return nil
		}

		fmt.Printf("Updating sap-devs v%s → %s...\n", Version, rel.TagName)
		if err := update.Install(repoURL, rel, token); err != nil {
			return err
		}

		fmt.Printf("✓ Updated to %s. Restart your shell if needed.\n", rel.TagName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
