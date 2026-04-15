package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// verbose is bound to the --verbose / -v flag on versionCmd.
var verbose bool

// GetVersion returns the current Version (used in tests).
func GetVersion() string { return Version }

// SetVersion sets Version (used in tests to simulate real builds).
func SetVersion(v string) { Version = v }

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the sap-devs version",
	Run: func(cmd *cobra.Command, args []string) {
		out := cmd.OutOrStdout()
		errOut := cmd.ErrOrStderr()

		if verbose {
			fmt.Fprintln(out, i18n.Tf(i18n.ActiveLang, "version.verbose", map[string]any{"Version": Version}))
			fmt.Fprintln(out, i18n.Tf(i18n.ActiveLang, "version.verbose_go", map[string]any{"GoVersion": runtime.Version()}))
			fmt.Fprintln(out, i18n.Tf(i18n.ActiveLang, "version.verbose_os", map[string]any{"OS": runtime.GOOS, "Arch": runtime.GOARCH}))
		} else {
			fmt.Fprintln(out, Version)
		}
		if Version == "dev" {
			fmt.Fprintln(errOut, i18n.T(i18n.ActiveLang, "version.dev_warn"))
		}
	},
}

func init() {
	versionCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print Go version, OS, and architecture")
	rootCmd.AddCommand(versionCmd)
}
