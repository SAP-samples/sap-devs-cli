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

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage SAP MCP servers",
}

// --- mcp list ---

var mcpListAll bool

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available SAP MCP servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		var packs []*content.Pack
		if mcpListAll {
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
				return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "mcp.list.no_profile"))
			}
			activeProfile, err2 := loader.FindProfile(profileCfg.ID)
			if err2 != nil {
				return err2
			}
			if activeProfile == nil {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "mcp.list.profile_not_found", map[string]any{"ID": profileCfg.ID}))
			}
			packs, err = loader.LoadPacks(activeProfile, i18n.ActiveLang)
			if err != nil {
				return err
			}
		}
		servers := content.FlattenMCPServers(packs)
		if len(servers) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "mcp.list.no_servers"))
			return nil
		}
		printMCPTable(servers)
		return nil
	},
}

func printMCPTable(servers []content.MCPServer) {
	fmt.Printf("%-24s %-12s %-28s %s\n",
		i18n.T(i18n.ActiveLang, "mcp.list.col_id"),
		i18n.T(i18n.ActiveLang, "mcp.list.col_pack"),
		i18n.T(i18n.ActiveLang, "mcp.list.col_hosts"),
		i18n.T(i18n.ActiveLang, "mcp.list.col_name"))
	fmt.Println(strings.Repeat("-", 80))
	for _, s := range servers {
		fmt.Printf("%-24s %-12s %-28s %s\n", s.ID, s.PackID, strings.Join(s.Hosts, ", "), s.Name)
	}
}

// --- mcp status ---

var mcpStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show which SAP MCP servers are registered in your AI tool configs",
	RunE: func(cmd *cobra.Command, args []string) error {
		adapters, err := loadAdapters()
		if err != nil {
			return err
		}
		mcpAdapters := mcpWireAdapters(adapters, nil)
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		servers := content.FlattenMCPServers(packs)
		if len(mcpAdapters) == 0 && len(servers) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "mcp.status.no_adapters"))
			return nil
		}

		// Build lookup: adapterID → registered server ID map
		registered := make(map[string]map[string]interface{})
		for _, a := range mcpAdapters {
			path, err := adapter.ExpandHome(a.MCPConfig.Path)
			if err != nil {
				continue
			}
			m, err := adapter.ReadMCPConfig(path, a.MCPConfig.Key)
			if err != nil {
				continue
			}
			registered[a.ID] = m
		}

		fmt.Printf("%-20s %-14s %s\n",
			i18n.T(i18n.ActiveLang, "mcp.status.col_server"),
			i18n.T(i18n.ActiveLang, "mcp.status.col_host"),
			i18n.T(i18n.ActiveLang, "mcp.status.col_status"))
		fmt.Println(strings.Repeat("-", 50))
		for _, s := range servers {
			for _, hostID := range s.Hosts {
				m, ok := registered[hostID]
				status := i18n.T(i18n.ActiveLang, "mcp.status.not_installed")
				if ok {
					if _, found := m[s.ID]; found {
						status = i18n.T(i18n.ActiveLang, "mcp.status.installed")
					}
				}
				fmt.Printf("%-20s %-14s %s\n", s.ID, hostID, status)
			}
		}
		return nil
	},
}

// --- mcp install ---

var mcpInstallAll bool
var mcpInstallDryRun bool

var mcpInstallCmd = &cobra.Command{
	Use:   "install [id]",
	Short: "Install and wire an SAP MCP server into your AI tools",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && !mcpInstallAll {
			return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "mcp.install.specify_server"))
		}
		if len(args) > 0 && mcpInstallAll {
			return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "mcp.install.both_server_all"))
		}

		allAdapters, err := loadAdapters()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		if mcpInstallAll {
			return installAll(loader, allAdapters)
		}
		return installOne(loader, allAdapters, args[0])
	},
}

func installOne(loader *content.ContentLoader, allAdapters []adapter.Adapter, id string) error {
	packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
	if err != nil {
		return err
	}
	server := content.FindMCPServer(packs, id)
	if server == nil {
		return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "mcp.install.server_not_found", map[string]any{"ID": id}))
	}

	hostSet := make(map[string]bool)
	for _, h := range server.Hosts {
		hostSet[h] = true
	}
	detected := detectAdapters(mcpWireAdapters(allAdapters, hostSet))
	if len(detected) == 0 {
		return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "mcp.install.no_hosts", map[string]any{
			"ID":    server.ID,
			"Hosts": strings.Join(server.Hosts, ", "),
		}))
	}

	fmt.Println(i18n.Tf(i18n.ActiveLang, "mcp.install.detected_hosts", map[string]any{"ID": server.ID}))
	for i, a := range detected {
		path, _ := adapter.ExpandHome(a.MCPConfig.Path)
		fmt.Printf("  %d. %s  (%s)\n", i+1, a.Name, path)
	}
	chosen, err := pickAdapters(detected)
	if err != nil {
		return err
	}

	for _, a := range chosen {
		path, err := adapter.ExpandHome(a.MCPConfig.Path)
		if err != nil {
			return err
		}
		if err := adapter.WriteMCPConfig(path, a.MCPConfig.Key, *server, mcpInstallDryRun); err != nil {
			return fmt.Errorf("%s: %w", i18n.Tf(i18n.ActiveLang, "mcp.install.install_err_one", map[string]any{"Name": a.Name}), err)
		}
		if !mcpInstallDryRun {
			fmt.Println(i18n.Tf(i18n.ActiveLang, "mcp.install.registered", map[string]any{"ServerID": server.ID, "Path": path}))
		}
	}
	return nil
}

func installAll(loader *content.ContentLoader, allAdapters []adapter.Adapter) error {
	paths, err := xdg.New()
	if err != nil {
		return err
	}
	profileCfg, err := config.LoadProfile(paths.ConfigDir)
	if err != nil {
		return err
	}
	if profileCfg.ID == "" {
		return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "mcp.list.no_profile"))
	}
	activeProfile, err := loader.FindProfile(profileCfg.ID)
	if err != nil {
		return err
	}
	if activeProfile == nil {
		return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "mcp.list.profile_not_found", map[string]any{"ID": profileCfg.ID}))
	}
	packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
	if err != nil {
		return err
	}
	servers := content.FlattenMCPServers(packs)
	if len(servers) == 0 {
		fmt.Println(i18n.T(i18n.ActiveLang, "mcp.install.no_servers"))
		return nil
	}

	// Collect union of all host IDs across all servers
	hostSet := make(map[string]bool)
	for _, s := range servers {
		for _, h := range s.Hosts {
			hostSet[h] = true
		}
	}
	detected := detectAdapters(mcpWireAdapters(allAdapters, hostSet))
	if len(detected) == 0 {
		var allHosts []string
		for h := range hostSet {
			allHosts = append(allHosts, h)
		}
		return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "mcp.install.no_hosts_all", map[string]any{
			"Hosts": strings.Join(allHosts, ", "),
		}))
	}

	fmt.Println(i18n.T(i18n.ActiveLang, "mcp.install.all_detected_hosts"))
	for i, a := range detected {
		path, _ := adapter.ExpandHome(a.MCPConfig.Path)
		fmt.Printf("  %d. %s  (%s)\n", i+1, a.Name, path)
	}
	chosen, err := pickAdapters(detected)
	if err != nil {
		return err
	}

	serversWritten := make(map[string]bool)
	for _, s := range servers {
		for _, a := range chosen {
			if !containsString(s.Hosts, a.ID) {
				continue
			}
			path, err := adapter.ExpandHome(a.MCPConfig.Path)
			if err != nil {
				return err
			}
			if err := adapter.WriteMCPConfig(path, a.MCPConfig.Key, s, mcpInstallDryRun); err != nil {
				return fmt.Errorf("%s: %w", i18n.Tf(i18n.ActiveLang, "mcp.install.install_err_all", map[string]any{"ID": s.ID, "Name": a.Name}), err)
			}
			if !mcpInstallDryRun {
				fmt.Println(i18n.Tf(i18n.ActiveLang, "mcp.install.registered", map[string]any{"ServerID": s.ID, "Path": path}))
				serversWritten[s.ID] = true
			}
		}
	}
	if !mcpInstallDryRun {
		fmt.Println(i18n.Tf(i18n.ActiveLang, "mcp.install.summary", map[string]any{"Servers": len(serversWritten), "Hosts": len(chosen)}))
	}
	return nil
}

// pickAdapters prints a numbered list and reads a selection from stdin.
// The user may enter comma/space-separated numbers or "all".
func pickAdapters(adapters []adapter.Adapter) ([]adapter.Adapter, error) {
	fmt.Print(i18n.T(i18n.ActiveLang, "mcp.install.prompt"))
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
			return nil, fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "mcp.install.invalid_selection", map[string]any{"Selection": part}))
		}
		chosen = append(chosen, adapters[n-1])
	}
	if len(chosen) == 0 {
		return nil, fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "mcp.install.invalid_selection", map[string]any{"Selection": line}))
	}
	return chosen, nil
}

// --- shared helpers ---

// mcpWireAdapters returns adapters of type "mcp-wire" with a non-nil MCPConfig.
// If hostSet is non-nil, only adapters whose ID is in hostSet are returned.
func mcpWireAdapters(adapters []adapter.Adapter, hostSet map[string]bool) []adapter.Adapter {
	var out []adapter.Adapter
	for _, a := range adapters {
		if a.MCPConfig == nil {
			continue
		}
		if hostSet != nil && !hostSet[a.ID] {
			continue
		}
		out = append(out, a)
	}
	return out
}

// detectAdapters filters adapters to those detected as installed on this machine.
func detectAdapters(adapters []adapter.Adapter) []adapter.Adapter {
	var out []adapter.Adapter
	for _, a := range adapters {
		if adapter.Detect(a) {
			out = append(out, a)
		}
	}
	return out
}

// containsString returns true if slice contains s.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func init() {
	mcpListCmd.Flags().BoolVar(&mcpListAll, "all", false, "list servers from all packs (default: active profile only)")
	mcpInstallCmd.Flags().BoolVar(&mcpInstallAll, "all", false, "install all MCP servers for the active profile")
	mcpInstallCmd.Flags().BoolVar(&mcpInstallDryRun, "dry-run", false, "preview without writing config files")
	mcpCmd.AddCommand(mcpListCmd, mcpInstallCmd, mcpStatusCmd)
	rootCmd.AddCommand(mcpCmd)
}
