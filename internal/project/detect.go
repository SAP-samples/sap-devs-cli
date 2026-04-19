package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type ProjectContext struct {
	Type          string
	CAPVersion    string
	LatestCAP     string
	Database      string
	Deployment    string
	Auth          string
	HasCDSRC      bool
	HasDefaultEnv bool
	Facts         []Fact
	RawFiles      map[string]bool
}

type Fact struct {
	Key   string
	Value string
	Warn  string
}

func Detect(cwd string) (*ProjectContext, error) {
	if cwd == "" {
		return &ProjectContext{RawFiles: make(map[string]bool)}, nil
	}
	ctx := &ProjectContext{RawFiles: make(map[string]bool)}
	detectCAP(cwd, ctx)
	detectMTA(cwd, ctx)
	detectAuth(cwd, ctx)
	detectAppRouter(cwd, ctx)
	detectKyma(cwd, ctx)
	detectDefaultEnv(cwd, ctx)
	buildFacts(ctx)
	return ctx, nil
}

func detectCAP(cwd string, ctx *ProjectContext) {
	// .cdsrc.json
	if fileExists(filepath.Join(cwd, ".cdsrc.json")) {
		ctx.HasCDSRC = true
		ctx.RawFiles[".cdsrc.json"] = true
	}

	// package.json — CAP Node.js
	pkgPath := filepath.Join(cwd, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		ctx.RawFiles["package.json"] = true
		var pkg packageJSON
		if json.Unmarshal(data, &pkg) == nil {
			if v, ok := pkg.Dependencies["@sap/cds"]; ok {
				ctx.Type = "CAP (Node.js)"
				ctx.CAPVersion = cleanVersion(v)
			} else if _, ok := pkg.DevDependencies["@sap/cds"]; ok {
				ctx.Type = "CAP (Node.js)"
			}
			// Database detection from cds.requires
			ctx.Database = detectDatabase(pkg)
		}
	}

	// pom.xml — CAP Java
	if ctx.Type == "" {
		pomPath := filepath.Join(cwd, "pom.xml")
		if data, err := os.ReadFile(pomPath); err == nil {
			ctx.RawFiles["pom.xml"] = true
			if strings.Contains(string(data), "com.sap.cds") {
				ctx.Type = "CAP (Java)"
			}
		}
	}

	// If .cdsrc.json exists but no @sap/cds dep found, still mark as CAP
	if ctx.HasCDSRC && ctx.Type == "" {
		ctx.Type = "CAP (Node.js)"
	}
}

func detectMTA(cwd string, ctx *ProjectContext) {
	for _, name := range []string{"mta.yaml", ".mta.yaml"} {
		if fileExists(filepath.Join(cwd, name)) {
			ctx.RawFiles[name] = true
			ctx.Deployment = "mta-cf"
			return
		}
	}
}

func detectAuth(cwd string, ctx *ProjectContext) {
	if fileExists(filepath.Join(cwd, "xs-security.json")) {
		ctx.RawFiles["xs-security.json"] = true
		ctx.Auth = "xsuaa"
	}
}

func detectAppRouter(cwd string, ctx *ProjectContext) {
	if fileExists(filepath.Join(cwd, "xs-app.json")) {
		ctx.RawFiles["xs-app.json"] = true
		if ctx.Type == "" {
			ctx.Type = "Fiori / BAS app"
		}
	}
}

func detectKyma(cwd string, ctx *ProjectContext) {
	for _, name := range []string{"chart", "helm"} {
		info, err := os.Stat(filepath.Join(cwd, name))
		if err == nil && info.IsDir() {
			ctx.RawFiles[name+"/"] = true
			if ctx.Deployment == "" {
				ctx.Deployment = "helm-kyma"
			}
			return
		}
	}
}

func detectDefaultEnv(cwd string, ctx *ProjectContext) {
	if fileExists(filepath.Join(cwd, "default-env.json")) {
		ctx.RawFiles["default-env.json"] = true
		ctx.HasDefaultEnv = true
	}
}

// RebuildFacts re-derives the Facts slice from the current typed fields.
// Call this after enriching LatestCAP from pack metadata.
func (ctx *ProjectContext) RebuildFacts() {
	ctx.Facts = nil
	buildFacts(ctx)
}

func buildFacts(ctx *ProjectContext) {
	if ctx.Type == "" {
		if ctx.Deployment == "mta-cf" {
			ctx.Type = "Multi-target Application (MTA)"
		} else if ctx.RawFiles["package.json"] {
			ctx.Type = "Node.js"
		}
	}
	if ctx.Type != "" {
		ctx.Facts = append(ctx.Facts, Fact{Key: "Project type", Value: ctx.Type})
	}
	if ctx.CAPVersion != "" {
		f := Fact{Key: "CAP version", Value: "@sap/cds " + ctx.CAPVersion}
		if ctx.LatestCAP != "" {
			cmp := CompareVersions(ctx.CAPVersion, ctx.LatestCAP)
			if cmp < 0 {
				f.Warn = "update available: " + ctx.LatestCAP
				f.Value += " (latest: " + ctx.LatestCAP + ")"
			}
		}
		ctx.Facts = append(ctx.Facts, f)
	}
	if ctx.Database != "" {
		label := ctx.Database
		if ctx.Database == "hana" {
			label = "SAP HANA Cloud"
		}
		ctx.Facts = append(ctx.Facts, Fact{Key: "Database", Value: label})
	}
	if ctx.Deployment != "" {
		label := ctx.Deployment
		if ctx.Deployment == "mta-cf" {
			label = "MTA to Cloud Foundry"
		} else if ctx.Deployment == "helm-kyma" {
			label = "Helm to Kyma/Kubernetes"
		}
		ctx.Facts = append(ctx.Facts, Fact{Key: "Deployment", Value: label})
	}
	if ctx.Auth != "" {
		ctx.Facts = append(ctx.Facts, Fact{Key: "Auth", Value: "XSUAA (xs-security.json detected)"})
	}
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Scripts         map[string]string `json:"scripts"`
	CDS             *cdsConfig        `json:"cds"`
}

type cdsConfig struct {
	Requires map[string]json.RawMessage `json:"requires"`
}

func detectDatabase(pkg packageJSON) string {
	// Check cds.requires for hana/sqlite/postgres
	if pkg.CDS != nil && pkg.CDS.Requires != nil {
		for key := range pkg.CDS.Requires {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "hana") {
				return "hana"
			}
		}
	}
	// Check dependencies for hana driver
	for dep := range pkg.Dependencies {
		if strings.Contains(dep, "hana") {
			return "hana"
		}
	}
	return ""
}

func cleanVersion(v string) string {
	v = strings.TrimLeft(v, "^~>=<! ")
	return v
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
