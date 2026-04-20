package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/editor"
	"github.com/SAP-samples/sap-devs-cli/internal/schema"
	"gopkg.in/yaml.v3"
)

var contentListPackFlag string
var contentListLayerFlag string

var contentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List content YAML files across all layers",
	Long:  "List all known content YAML files across every content layer, with item counts.",
	RunE:  runContentList,
}

func init() {
	contentListCmd.Flags().StringVar(&contentListPackFlag, "pack", "", "Filter to a specific pack ID")
	contentListCmd.Flags().StringVar(&contentListLayerFlag, "layer", "", "Filter to a specific layer (official, company, user, project)")
	contentCmd.AddCommand(contentListCmd)
}

func runContentList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	layers := editor.AllLayers(cwd)
	knownFiles := schema.KnownFiles()

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PACK\tFILE\tLAYER\tITEMS")

	for _, li := range layers {
		if contentListLayerFlag != "" && li.Layer.String() != contentListLayerFlag {
			continue
		}

		// Enumerate packs in this layer's packs directory
		entries, err := os.ReadDir(li.Dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			packID := entry.Name()
			if contentListPackFlag != "" && packID != contentListPackFlag {
				continue
			}

			for _, filename := range knownFiles {
				filePath := filepath.Join(li.Dir, packID, filename)
				if _, err := os.Stat(filePath); err != nil {
					continue
				}

				items := countYAMLItems(filePath)
				var itemStr string
				if items < 0 {
					itemStr = "—"
				} else {
					itemStr = fmt.Sprintf("%d", items)
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", packID, filename, li.Layer.String(), itemStr)
			}
		}
	}

	return w.Flush()
}

// countYAMLItems reads the YAML file and returns the number of top-level array
// items. Returns -1 if the file is not an array (e.g. an object like pack.yaml).
func countYAMLItems(filePath string) int {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return -1
	}

	var arr []any
	if err := yaml.Unmarshal(data, &arr); err != nil {
		return -1
	}
	if arr == nil {
		// Could be an object
		return -1
	}
	return len(arr)
}
