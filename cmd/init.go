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
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/shellhook"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "First-time setup wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.welcome"))
		fmt.Fprintln(cmd.OutOrStdout())

		paths, err := xdg.New()
		if err != nil {
			return err
		}

		// Step 1: GitHub authentication (optional)
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step1_header"))
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step1_body"))
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprint(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step1_prompt"))
		raw, termErr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(cmd.OutOrStdout()) // newline after hidden input
		if termErr != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step1_no_tty"))
		} else {
			token := strings.TrimSpace(string(raw))
			if token != "" {
				if storeErr := credentials.Store(paths.ConfigDir, token); storeErr != nil {
					fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "init.step1_token_warn", map[string]any{"Err": storeErr}))
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step1_token_stored"))
				}
			}
		}

		// Step 2: Sync content
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step2_header"))
		if err := runSyncForce(); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "init.step2_warn_failed", map[string]any{"Err": err}))
		}

		// Step 3: Choose profile
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step3_header"))
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
			fmt.Fprint(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step3_prompt"))
			choice := readLine()
			if choice != "" {
				idx := 0
				fmt.Sscanf(choice, "%d", &idx)
				if idx >= 1 && idx <= len(profiles) {
					chosen := profiles[idx-1]
					if err := config.SaveProfile(paths.ConfigDir, &config.Profile{ID: chosen.ID}); err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "init.step3_profile_set", map[string]any{"Name": chosen.Name}))
				}
			}
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step3_no_profiles"))
		}

		// Step 4: Set location (optional)
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step4_location_header"))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step4_location_body"))
		fmt.Fprint(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step4_location_prompt"))
		locInput := strings.TrimSpace(readLine())
		if locInput != "" {
			locationCfg, locErr := config.Load(paths.ConfigDir)
			if locErr == nil {
				if strings.ToLower(locInput) == "detect" {
					if detected, detectErr := detectLocation(cmd.OutOrStdout(), os.Stdin); detectErr == nil && detected != "" {
						locationCfg.Location = detected
						locationCfg.Save(paths.ConfigDir) //nolint:errcheck
					}
				} else {
					locationCfg.Location = locInput // preserve original casing
					locationCfg.Save(paths.ConfigDir) //nolint:errcheck
				}
			}
		}

		// Step 5: Inject into AI tools
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step5_header"))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step5_body"))
		fmt.Fprint(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step5_prompt"))
		if answer := strings.ToLower(strings.TrimSpace(readLine())); answer == "" || answer == "y" {
			if err := runInjectGlobal(); err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "init.step5_warn_failed", map[string]any{"Err": err}))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step5_done"))
			}
		}

		// Step 6: Shell profile hook
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step6_header"))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step6_body"))
		fmt.Fprint(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step6_prompt"))
		if strings.ToLower(strings.TrimSpace(readLine())) == "y" {
			results, err := shellhook.Add("sap-devs tip", "# SAP developer tips")
			if err != nil && len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step6_no_profile"))
			} else {
				anyUpdated := false
				for _, r := range results {
					if r.Updated {
						anyUpdated = true
						fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "init.step6_added", map[string]any{"Path": r.Path}))
					}
				}
				if err != nil {
					fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "init.step6_warn_partial", map[string]any{"Err": err}))
				}
				if anyUpdated {
					fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step6_restart"))
				} else if err == nil {
					fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step6_already_present"))
				}
			}
		}

		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.complete"))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.complete_hint"))
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
