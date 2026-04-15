package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/credentials"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/shellhook"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "First-time setup wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "Welcome to sap-devs — your AI-first SAP developer toolkit.")
		fmt.Fprintln(cmd.OutOrStdout())

		paths, err := xdg.New()
		if err != nil {
			return err
		}

		// Step 1: GitHub authentication (optional)
		fmt.Fprintln(cmd.OutOrStdout(), "Step 1/5: GitHub authentication (optional)")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "  sap-devs syncs content from github.tools.sap, which requires a Personal")
		fmt.Fprintln(cmd.OutOrStdout(), "  Access Token if you are inside the SAP corporate network. If you are")
		fmt.Fprintln(cmd.OutOrStdout(), "  outside SAP or already have GITHUB_TOOLS_SAP_TOKEN set in your")
		fmt.Fprintln(cmd.OutOrStdout(), "  environment, press Enter to skip.")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprint(cmd.OutOrStdout(), "  GitHub token (press Enter to skip): ")
		raw, termErr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(cmd.OutOrStdout()) // newline after hidden input
		if termErr != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "  Note: interactive token input unavailable. Run 'sap-devs config token <value>' after setup to authenticate.")
		} else {
			token := strings.TrimSpace(string(raw))
			if token != "" {
				if storeErr := credentials.Store(paths.ConfigDir, token); storeErr != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  Warning: could not store token (%v).\n", storeErr)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "  Token stored securely.")
				}
			}
		}

		// Step 2: Sync content
		fmt.Fprintln(cmd.OutOrStdout(), "\nStep 2/5: Downloading SAP developer content...")
		if err := runSyncForce(); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Warning: content sync failed (%v). Continuing with any cached content.\n", err)
		}

		// Step 3: Choose profile
		fmt.Fprintln(cmd.OutOrStdout(), "\nStep 3/5: What kind of SAP developer are you?")
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
				fmt.Fprintf(cmd.OutOrStdout(), "  [%d] %-25s %s\n", i+1, p.ID, p.Description)
			}
			fmt.Fprint(cmd.OutOrStdout(), "\nEnter number (or press Enter to skip): ")
			choice := readLine()
			if choice != "" {
				idx := 0
				fmt.Sscanf(choice, "%d", &idx)
				if idx >= 1 && idx <= len(profiles) {
					chosen := profiles[idx-1]
					if err := config.SaveProfile(paths.ConfigDir, &config.Profile{ID: chosen.ID}); err != nil {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Profile set to: %s\n", chosen.Name)
				}
			}
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "No profiles found. Run 'sap-devs sync' then 'sap-devs profile list'.")
		}

		// Step 4: Inject into AI tools
		fmt.Fprintln(cmd.OutOrStdout(), "\nStep 4/5: Inject SAP context into your AI tools?")
		fmt.Fprintln(cmd.OutOrStdout(), "  This writes SAP developer context to your AI tool configuration files.")
		fmt.Fprint(cmd.OutOrStdout(), "  Inject now? [Y/n]: ")
		if answer := strings.ToLower(strings.TrimSpace(readLine())); answer == "" || answer == "y" {
			if err := runInjectGlobal(); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  Warning: inject failed (%v). You can run 'sap-devs inject' manually.\n", err)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "  SAP context injected into your AI tools.")
			}
		}

		// Step 5: Shell profile hook
		fmt.Fprintln(cmd.OutOrStdout(), "\nStep 5/5: Add SAP tip to your terminal startup?")
		fmt.Fprintln(cmd.OutOrStdout(), "  This adds 'sap-devs tip' to your shell profile so you see a tip each time you open a terminal.")
		fmt.Fprint(cmd.OutOrStdout(), "  Add it? [y/N]: ")
		if strings.ToLower(strings.TrimSpace(readLine())) == "y" {
			results, err := shellhook.Add("sap-devs tip", "# SAP developer tips")
			if err != nil && len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  Could not add hook: no shell profile found.\n  Add 'sap-devs tip' to your shell profile manually.\n")
			} else {
				anyUpdated := false
				for _, r := range results {
					if r.Updated {
						anyUpdated = true
						fmt.Fprintf(cmd.OutOrStdout(), "  ✓ Added to %s\n", r.Path)
					}
				}
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  Warning: some profiles could not be updated (%v).\n", err)
				}
				if anyUpdated {
					fmt.Fprintln(cmd.OutOrStdout(), "  Restart your terminal to see your first tip.")
				} else if err == nil {
					fmt.Fprintln(cmd.OutOrStdout(), "  Hook already present in your shell profile(s).")
				}
			}
		}

		fmt.Fprintln(cmd.OutOrStdout(), "\nSetup complete! Run 'sap-devs --help' to explore all commands.")
		fmt.Fprintln(cmd.OutOrStdout(), "Run 'sap-devs inject' to re-inject after syncing new content.")
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

func runInjectGlobal() error {
	prevProject, prevDryRun, prevTool := injectProject, injectDryRun, injectTool
	injectProject = false
	injectDryRun = false
	injectTool = ""
	defer func() { injectProject, injectDryRun, injectTool = prevProject, prevDryRun, prevTool }()
	return injectCmd.RunE(injectCmd, nil)
}

func init() {
	rootCmd.AddCommand(initCmd)
}
