package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/project"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

// profileActive is the sentinel value for --profile that means "use the configured profile".
const profileActive = "@active"

var doctorProfile string
var doctorFix bool
var doctorToolsOnly bool
var doctorProjectOnly bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check local tool versions against pack requirements",
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		var packs []*content.Pack
		switch doctorProfile {
		case "":
			packs, err = loader.LoadPacks(nil, i18n.ActiveLang)
			if err != nil {
				return err
			}
		case profileActive:
			paths, err := xdg.New()
			if err != nil {
				return err
			}
			profileCfg, err := config.LoadProfile(paths.ConfigDir)
			if err != nil {
				return err
			}
			if profileCfg.ID == "" {
				return fmt.Errorf("no profile set — run 'sap-devs profile set <name>' first")
			}
			active, err := loader.FindProfile(profileCfg.ID)
			if err != nil {
				return err
			}
			if active == nil {
				return fmt.Errorf("profile %q not found — run 'sap-devs sync' to refresh content", profileCfg.ID)
			}
			packs, err = loader.LoadPacks(active, i18n.ActiveLang)
			if err != nil {
				return err
			}
		default:
			p, err := loader.FindProfile(doctorProfile)
			if err != nil {
				return err
			}
			if p == nil {
				return fmt.Errorf("profile %q not found — run 'sap-devs sync' to refresh content", doctorProfile)
			}
			packs, err = loader.LoadPacks(p, i18n.ActiveLang)
			if err != nil {
				return err
			}
		}

		// Collect all tools across packs
		if !doctorProjectOnly {
			var tools []content.ToolDef
			for _, p := range packs {
				tools = append(tools, p.Tools...)
			}

			if len(tools) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "doctor.no_tools"))
				return nil
			}

			results := content.CheckTools(tools, execRunner)
			printDoctorTable(results, i18n.ActiveLang)

			if doctorFix {
				printInstallCommands(results)
			}

			for _, r := range results {
				if r.Status == content.StatusFail || r.Status == content.StatusMissing {
					return fmt.Errorf("one or more tools failed the version check")
				}
			}
		}

		// Project health checks
		if !doctorToolsOnly {
			cwd, _ := os.Getwd()
			pc, err := project.Detect(cwd)
			if err == nil && pc.Type != "" {
				findings := project.Check(pc, cwd, packs)
				fmt.Fprintln(cmd.OutOrStdout())
				printProjectHealth(cmd.OutOrStdout(), pc, findings, i18n.ActiveLang)
				for _, f := range findings {
					if f.Severity == "error" {
						return fmt.Errorf("one or more project health checks failed")
					}
				}
			}
		}
		return nil
	},
}

// execRunner runs a command string using exec.Command and returns combined output.
func execRunner(command string) (string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}
	out, err := exec.Command(parts[0], parts[1:]...).CombinedOutput()
	return string(out), err
}

// printDoctorTable prints an aligned table of tool check results.
func printDoctorTable(results []content.ToolResult, lang string) {
	fmt.Printf("%-20s %-12s %-12s %s\n",
		i18n.T(lang, "doctor.col_tool"),
		i18n.T(lang, "doctor.col_required"),
		i18n.T(lang, "doctor.col_found"),
		i18n.T(lang, "doctor.col_status"))
	fmt.Println(strings.Repeat("-", 62))
	for _, r := range results {
		found := r.Found
		if found == "" {
			found = "-"
		}
		fmt.Printf("%-20s %-12s %-12s %s\n", r.Tool.ID, r.Tool.Required, found, statusLabel(r.Status, lang))
	}
}

func statusLabel(s content.CheckStatus, lang string) string {
	switch s {
	case content.StatusOK:
		return i18n.T(lang, "doctor.status_ok")
	case content.StatusUnknown:
		return i18n.T(lang, "doctor.status_ok_unverified")
	case content.StatusFail:
		return i18n.T(lang, "doctor.status_fail")
	case content.StatusMissing:
		return i18n.T(lang, "doctor.status_missing")
	}
	return string(s)
}

// printInstallCommands prints install hints for failed/missing tools.
func printInstallCommands(results []content.ToolResult) {
	var toFix []content.ToolResult
	for _, r := range results {
		if r.Status == content.StatusFail || r.Status == content.StatusMissing {
			toFix = append(toFix, r)
		}
	}
	if len(toFix) == 0 {
		return
	}
	fmt.Println(i18n.T(i18n.ActiveLang, "doctor.install_header"))
	for _, r := range toFix {
		cmd := installCommand(r.Tool, i18n.ActiveLang)
		fmt.Printf("  %-20s %s\n", r.Tool.ID, cmd)
	}
}

// installCommand returns the best install command for the current OS,
// falling back to "all", then to the docs URL.
func installCommand(tool content.ToolDef, lang string) string {
	osKey := map[string]string{
		"windows": "windows",
		"darwin":  "macos",
		"linux":   "linux",
	}[runtime.GOOS]

	if cmd, ok := tool.Install[osKey]; ok && cmd != "" {
		return cmd
	}
	if cmd, ok := tool.Install["all"]; ok && cmd != "" {
		return cmd
	}
	if tool.Docs != "" {
		return i18n.T(lang, "doctor.install_see") + tool.Docs
	}
	return i18n.T(lang, "doctor.install_none")
}

func init() {
	doctorCmd.Flags().StringVar(&doctorProfile, "profile", "", `profile to check ("@active" for configured profile, or a profile ID)`)
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "print install commands for failed or missing tools")
	doctorCmd.Flags().BoolVar(&doctorToolsOnly, "tools-only", false, "check tool versions only (skip project health)")
	doctorCmd.Flags().BoolVar(&doctorProjectOnly, "project-only", false, "check project health only (skip tool versions)")
	rootCmd.AddCommand(doctorCmd)
}

func printProjectHealth(w io.Writer, pc *project.ProjectContext, findings []project.Finding, lang string) {
	header := fmt.Sprintf("%s (%s)", i18n.T(lang, "doctor.project.header"), pc.Type)
	if pc.Deployment != "" {
		deployLabel := pc.Deployment
		if pc.Deployment == "mta-cf" {
			deployLabel = "MTA to Cloud Foundry"
		} else if pc.Deployment == "helm-kyma" {
			deployLabel = "Helm to Kyma"
		}
		header += " — " + deployLabel
	}
	fmt.Fprintln(w, header)
	fmt.Fprintln(w, strings.Repeat("─", 55))

	if len(findings) == 0 {
		fmt.Fprintln(w, i18n.T(lang, "doctor.project.no_findings"))
		return
	}

	for _, f := range findings {
		var icon, sevLabel string
		switch f.Severity {
		case "error":
			icon = "✗"
			sevLabel = i18n.T(lang, "doctor.project.severity_error")
		case "warning":
			icon = "⚠"
			sevLabel = i18n.T(lang, "doctor.project.severity_warning")
		default:
			icon = "ℹ"
			sevLabel = i18n.T(lang, "doctor.project.severity_info")
		}
		fmt.Fprintf(w, "%s %-8s %s\n", icon, sevLabel, f.Message)
		if f.Fix != "" {
			fmt.Fprintf(w, "           %s %s\n", i18n.T(lang, "doctor.project.fix_prefix"), f.Fix)
		}
	}
}
