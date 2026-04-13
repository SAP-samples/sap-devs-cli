package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "First-time setup wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Welcome to sap-devs — your AI-first SAP developer toolkit.")
		fmt.Println()

		// Step 1: Sync content
		fmt.Println("Step 1/4: Downloading SAP developer content...")
		if err := runSyncForce(); err != nil {
			fmt.Printf("Warning: content sync failed (%v). Continuing with any cached content.\n", err)
		}

		paths, err := xdg.New()
		if err != nil {
			return err
		}

		// Step 2: Choose profile
		fmt.Println("\nStep 2/4: What kind of SAP developer are you?")
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		profiles, err := loader.LoadProfiles()
		if err != nil {
			return err
		}
		if len(profiles) > 0 {
			for i, p := range profiles {
				fmt.Printf("  [%d] %-25s %s\n", i+1, p.ID, p.Description)
			}
			fmt.Print("\nEnter number (or press Enter to skip): ")
			choice := readLine()
			if choice != "" {
				idx := 0
				fmt.Sscanf(choice, "%d", &idx)
				if idx >= 1 && idx <= len(profiles) {
					chosen := profiles[idx-1]
					if err := config.SaveProfile(paths.ConfigDir, &config.Profile{ID: chosen.ID}); err != nil {
						return err
					}
					fmt.Printf("Profile set to: %s\n", chosen.Name)
				}
			}
		} else {
			fmt.Println("No profiles found. Run 'sap-devs sync' then 'sap-devs profile list'.")
		}

		// Step 3: Inject into AI tools
		fmt.Println("\nStep 3/4: Inject SAP context into your AI tools?")
		fmt.Println("  This writes SAP developer context to your AI tool configuration files.")
		fmt.Print("  Inject now? [Y/n]: ")
		if answer := strings.ToLower(strings.TrimSpace(readLine())); answer == "" || answer == "y" {
			if err := runInjectGlobal(); err != nil {
				fmt.Printf("  Warning: inject failed (%v). You can run 'sap-devs inject' manually.\n", err)
			} else {
				fmt.Println("  SAP context injected into your AI tools.")
			}
		}

		// Step 4: Shell profile hook
		fmt.Println("\nStep 4/4: Add SAP tip to your terminal startup?")
		fmt.Println("  This adds 'sap-devs tip' to your shell profile so you see a tip each time you open a terminal.")
		fmt.Print("  Add it? [y/N]: ")
		if strings.ToLower(strings.TrimSpace(readLine())) == "y" {
			if err := addShellHook(); err != nil {
				fmt.Printf("  Could not auto-add hook: %v\n  Add 'sap-devs tip' to your shell profile manually.\n", err)
			} else {
				fmt.Println("  Added. Restart your terminal to see your first tip.")
			}
		}

		fmt.Println("\nSetup complete! Run 'sap-devs --help' to explore all commands.")
		fmt.Println("Run 'sap-devs inject' to re-inject after syncing new content.")
		return nil
	},
}

func runSyncForce() error {
	syncForce = true
	defer func() { syncForce = false }()
	return syncCmd.RunE(syncCmd, nil)
}

func readLine() string {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func addShellHook() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// Try common shell rc files in priority order
	candidates := []string{".zshrc", ".bashrc", ".bash_profile"}
	for _, rc := range candidates {
		path := home + "/" + rc
		if _, err := os.Stat(path); err == nil {
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			_, err = f.WriteString("\n# SAP developer tips\nsap-devs tip\n")
			f.Close()
			return err
		}
	}
	return fmt.Errorf("no shell rc file found (.zshrc, .bashrc, .bash_profile)")
}

func runInjectGlobal() error {
	injectProject = false
	injectDryRun = false
	injectTool = ""
	return injectCmd.RunE(injectCmd, nil)
}

func init() {
	rootCmd.AddCommand(initCmd)
}
