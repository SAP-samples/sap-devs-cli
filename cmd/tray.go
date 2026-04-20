package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/credentials"
	"github.com/SAP-samples/sap-devs-cli/internal/trayctl"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
	"github.com/spf13/cobra"
)

var trayCmd = &cobra.Command{
	Use:   "tray",
	Short: "Manage the optional GUI tray companion",
	Long: `Manage the sap-devs system tray companion — an optional graphical dashboard
that shows sync status, active profile, and injected tools at a glance.

Note: The tray companion uses Wails v3 (currently in alpha).
This is an optional enhancement — all CLI features work without it.`,
}

var trayInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Download and install the tray companion binary",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}

		mgr := &trayctl.Manager{
			CacheDir: paths.CacheDir,
			Token:    credentials.Resolve(paths.ConfigDir),
			Version:  Version,
		}

		out := cmd.OutOrStdout()
		fmt.Fprintln(out, "Downloading sap-devs-tray...")
		if err := mgr.Install(); err != nil {
			return err
		}

		fmt.Fprintln(out, "Verifying...")
		if err := mgr.Verify(); err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}

		fmt.Fprintln(out, "Tray companion installed successfully.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Note: The sap-devs tray companion uses Wails v3 (currently in alpha).")
		fmt.Fprintln(out, "This is an optional enhancement — all CLI features work without it.")
		fmt.Fprintln(out, "If you encounter issues, run `sap-devs tray uninstall` to remove it.")
		fmt.Fprintln(out)

		fmt.Fprint(out, "Start tray automatically on login? [Y/n] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "" || answer == "y" || answer == "yes" {
			if err := mgr.RegisterAutostart(); err != nil {
				fmt.Fprintf(out, "Warning: could not register autostart: %v\n", err)
			} else {
				fmt.Fprintln(out, "Autostart registered.")
				cfg, _ := config.Load(paths.ConfigDir)
				cfg.Tray.Autostart = true
				_ = cfg.Save(paths.ConfigDir)
			}
		}

		return nil
	},
}

var trayUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the tray companion",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		mgr := &trayctl.Manager{CacheDir: paths.CacheDir, Version: Version}

		mgr.UnregisterAutostart()
		if err := mgr.Uninstall(); err != nil {
			return err
		}

		cfg, _ := config.Load(paths.ConfigDir)
		cfg.Tray.Autostart = false
		_ = cfg.Save(paths.ConfigDir)

		fmt.Fprintln(cmd.OutOrStdout(), "Tray companion uninstalled.")
		return nil
	},
}

var trayStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Launch the tray companion",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		mgr := &trayctl.Manager{CacheDir: paths.CacheDir}
		if err := mgr.Start(); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Tray companion started.")
		return nil
	},
}

var trayStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running tray companion",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		mgr := &trayctl.Manager{CacheDir: paths.CacheDir}
		if err := mgr.Stop(); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Tray companion stopped.")
		return nil
	},
}

var trayStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show tray companion status",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		mgr := &trayctl.Manager{CacheDir: paths.CacheDir, Version: Version}
		out := cmd.OutOrStdout()

		if !mgr.IsInstalled() {
			fmt.Fprintln(out, "Tray: not installed")
			fmt.Fprintln(out, "Run `sap-devs tray install` to download the tray companion.")
			return nil
		}

		running := "stopped"
		if mgr.IsRunning() {
			running = "running"
		}

		cfg, _ := config.Load(paths.ConfigDir)
		autostart := "disabled"
		if cfg.Tray.Autostart {
			autostart = "enabled"
		}

		fmt.Fprintf(out, "Tray:      installed (%s)\n", running)
		fmt.Fprintf(out, "Autostart: %s\n", autostart)
		fmt.Fprintf(out, "Binary:    %s\n", mgr.BinaryPath())
		return nil
	},
}

func init() {
	trayCmd.AddCommand(trayInstallCmd)
	trayCmd.AddCommand(trayUninstallCmd)
	trayCmd.AddCommand(trayStartCmd)
	trayCmd.AddCommand(trayStopCmd)
	trayCmd.AddCommand(trayStatusCmd)
	rootCmd.AddCommand(trayCmd)
}
