package editor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"charm.land/huh/v2"
	"gopkg.in/yaml.v3"

	"github.com/SAP-samples/sap-devs-cli/internal/schema"
	"github.com/SAP-samples/sap-devs-cli/internal/theme"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var rePackID = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func validPackID(id string) bool {
	return rePackID.MatchString(id)
}

// WizardState holds all answers collected during the creation wizard.
type WizardState struct {
	Layer    Layer
	PackDir  string
	Metadata map[string]any
	// SelectedFiles lists content filenames chosen by the user (e.g. "resources.yaml", "tips.md").
	SelectedFiles []string
	// Entries maps a YAML filename to the single initial entry data collected via BuildForm.
	Entries map[string]map[string]any
}

const contextTemplate = `### Overview

<!-- TODO: Describe what this pack covers -->

### Key Concepts

<!-- TODO: List the essential concepts -->

### Best Practices

<!-- TODO: Add best practices -->
`

const tipsTemplate = `## Tip title here

Tip content here.
`

const constraintsTemplate = `1. First constraint here.
`

// WriteFiles creates the pack directory and writes all files.
func (s *WizardState) WriteFiles() error {
	if err := os.MkdirAll(s.PackDir, 0755); err != nil {
		return fmt.Errorf("create pack directory: %w", err)
	}

	// 1. pack.yaml via SaveObject
	packPath := filepath.Join(s.PackDir, "pack.yaml")
	if err := SaveObject(packPath, s.Metadata); err != nil {
		return fmt.Errorf("write pack.yaml: %w", err)
	}

	// 2. context.md
	contextPath := filepath.Join(s.PackDir, "context.md")
	if err := os.WriteFile(contextPath, []byte(contextTemplate), 0644); err != nil {
		return fmt.Errorf("write context.md: %w", err)
	}

	// 3. Selected content files
	for _, filename := range s.SelectedFiles {
		filePath := filepath.Join(s.PackDir, filename)

		switch filename {
		case "tips.md":
			if err := os.WriteFile(filePath, []byte(tipsTemplate), 0644); err != nil {
				return fmt.Errorf("write %s: %w", filename, err)
			}
		case "constraints.md":
			if err := os.WriteFile(filePath, []byte(constraintsTemplate), 0644); err != nil {
				return fmt.Errorf("write %s: %w", filename, err)
			}
		default:
			// YAML array file: marshal entry or empty array
			var items []map[string]any
			if entry, ok := s.Entries[filename]; ok {
				items = append(items, entry)
			}
			data, err := yaml.Marshal(items)
			if err != nil {
				return fmt.Errorf("marshal %s: %w", filename, err)
			}
			if err := os.WriteFile(filePath, data, 0644); err != nil {
				return fmt.Errorf("write %s: %w", filename, err)
			}
		}
	}

	return nil
}

// Summary returns a human-readable description of what will be created.
func (s *WizardState) Summary() string {
	packID, _ := s.Metadata["id"].(string)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Creating pack %q in %s layer:\n\n", packID, s.Layer))
	sb.WriteString(fmt.Sprintf("  %s/\n", s.PackDir))
	sb.WriteString("    pack.yaml\n")
	sb.WriteString("    context.md\n")

	for _, filename := range s.SelectedFiles {
		if entry, ok := s.Entries[filename]; ok && len(entry) > 0 {
			sb.WriteString(fmt.Sprintf("    %s (1 entry)\n", filename))
		} else {
			sb.WriteString(fmt.Sprintf("    %s\n", filename))
		}
	}

	return sb.String()
}

func checkPackConflict(packDir string) error {
	if _, err := os.Stat(packDir); err == nil {
		return fmt.Errorf("pack directory already exists: %s\nUse 'sap-devs content edit' to modify existing packs", packDir)
	}
	return nil
}

func resolvePackDir(layer Layer, cwd, packID string) (string, error) {
	switch layer {
	case LayerOfficial, LayerCompany:
		return filepath.Join(cwd, "content", "packs", packID), nil
	case LayerProject:
		return filepath.Join(cwd, ".sap-devs", "packs", packID), nil
	case LayerUser:
		paths, err := xdg.New()
		if err != nil {
			return "", fmt.Errorf("cannot resolve user data directory: %w", err)
		}
		return filepath.Join(paths.DataDir, "packs", packID), nil
	}
	return "", fmt.Errorf("unknown layer: %v", layer)
}

func availableLayers(cwd string) []Layer {
	var layers []Layer
	if _, err := os.Stat(filepath.Join(cwd, "content", "packs")); err == nil {
		if isOfficialRepo(cwd) {
			layers = append(layers, LayerOfficial)
		}
		if isCompanyRepo(cwd) {
			layers = append(layers, LayerCompany)
		}
	}
	layers = append(layers, LayerUser, LayerProject)
	return layers
}

func runLayerForm(cwd string) (Layer, error) {
	detected, _ := detectLayer(cwd)
	available := availableLayers(cwd)

	layerStr := detected.String()
	opts := make([]huh.Option[string], 0, len(available))
	for _, l := range available {
		opts = append(opts, huh.NewOption(l.String(), l.String()))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Content layer").
				Description("Where should the new pack be created?").
				Options(opts...).
				Value(&layerStr),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))

	if err := form.Run(); err != nil {
		return 0, err
	}

	switch layerStr {
	case "official":
		return LayerOfficial, nil
	case "company":
		return LayerCompany, nil
	case "user":
		return LayerUser, nil
	case "project":
		return LayerProject, nil
	}
	return LayerUser, nil
}

func buildMetadataMap(id, name, description, tagsRaw, weightRaw string, additive bool, additivePosition string) map[string]any {
	tags := splitTags(tagsRaw)
	anyTags := make([]any, len(tags))
	for i, t := range tags {
		anyTags[i] = t
	}

	weight := 50
	if weightRaw != "" {
		if n, err := strconv.Atoi(weightRaw); err == nil {
			weight = n
		}
	}

	m := map[string]any{
		"id":          id,
		"name":        name,
		"description": description,
		"tags":        anyTags,
		"weight":      weight,
	}

	if additive {
		m["additive"] = true
		if additivePosition != "" {
			m["additive_position"] = additivePosition
		} else {
			m["additive_position"] = "after"
		}
	}

	return m
}

type metadataFormResult struct {
	ID          string
	Name        string
	Description string
	TagsRaw     string
	WeightRaw   string
	Additive    bool
}

func runMetadataForm() (*metadataFormResult, error) {
	r := &metadataFormResult{WeightRaw: "50"}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Pack ID *").
				Placeholder("my-pack").
				Value(&r.ID).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("required")
					}
					if !validPackID(s) {
						return fmt.Errorf("must match ^[a-z][a-z0-9-]*$")
					}
					return nil
				}),
			huh.NewInput().
				Title("Name *").
				Placeholder("My Content Pack").
				Value(&r.Name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Description *").
				Placeholder("A brief description of this pack").
				Value(&r.Description).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Tags *").
				Placeholder("tag1, tag2, tag3").
				Value(&r.TagsRaw).
				Validate(func(s string) error {
					parts := splitTags(s)
					if len(parts) == 0 {
						return fmt.Errorf("at least one tag required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Weight").
				Placeholder("50").
				Value(&r.WeightRaw).
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					if _, err := strconv.Atoi(s); err != nil {
						return fmt.Errorf("must be an integer")
					}
					return nil
				}),
			huh.NewConfirm().
				Title("Additive").
				Description("Augment same-ID pack from a lower layer instead of replacing it?").
				Value(&r.Additive),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))

	if err := form.Run(); err != nil {
		return nil, err
	}
	return r, nil
}

func runAdditivePositionForm() (string, error) {
	position := "after"
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Additive position").
				Description("Where should additive content appear relative to the base pack?").
				Options(
					huh.NewOption("after", "after"),
					huh.NewOption("before", "before"),
				).
				Value(&position),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))

	if err := form.Run(); err != nil {
		return "", err
	}
	return position, nil
}

type contentFileOption struct {
	Filename    string
	Description string
}

var defaultContentFiles = []contentFileOption{
	{"resources.yaml", "Curated links and documentation"},
	{"tools.yaml", "Required/recommended developer tools"},
	{"mcp.yaml", "MCP server definitions"},
	{"samples.yaml", "Canonical code sample references"},
	{"known_errors.yaml", "Common error patterns with fixes"},
	{"tips.md", "Developer tips (H2-delimited)"},
	{"constraints.md", "Behavioral rules for AI agents"},
}

func runContentFileForm() ([]string, error) {
	var selected []string

	opts := make([]huh.Option[string], 0, len(defaultContentFiles))
	for _, f := range defaultContentFiles {
		opts = append(opts, huh.NewOption(
			fmt.Sprintf("%s — %s", f.Filename, f.Description),
			f.Filename,
		))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Content files to scaffold").
				Description("Select files to include in the pack (Space to toggle, Enter to confirm)").
				Options(opts...).
				Value(&selected),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))

	if err := form.Run(); err != nil {
		return nil, err
	}
	return selected, nil
}

func isMarkdownFile(filename string) bool {
	return filename == "tips.md" || filename == "constraints.md"
}

func collectInitialEntries(schemasDir string, selectedFiles []string) (map[string]map[string]any, error) {
	entries := make(map[string]map[string]any)

	for _, filename := range selectedFiles {
		if isMarkdownFile(filename) {
			continue
		}

		schemaName, ok := schema.SchemaForFile(filename)
		if !ok {
			fmt.Fprintf(os.Stderr, "Warning: no schema found for %s, skipping initial entry\n", filename)
			continue
		}

		s, err := schema.Load(schemasDir, schemaName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot load schema for %s: %v\n", filename, err)
			continue
		}

		if s.ItemSpec == nil {
			continue
		}

		fmt.Printf("\n  Initial entry for %s (Esc to skip):\n\n", filename)

		form, bindings := BuildForm(s.ItemSpec, make(map[string]any))
		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				continue
			}
			return nil, err
		}

		entry := bindings.ToMap(s.ItemSpec)
		entries[filename] = entry
	}

	return entries, nil
}

func runConfirmForm(summary string) (bool, error) {
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Proceed?").
				Description(summary).
				Affirmative("Create").
				Negative("Cancel").
				Value(&confirmed),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))

	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}

// RunCreateWizard runs the full content pack creation wizard.
func RunCreateWizard(cwd, schemasDir string) error {
	// Step 1: Layer selection
	layer, err := runLayerForm(cwd)
	if err != nil {
		return err
	}

	// Step 2: Pack metadata
	meta, err := runMetadataForm()
	if err != nil {
		return err
	}

	// Resolve pack directory and check for conflicts
	packDir, err := resolvePackDir(layer, cwd, meta.ID)
	if err != nil {
		return err
	}
	if err := checkPackConflict(packDir); err != nil {
		return err
	}

	// Build additive_position if needed
	var additivePosition string
	if meta.Additive {
		pos, err := runAdditivePositionForm()
		if err != nil {
			return err
		}
		additivePosition = pos
	}

	metadata := buildMetadataMap(
		meta.ID, meta.Name, meta.Description,
		meta.TagsRaw, meta.WeightRaw,
		meta.Additive, additivePosition,
	)

	// Step 4: Content file selection
	selectedFiles, err := runContentFileForm()
	if err != nil {
		return err
	}

	// Step 5: Initial entries for selected YAML files
	entries, err := collectInitialEntries(schemasDir, selectedFiles)
	if err != nil {
		return err
	}

	// Assemble state
	state := &WizardState{
		Layer:         layer,
		PackDir:       packDir,
		Metadata:      metadata,
		SelectedFiles: selectedFiles,
		Entries:       entries,
	}

	// Step 6: Summary and confirmation
	confirmed, err := runConfirmForm(state.Summary())
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("Aborted — no files written.")
		return nil
	}

	// Write all files
	if err := state.WriteFiles(); err != nil {
		return err
	}

	packID, _ := metadata["id"].(string)
	fmt.Printf("\nPack %q created at %s\n", packID, packDir)
	fmt.Printf("Edit with: sap-devs content edit %s/pack.yaml\n", packID)
	return nil
}
