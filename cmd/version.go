package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
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
			fmt.Fprintf(out, "sap-devs %s\n", Version)
			fmt.Fprintf(out, "  go:      %s\n", runtime.Version())
			fmt.Fprintf(out, "  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		} else {
			fmt.Fprintln(out, Version)
		}
		if Version == "dev" {
			fmt.Fprintln(errOut, "(dev build: built without -ldflags version injection — auto-update is disabled)")
		}
	},
}

func init() {
	versionCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print Go version, OS, and architecture")
	rootCmd.AddCommand(versionCmd)
}
