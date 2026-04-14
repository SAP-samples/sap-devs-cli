package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
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
			packs, err = loader.LoadPacks(nil)
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
				return fmt.Errorf("no profile set — run 'sap-devs profile set <name>' first")
			}
			activeProfile, err2 := loader.FindProfile(profileCfg.ID)
			if err2 != nil {
				return err2
			}
			if activeProfile == nil {
				return fmt.Errorf("profile %q not found — run 'sap-devs sync' to refresh content", profileCfg.ID)
			}
			packs, err = loader.LoadPacks(activeProfile)
			if err != nil {
				return err
			}
		}
		servers := content.FlattenMCPServers(packs)
		if len(servers) == 0 {
			fmt.Println("No MCP servers found for your current profile.")
			return nil
		}
		printMCPTable(servers)
		return nil
	},
}

func printMCPTable(servers []content.MCPServer) {
	fmt.Printf("%-24s %-12s %-28s %s\n", "ID", "PACK", "HOSTS", "NAME")
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
		packs, err := loader.LoadPacks(nil)
		if err != nil {
			return err
		}
		servers := content.FlattenMCPServers(packs)
		if len(mcpAdapters) == 0 && len(servers) == 0 {
			fmt.Println("No MCP adapters or servers found.")
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

		fmt.Printf("%-20s %-14s %s\n", "SERVER", "HOST", "STATUS")
		fmt.Println(strings.Repeat("-", 50))
		for _, s := range servers {
			for _, hostID := range s.Hosts {
				m, ok := registered[hostID]
				status := "not installed"
				if ok {
					if _, found := m[s.ID]; found {
						status = "installed"
					}
				}
				fmt.Printf("%-20s %-14s %s\n", s.ID, hostID, status)
			}
		}
		return nil
	},
}

// --- shared helpers ---

// mcpWireAdapters returns adapters of type "mcp-wire" with a non-nil MCPConfig.
// If hostSet is non-nil, only adapters whose ID is in hostSet are returned.
func mcpWireAdapters(adapters []adapter.Adapter, hostSet map[string]bool) []adapter.Adapter {
	var out []adapter.Adapter
	for _, a := range adapters {
		if a.Type != "mcp-wire" || a.MCPConfig == nil {
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
	mcpCmd.AddCommand(mcpListCmd, mcpStatusCmd)
	rootCmd.AddCommand(mcpCmd)
}
