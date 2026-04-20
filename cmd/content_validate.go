package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/editor"
	"github.com/SAP-samples/sap-devs-cli/internal/schema"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var contentValidatePackFlag string
var contentValidateLayerFlag string
var contentValidateJSONFlag bool

var contentValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate content YAML files against their schemas",
	Long:  "Validate all known content YAML files across every content layer.",
	RunE:  runContentValidate,
}

func init() {
	contentValidateCmd.Flags().StringVar(&contentValidatePackFlag, "pack", "", "Filter to a specific pack ID")
	contentValidateCmd.Flags().StringVar(&contentValidateLayerFlag, "layer", "", "Filter to a specific layer (official, company, user, project)")
	contentValidateCmd.Flags().BoolVar(&contentValidateJSONFlag, "json", false, "Output results as JSON")
	contentCmd.AddCommand(contentValidateCmd)
}

// validateResult holds validation output for one file (used for JSON output).
type validateResult struct {
	Pack   string                   `json:"pack"`
	File   string                   `json:"file"`
	Layer  string                   `json:"layer"`
	Valid  bool                     `json:"valid"`
	Errors []schema.ValidationError `json:"errors,omitempty"`
}

func runContentValidate(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	schemasDir, err := findSchemasDir(cwd)
	if err != nil {
		return fmt.Errorf("cannot locate schemas directory: %w", err)
	}

	layers := editor.AllLayers(cwd)
	knownFiles := schema.KnownFiles()

	var results []validateResult
	anyErrors := false

	for _, li := range layers {
		if contentValidateLayerFlag != "" && li.Layer.String() != contentValidateLayerFlag {
			continue
		}

		entries, err := os.ReadDir(li.Dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			packID := entry.Name()
			if contentValidatePackFlag != "" && packID != contentValidatePackFlag {
				continue
			}

			for _, filename := range knownFiles {
				filePath := filepath.Join(li.Dir, packID, filename)
				if _, err := os.Stat(filePath); err != nil {
					continue
				}

				errs, err := editor.ValidateFile(filePath, schemasDir)
				if err != nil {
					// File exists but couldn't be validated (e.g. schema not found)
					results = append(results, validateResult{
						Pack:  packID,
						File:  filename,
						Layer: li.Layer.String(),
						Valid: false,
						Errors: []schema.ValidationError{{
							Path:     "",
							Field:    "",
							Message:  err.Error(),
							Severity: schema.SeverityError,
						}},
					})
					anyErrors = true
					continue
				}

				hasErr := false
				for _, e := range errs {
					if e.Severity == schema.SeverityError {
						hasErr = true
						anyErrors = true
						break
					}
				}

				results = append(results, validateResult{
					Pack:   packID,
					File:   filename,
					Layer:  li.Layer.String(),
					Valid:  !hasErr,
					Errors: errs,
				})
			}
		}
	}

	if contentValidateJSONFlag {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
	}

	// Human-readable output
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	for _, r := range results {
		errorCount := 0
		warnCount := 0
		for _, e := range r.Errors {
			if e.Severity == schema.SeverityError {
				errorCount++
			} else {
				warnCount++
			}
		}

		status := "✓"
		if !r.Valid {
			status = "✗"
		}

		line := fmt.Sprintf("%s  %s/%s  [%s]", status, r.Pack, r.File, r.Layer)
		if errorCount > 0 {
			line += fmt.Sprintf("  %d error(s)", errorCount)
		}
		if warnCount > 0 {
			line += fmt.Sprintf("  %d warning(s)", warnCount)
		}
		fmt.Fprintln(w, line)

		// Print details for errors
		for _, e := range r.Errors {
			if e.Severity == schema.SeverityError {
				fmt.Fprintf(w, "\t  ERROR  %s\n", e)
			}
		}
		// Print warnings separately
		for _, e := range r.Errors {
			if e.Severity == schema.SeverityWarning {
				fmt.Fprintf(w, "\t  WARN   %s\n", e)
			}
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}

	if anyErrors {
		return fmt.Errorf("validation failed: found errors in content files")
	}
	return nil
}

// findSchemasDir returns the path to the schemas directory.
// It checks cwd/content/schemas/ first (developer checkout), then falls back
// to the official cache.
func findSchemasDir(cwd string) (string, error) {
	local := filepath.Join(cwd, "content", "schemas")
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}

	paths, err := xdg.New()
	if err != nil {
		return "", err
	}
	cached := filepath.Join(paths.CacheDir, "official", "content", "schemas")
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}

	return "", fmt.Errorf("no schemas directory found at %s or %s", local, cached)
}
