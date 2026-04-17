package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Manage AI tool lifecycle hooks from pack definitions",
}

// --- hook list ---

var hookListAll bool

var hookListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available hooks",
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
				return fmt.Errorf("no active profile — run `sap-devs profile set`")
			}
			activeProfile, err2 := loader.FindProfile(profileCfg.ID)
			if err2 != nil {
				return err2
			}
			if activeProfile == nil {
				return fmt.Errorf("profile %q not found", profileCfg.ID)
			}
			packs, err = loader.LoadPacks(activeProfile, i18n.ActiveLang)
			if err != nil {
				return err
			}
		}
		hooks := content.FlattenHooks(packs)
		if len(hooks) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No hooks found.")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-16s %-32s %s\n", "ID", "PACK", "EVENT", "COMMAND", "TOOLS")
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 95))
		for _, h := range hooks {
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-16s %-32s %s\n",
				h.ID, h.PackID, h.Event, h.Command, strings.Join(h.Tools, ", "))
		}
		return nil
	},
}

// --- hook status ---

var hookStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show which hooks are installed in your AI tool configs",
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
			fmt.Fprintln(cmd.OutOrStdout(), "No hooks found.")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-14s %s\n", "ID", "PACK", "ADAPTER", "STATUS")
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
				status := "✗ not installed"
				if err == nil && installed {
					status = "✓ installed"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-14s %s\n", h.ID, h.PackID, toolID, status)
			}
		}
		return nil
	},
}

// --- hook install ---

var hookInstallDryRun bool

var hookInstallCmd = &cobra.Command{
	Use:   "install [id]",
	Short: "Wire a hook into your AI tool configs",
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
		return fmt.Errorf("hook %q not found", id)
	}
	toolSet := make(map[string]bool)
	for _, t := range h.Tools {
		toolSet[t] = true
	}
	detected := detectHookAdapters(allAdapters, toolSet)
	if len(detected) == 0 {
		return fmt.Errorf("no detected AI tools support hook %q (tools: %s)", id, strings.Join(h.Tools, ", "))
	}
	if len(detected) > 1 {
		fmt.Printf("Detected AI tools for hook %q:\n", id)
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
			return fmt.Errorf("install hook to %s: %w", a.Name, err)
		}
		if !hookInstallDryRun {
			fmt.Printf("Registered hook %q in %s\n", h.ID, path)
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
		return fmt.Errorf("no active profile — run `sap-devs profile set`")
	}
	activeProfile, err := loader.FindProfile(profileCfg.ID)
	if err != nil {
		return err
	}
	if activeProfile == nil {
		return fmt.Errorf("profile %q not found", profileCfg.ID)
	}
	packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
	if err != nil {
		return err
	}
	hooks := content.FlattenHooks(packs)
	if len(hooks) == 0 {
		return fmt.Errorf("no hooks to install for active profile")
	}
	toolSet := make(map[string]bool)
	for _, h := range hooks {
		for _, t := range h.Tools {
			toolSet[t] = true
		}
	}
	detected := detectHookAdapters(allAdapters, toolSet)
	if len(detected) == 0 {
		return fmt.Errorf("no detected AI tools support these hooks")
	}
	fmt.Println("Detected AI tools:")
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
				return fmt.Errorf("install hook %s to %s: %w", h.ID, a.Name, err)
			}
			if !hookInstallDryRun {
				fmt.Printf("Registered hook %q in %s\n", h.ID, path)
				installed++
			}
		}
	}
	if !hookInstallDryRun {
		fmt.Printf("Installed %d hook(s) into %d tool(s).\n", installed, len(chosen))
	}
	return nil
}

// --- hook uninstall ---

var hookUninstallCmd = &cobra.Command{
	Use:   "uninstall [id]",
	Short: "Remove a hook from your AI tool configs",
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
			return fmt.Errorf("hook %q not found", args[0])
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
				return fmt.Errorf("uninstall hook from %s: %w", a.Name, err)
			}
			fmt.Printf("Removed hook %q from %s\n", h.ID, path)
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
	fmt.Print("Install into (comma-separated numbers or 'all'): ")
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
			return nil, fmt.Errorf("invalid selection: %q", part)
		}
		chosen = append(chosen, adapters[n-1])
	}
	if len(chosen) == 0 {
		return nil, fmt.Errorf("no adapters selected")
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

func init() {
	hookListCmd.Flags().BoolVar(&hookListAll, "all", false, "list hooks from all packs (default: active profile only)")
	hookInstallCmd.Flags().BoolVar(&hookInstallDryRun, "dry-run", false, "preview without writing config files")
	hookCmd.AddCommand(hookListCmd, hookInstallCmd, hookUninstallCmd, hookStatusCmd)
	rootCmd.AddCommand(hookCmd)
}
