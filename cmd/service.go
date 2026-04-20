package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/service"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage background sync scheduler",
	Long:  "Register or remove an OS-native scheduled task that runs sap-devs sync + inject on a configurable interval.",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Register the OS scheduler entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}

		binaryPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("could not determine binary path: %w", err)
		}
		binaryPath, err = filepath.EvalSymlinks(binaryPath)
		if err != nil {
			return fmt.Errorf("could not resolve binary path: %w", err)
		}

		sched := service.New(paths.CacheDir)
		interval := cfg.Service.Interval
		if interval == 0 {
			interval = 6 * time.Hour
		}

		if err := sched.Install(interval, binaryPath); err != nil {
			return fmt.Errorf("could not install scheduler: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Background scheduler installed (every %s).\n", interval)
		fmt.Fprintf(cmd.OutOrStdout(), "Runs: %s sync && %s inject --no-sync\n", binaryPath, binaryPath)
		return nil
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the OS scheduler entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		sched := service.New(paths.CacheDir)
		if err := sched.Uninstall(); err != nil {
			return fmt.Errorf("could not uninstall scheduler: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Background scheduler removed.")
		return nil
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show scheduler status",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		sched := service.New(paths.CacheDir)
		st, err := sched.Status()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		if !st.Installed {
			fmt.Fprintln(out, "Scheduler: not installed")
			fmt.Fprintln(out, "Run `sap-devs service install` to enable background sync.")
			return nil
		}
		fmt.Fprintln(out, "Scheduler: installed")
		if !st.LastRun.IsZero() {
			fmt.Fprintf(out, "Last run:  %s\n", st.LastRun.Format(time.RFC3339))
		}
		if !st.NextRun.IsZero() {
			fmt.Fprintf(out, "Next run:  %s\n", st.NextRun.Format(time.RFC3339))
		}
		return nil
	},
}

func init() {
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	rootCmd.AddCommand(serviceCmd)
}
