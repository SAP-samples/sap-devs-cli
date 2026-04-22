package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/adapter"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: i18n.T("en", "hook.short"),
}

// --- hook list ---

var hookListAll bool

var hookListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("en", "hook.list.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		var packs []*content.Pack
		if hookListAll {
			packs, err = loader.LoadPacks(nil, i18n.ActiveLang)
			if err != nil {
				return err
			}
		} else {
			paths, err2 := xdg.New()
			if err2 != nil {
				return err2
			}
			profileCfg, err2 := config.LoadProfile(paths.ConfigDir)
			if err2 != nil {
				return err2
			}
			if profileCfg.ID == "" {
				return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "hook.error.no_profile"))
			}
			activeProfile, err2 := loader.FindProfile(profileCfg.ID)
			if err2 != nil {
				return err2
			}
			if activeProfile == nil {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "hook.error.profile_not_found", map[string]any{"ID": profileCfg.ID}))
			}
			packs, err = loader.LoadPacks(activeProfile, i18n.ActiveLang)
			if err != nil {
				return err
			}
		}
		hooks := content.FlattenHooks(packs)
		if len(hooks) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "hook.no_hooks"))
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-16s %-32s %s\n",
			i18n.T(i18n.ActiveLang, "hook.col_id"),
			i18n.T(i18n.ActiveLang, "hook.col_pack"),
			i18n.T(i18n.ActiveLang, "hook.col_event"),
			i18n.T(i18n.ActiveLang, "hook.col_command"),
			i18n.T(i18n.ActiveLang, "hook.col_tools"))
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 95))
		for _, h := range hooks {
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-16s %-32s %s\n",
				h.ID, h.PackID, h.Event, h.Command, strings.Join(h.Tools, ", "))
		}

		skills := content.FlattenSkills(packs)
		if len(skills) > 0 {
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %s\n", "SKILL", "PACK", "TOOLS")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 55))
			for _, s := range skills {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %s\n",
					s.ID, s.PackID, strings.Join(s.Tools, ", "))
			}
		}
		return nil
	},
}

// --- hook status ---

var hookStatusCmd = &cobra.Command{
	Use:   "status",
	Short: i18n.T("en", "hook.status.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		allAdapters, err := loadAdapters()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		hooks := content.FlattenHooks(packs)
		if len(hooks) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "hook.no_hooks"))
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-14s %s\n",
			i18n.T(i18n.ActiveLang, "hook.col_id"),
			i18n.T(i18n.ActiveLang, "hook.col_pack"),
			i18n.T(i18n.ActiveLang, "hook.col_adapter"),
			i18n.T(i18n.ActiveLang, "hook.col_status"))
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 65))

		for _, h := range hooks {
			for _, toolID := range h.Tools {
				a := findAdapterByID(allAdapters, toolID)
				if a == nil || a.HookConfig == nil {
					continue
				}
				path, err := adapter.ExpandHome(a.HookConfig.Path)
				if err != nil {
					continue
				}
				installed, err := adapter.HookConfigInstalled(path, a.HookConfig.Key, h.Command)
				status := i18n.T(i18n.ActiveLang, "hook.status.not_installed")
				if err == nil && installed {
					status = i18n.T(i18n.ActiveLang, "hook.status.installed")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-14s %s\n", h.ID, h.PackID, toolID, status)
			}
		}

		skills := content.FlattenSkills(packs)
		if len(skills) > 0 {
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-14s %s\n", "SKILL", "PACK", "ADAPTER", "STATUS")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 65))
			for _, s := range skills {
				for _, toolID := range s.Tools {
					a := findAdapterByID(allAdapters, toolID)
					if a == nil || a.SkillConfig == nil {
						continue
					}
					basePath, err := adapter.ExpandHome(a.SkillConfig.Path)
					if err != nil {
						continue
					}
					status := i18n.T(i18n.ActiveLang, "hook.status.not_installed")
					if adapter.SkillFileInstalled(basePath, s.ID) {
						status = i18n.T(i18n.ActiveLang, "hook.status.installed")
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-14s %s\n", s.ID, s.PackID, toolID, status)
				}
			}
		}
		return nil
	},
}

// --- hook install ---

var hookInstallDryRun bool

var hookInstallCmd = &cobra.Command{
	Use:   "install [id]",
	Short: i18n.T("en", "hook.install.short"),
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		allAdapters, err := loadAdapters()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		if len(args) == 1 {
			return hookInstallOne(loader, allAdapters, args[0])
		}
		return hookInstallAll(loader, allAdapters)
	},
}

func hookInstallOne(loader *content.ContentLoader, allAdapters []adapter.Adapter, id string) error {
	packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
	if err != nil {
		return err
	}
	h := content.FindHookDef(packs, id)
	if h == nil {
		return installSkillOne(packs, allAdapters, id)
	}
	toolSet := make(map[string]bool)
	for _, t := range h.Tools {
		toolSet[t] = true
	}
	detected := detectHookAdapters(allAdapters, toolSet)
	if len(detected) == 0 {
		return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "hook.error.no_tools", map[string]any{"ID": id, "Tools": strings.Join(h.Tools, ", ")}))
	}
	if len(detected) > 1 {
		fmt.Println(i18n.Tf(i18n.ActiveLang, "hook.detected_header", map[string]any{"ID": id}))
		for i, a := range detected {
			p, _ := adapter.ExpandHome(a.HookConfig.Path)
			fmt.Printf("  %d. %s  (%s)\n", i+1, a.Name, p)
		}
	}
	chosen, err := pickHookAdapters(detected)
	if err != nil {
		return err
	}
	for _, a := range chosen {
		path, err := adapter.ExpandHome(a.HookConfig.Path)
		if err != nil {
			return err
		}
		if err := adapter.WriteHookConfig(path, a.HookConfig.Key, h.Command, hookInstallDryRun); err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "hook.error.install", map[string]any{"Name": a.Name, "Err": err}))
		}
		if !hookInstallDryRun {
			fmt.Println(i18n.Tf(i18n.ActiveLang, "hook.install.registered", map[string]any{"ID": h.ID, "Path": path}))
		}
	}
	return nil
}

func hookInstallAll(loader *content.ContentLoader, allAdapters []adapter.Adapter) error {
	paths, err := xdg.New()
	if err != nil {
		return err
	}
	profileCfg, err := config.LoadProfile(paths.ConfigDir)
	if err != nil {
		return err
	}
	if profileCfg.ID == "" {
		return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "hook.error.no_profile"))
	}
	activeProfile, err := loader.FindProfile(profileCfg.ID)
	if err != nil {
		return err
	}
	if activeProfile == nil {
		return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "hook.error.profile_not_found", map[string]any{"ID": profileCfg.ID}))
	}
	packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
	if err != nil {
		return err
	}
	hooks := content.FlattenHooks(packs)
	if len(hooks) == 0 {
		return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "hook.error.no_hooks_profile"))
	}
	toolSet := make(map[string]bool)
	for _, h := range hooks {
		for _, t := range h.Tools {
			toolSet[t] = true
		}
	}
	detected := detectHookAdapters(allAdapters, toolSet)
	if len(detected) == 0 {
		return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "hook.error.no_tools_profile"))
	}
	fmt.Println(i18n.T(i18n.ActiveLang, "hook.detected_header_all"))
	for i, a := range detected {
		p, _ := adapter.ExpandHome(a.HookConfig.Path)
		fmt.Printf("  %d. %s  (%s)\n", i+1, a.Name, p)
	}
	chosen, err := pickHookAdapters(detected)
	if err != nil {
		return err
	}
	installed := 0
	for _, h := range hooks {
		for _, a := range chosen {
			if !containsString(h.Tools, a.ID) {
				continue
			}
			path, err := adapter.ExpandHome(a.HookConfig.Path)
			if err != nil {
				return err
			}
			if err := adapter.WriteHookConfig(path, a.HookConfig.Key, h.Command, hookInstallDryRun); err != nil {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "hook.error.install_named", map[string]any{"ID": h.ID, "Name": a.Name, "Err": err}))
			}
			if !hookInstallDryRun {
				fmt.Println(i18n.Tf(i18n.ActiveLang, "hook.install.registered", map[string]any{"ID": h.ID, "Path": path}))
				installed++
			}
		}
	}
	if !hookInstallDryRun {
		fmt.Println(i18n.Tf(i18n.ActiveLang, "hook.install.summary", map[string]any{"Hooks": installed, "Tools": len(chosen)}))
	}

	skills := content.FlattenSkills(packs)
	skillAdapters := detectSkillAdapters(allAdapters, nil)
	skillsInstalled := 0
	for _, s := range skills {
		if s.Content == "" {
			continue
		}
		for _, a := range skillAdapters {
			if !containsString(s.Tools, a.ID) {
				continue
			}
			basePath, err := adapter.ExpandHome(a.SkillConfig.Path)
			if err != nil {
				return err
			}
			if err := adapter.WriteSkillFile(basePath, s.ID, s.Content, hookInstallDryRun); err != nil {
				return fmt.Errorf("skill %q: %w", s.ID, err)
			}
			if !hookInstallDryRun {
				fmt.Println(i18n.Tf(i18n.ActiveLang, "skill.install.registered", map[string]any{
					"ID": s.ID, "Path": basePath,
				}))
				skillsInstalled++
			}
		}
	}
	if !hookInstallDryRun && skillsInstalled > 0 {
		fmt.Println(i18n.Tf(i18n.ActiveLang, "skill.install.summary", map[string]any{"Skills": skillsInstalled}))
	}
	return nil
}

// --- hook uninstall ---

var hookUninstallCmd = &cobra.Command{
	Use:   "uninstall [id]",
	Short: i18n.T("en", "hook.uninstall.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		allAdapters, err := loadAdapters()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		h := content.FindHookDef(packs, args[0])
		if h == nil {
			return uninstallSkillOne(packs, allAdapters, args[0])
		}
		for _, toolID := range h.Tools {
			a := findAdapterByID(allAdapters, toolID)
			if a == nil || a.HookConfig == nil {
				continue
			}
			path, err := adapter.ExpandHome(a.HookConfig.Path)
			if err != nil {
				return err
			}
			if err := adapter.RemoveHookConfig(path, a.HookConfig.Key, h.Command, false); err != nil {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "hook.error.uninstall", map[string]any{"Name": a.Name, "Err": err}))
			}
			fmt.Println(i18n.Tf(i18n.ActiveLang, "hook.uninstall.removed", map[string]any{"ID": h.ID, "Path": path}))
		}
		return nil
	},
}

// --- shared helpers ---

// detectHookAdapters returns adapters that have hook_config, whose IDs are in
// toolSet, and are detected as installed on this machine.
func detectHookAdapters(adapters []adapter.Adapter, toolSet map[string]bool) []adapter.Adapter {
	var out []adapter.Adapter
	for _, a := range adapters {
		if a.HookConfig == nil {
			continue
		}
		if toolSet != nil && !toolSet[a.ID] {
			continue
		}
		if adapter.Detect(a) {
			out = append(out, a)
		}
	}
	return out
}

// pickHookAdapters prompts the user to select adapters; single adapter skips the prompt.
func pickHookAdapters(adapters []adapter.Adapter) ([]adapter.Adapter, error) {
	if len(adapters) == 1 {
		return adapters, nil
	}
	fmt.Print(i18n.T(i18n.ActiveLang, "hook.prompt"))
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(line)
	if strings.ToLower(line) == "all" {
		return adapters, nil
	}
	var chosen []adapter.Adapter
	for _, part := range strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == ' ' }) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > len(adapters) {
			return nil, fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "hook.error.invalid_selection", map[string]any{"Value": part}))
		}
		chosen = append(chosen, adapters[n-1])
	}
	if len(chosen) == 0 {
		return nil, fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "hook.error.no_selection"))
	}
	return chosen, nil
}

// findAdapterByID returns the adapter with the given ID, or nil.
func findAdapterByID(adapters []adapter.Adapter, id string) *adapter.Adapter {
	for i := range adapters {
		if adapters[i].ID == id {
			return &adapters[i]
		}
	}
	return nil
}

func detectSkillAdapters(adapters []adapter.Adapter, toolSet map[string]bool) []adapter.Adapter {
	var out []adapter.Adapter
	for _, a := range adapters {
		if a.SkillConfig == nil {
			continue
		}
		if toolSet != nil && !toolSet[a.ID] {
			continue
		}
		if adapter.Detect(a) {
			out = append(out, a)
		}
	}
	return out
}

func installSkillOne(packs []*content.Pack, allAdapters []adapter.Adapter, id string) error {
	s := content.FindSkillDef(packs, id)
	if s == nil {
		return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "hook.error.not_found", map[string]any{"ID": id}))
	}
	if s.Content == "" {
		return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "skill.error.no_content", map[string]any{"ID": id}))
	}
	toolSet := make(map[string]bool)
	for _, t := range s.Tools {
		toolSet[t] = true
	}
	detected := detectSkillAdapters(allAdapters, toolSet)
	if len(detected) == 0 {
		return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "skill.error.no_tools", map[string]any{"ID": id, "Tools": strings.Join(s.Tools, ", ")}))
	}
	if len(detected) > 1 {
		fmt.Println(i18n.Tf(i18n.ActiveLang, "skill.detected_header", map[string]any{"ID": id}))
		for i, a := range detected {
			p, _ := adapter.ExpandHome(a.SkillConfig.Path)
			fmt.Printf("  %d. %s  (%s)\n", i+1, a.Name, p)
		}
	}
	chosen, err := pickHookAdapters(detected)
	if err != nil {
		return err
	}
	for _, a := range chosen {
		basePath, err := adapter.ExpandHome(a.SkillConfig.Path)
		if err != nil {
			return err
		}
		if err := adapter.WriteSkillFile(basePath, s.ID, s.Content, hookInstallDryRun); err != nil {
			return fmt.Errorf("skill %q: %w", s.ID, err)
		}
		if !hookInstallDryRun {
			fmt.Println(i18n.Tf(i18n.ActiveLang, "skill.install.registered", map[string]any{"ID": s.ID, "Path": basePath}))
		}
	}
	return nil
}

func uninstallSkillOne(packs []*content.Pack, allAdapters []adapter.Adapter, id string) error {
	s := content.FindSkillDef(packs, id)
	if s == nil {
		return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "hook.error.not_found", map[string]any{"ID": id}))
	}
	for _, toolID := range s.Tools {
		a := findAdapterByID(allAdapters, toolID)
		if a == nil || a.SkillConfig == nil {
			continue
		}
		basePath, err := adapter.ExpandHome(a.SkillConfig.Path)
		if err != nil {
			return err
		}
		if err := adapter.RemoveSkillFile(basePath, s.ID, false); err != nil {
			return fmt.Errorf("skill %q: %w", s.ID, err)
		}
		fmt.Println(i18n.Tf(i18n.ActiveLang, "skill.uninstall.removed", map[string]any{"ID": s.ID, "Path": basePath}))
	}
	return nil
}

func init() {
	hookListCmd.Flags().BoolVar(&hookListAll, "all", false, "list hooks from all packs (default: active profile only)")
	hookInstallCmd.Flags().BoolVar(&hookInstallDryRun, "dry-run", false, "preview without writing config files")
	hookCmd.AddCommand(hookListCmd, hookInstallCmd, hookUninstallCmd, hookStatusCmd)
	rootCmd.AddCommand(hookCmd)
}
