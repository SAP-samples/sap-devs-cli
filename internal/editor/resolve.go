package editor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

// Layer describes which content layer is being edited.
type Layer int

const (
	LayerOfficial Layer = iota
	LayerCompany
	LayerUser
	LayerProject
)

// String returns a human-readable label for the layer.
func (l Layer) String() string {
	switch l {
	case LayerOfficial:
		return "official"
	case LayerCompany:
		return "company"
	case LayerUser:
		return "user"
	case LayerProject:
		return "project"
	}
	return "unknown"
}

// ResolvedFile contains the resolved editing target.
type ResolvedFile struct {
	Layer      Layer
	PackID     string
	Filename   string
	SchemaName string
	FilePath   string // actual file path to edit
	PackDir    string // directory containing the pack
}

const officialRepoURL = "github.tools.sap/developer-relations/sap-devs-cli"

// ResolveEditTarget determines the file path and layer for a content edit
// request. arg can be a direct path (starts with ./, .sap-devs/, or content/),
// a pack/file pair (e.g. "cap/resources.yaml"), or a bare filename
// (e.g. "resources.yaml").
func ResolveEditTarget(cwd, arg string) (*ResolvedFile, error) {
	// Direct path: starts with ./ or .sap-devs/ or content/
	if strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, ".sap-devs/") || strings.HasPrefix(arg, "content/") {
		return resolveDirectPath(cwd, arg)
	}

	// Pack/file format: "cap/resources.yaml"
	if parts := strings.SplitN(arg, "/", 2); len(parts) == 2 {
		packID := parts[0]
		filename := parts[1]
		schemaName, ok := schema.SchemaForFile(filename)
		if !ok {
			return nil, fmt.Errorf("unknown content file: %s", filename)
		}
		return resolveForPack(cwd, packID, filename, schemaName)
	}

	// Bare filename: "resources.yaml"
	schemaName, ok := schema.SchemaForFile(arg)
	if !ok {
		return nil, fmt.Errorf("unknown content file: %s", arg)
	}
	return resolveForFile(cwd, arg, schemaName)
}

func resolveDirectPath(cwd, arg string) (*ResolvedFile, error) {
	fullPath := filepath.Join(cwd, arg)
	filename := filepath.Base(fullPath)
	schemaName, ok := schema.SchemaForFile(filename)
	if !ok {
		return nil, fmt.Errorf("unknown content file: %s", filename)
	}

	packDir := filepath.Dir(fullPath)
	packID := filepath.Base(packDir)

	layer := LayerProject
	if strings.Contains(filepath.ToSlash(fullPath), "content/packs/") {
		layer = LayerOfficial
	}

	return &ResolvedFile{
		Layer:      layer,
		PackID:     packID,
		Filename:   filename,
		SchemaName: schemaName,
		FilePath:   fullPath,
		PackDir:    packDir,
	}, nil
}

func resolveForPack(cwd, packID, filename, schemaName string) (*ResolvedFile, error) {
	layer, baseDir := detectLayer(cwd)

	var packDir string
	switch layer {
	case LayerOfficial, LayerCompany:
		packDir = filepath.Join(baseDir, "content", "packs", packID)
	case LayerProject:
		packDir = filepath.Join(baseDir, ".sap-devs", "packs", packID)
	case LayerUser:
		packDir = filepath.Join(baseDir, "packs", packID)
	}

	return &ResolvedFile{
		Layer:      layer,
		PackID:     packID,
		Filename:   filename,
		SchemaName: schemaName,
		FilePath:   filepath.Join(packDir, filename),
		PackDir:    packDir,
	}, nil
}

func resolveForFile(cwd, filename, schemaName string) (*ResolvedFile, error) {
	layer, baseDir := detectLayer(cwd)

	var packsDir string
	switch layer {
	case LayerOfficial, LayerCompany:
		packsDir = filepath.Join(baseDir, "content", "packs")
	case LayerProject:
		packsDir = filepath.Join(baseDir, ".sap-devs", "packs")
	case LayerUser:
		packsDir = filepath.Join(baseDir, "packs")
	}

	// Scan for packs containing this file
	entries, err := os.ReadDir(packsDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read packs directory %s: %w", packsDir, err)
	}

	var matches []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(packsDir, e.Name(), filename)
		if _, statErr := os.Stat(candidate); statErr == nil {
			matches = append(matches, e.Name())
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no pack contains %s in layer %s", filename, layer)
	}

	// Use first match; caller can use AmbiguousPacks to prompt disambiguation.
	packID := matches[0]
	packDir := filepath.Join(packsDir, packID)

	return &ResolvedFile{
		Layer:      layer,
		PackID:     packID,
		Filename:   filename,
		SchemaName: schemaName,
		FilePath:   filepath.Join(packDir, filename),
		PackDir:    packDir,
	}, nil
}

// AmbiguousPacks returns all pack IDs that contain a given filename in the
// detected layer. Useful for prompting the user to disambiguate.
func AmbiguousPacks(cwd, filename string) []string {
	layer, baseDir := detectLayer(cwd)

	var packsDir string
	switch layer {
	case LayerOfficial, LayerCompany:
		packsDir = filepath.Join(baseDir, "content", "packs")
	case LayerProject:
		packsDir = filepath.Join(baseDir, ".sap-devs", "packs")
	case LayerUser:
		packsDir = filepath.Join(baseDir, "packs")
	}

	entries, err := os.ReadDir(packsDir)
	if err != nil {
		return nil
	}

	var matches []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(packsDir, e.Name(), filename)); statErr == nil {
			matches = append(matches, e.Name())
		}
	}
	return matches
}

// detectLayer inspects the working directory to determine the content layer and
// its base directory.
func detectLayer(cwd string) (Layer, string) {
	// Check for official or company repo checkout
	if _, err := os.Stat(filepath.Join(cwd, "content", "packs")); err == nil {
		if isOfficialRepo(cwd) {
			return LayerOfficial, cwd
		}
		if isCompanyRepo(cwd) {
			return LayerCompany, cwd
		}
	}

	// Check for project layer
	if _, err := os.Stat(filepath.Join(cwd, ".sap-devs")); err == nil {
		return LayerProject, cwd
	}

	// Fall back to user layer
	paths, err := xdg.New()
	if err != nil {
		return LayerUser, ""
	}
	return LayerUser, paths.DataDir
}

func gitRemoteURL(dir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func isOfficialRepo(dir string) bool {
	url, err := gitRemoteURL(dir)
	if err != nil {
		return false
	}
	return strings.Contains(url, officialRepoURL)
}

func isCompanyRepo(dir string) bool {
	url, err := gitRemoteURL(dir)
	if err != nil {
		return false
	}

	paths, err := xdg.New()
	if err != nil {
		return false
	}
	cfg, err := config.Load(paths.ConfigDir)
	if err != nil || cfg.CompanyRepo == "" {
		return false
	}

	return strings.Contains(url, cfg.CompanyRepo)
}

// LayerInfo pairs a Layer with its packs directory on disk.
type LayerInfo struct {
	Layer Layer
	Dir   string
}

// AllLayers returns the directories for each content layer that exists on disk.
func AllLayers(cwd string) []LayerInfo {
	paths, _ := xdg.New()

	var layers []LayerInfo

	// Official: CWD checkout or cache
	if _, err := os.Stat(filepath.Join(cwd, "content", "packs")); err == nil && isOfficialRepo(cwd) {
		layers = append(layers, LayerInfo{LayerOfficial, filepath.Join(cwd, "content", "packs")})
	} else if paths != nil {
		officialDir := filepath.Join(paths.CacheDir, "official", "content", "packs")
		if _, err := os.Stat(officialDir); err == nil {
			layers = append(layers, LayerInfo{LayerOfficial, officialDir})
		}
	}

	// Company
	if paths != nil {
		cfg, _ := config.Load(paths.ConfigDir)
		if cfg != nil && cfg.CompanyRepo != "" {
			companyDir := filepath.Join(paths.CacheDir, "company", "content", "packs")
			if _, err := os.Stat(companyDir); err == nil {
				layers = append(layers, LayerInfo{LayerCompany, companyDir})
			}
		}
	}

	// User
	if paths != nil {
		userDir := filepath.Join(paths.DataDir, "packs")
		if _, err := os.Stat(userDir); err == nil {
			layers = append(layers, LayerInfo{LayerUser, userDir})
		}
	}

	// Project
	projectDir := filepath.Join(cwd, ".sap-devs", "packs")
	if _, err := os.Stat(projectDir); err == nil {
		layers = append(layers, LayerInfo{LayerProject, projectDir})
	}

	return layers
}
