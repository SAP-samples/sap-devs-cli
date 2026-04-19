package project

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
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
	BTPSubaccount string
	BTPRegion     string
	BTPIsTrial    bool
	CFOrg         string
	CFSpace       string
	CFRegion      string
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
	detectBTP(ctx)
	detectCF(ctx)
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

var reCFRegion = regexp.MustCompile(`api\.cf\.([a-z0-9-]+)\.hana\.ondemand\.com`)
var reBTPRegion = regexp.MustCompile(`^([a-z]{2}\d{2})`)

type cfConfig struct {
	Target             string `json:"Target"`
	OrganizationFields struct {
		Name string `json:"Name"`
	} `json:"OrganizationFields"`
	SpaceFields struct {
		Name string `json:"Name"`
	} `json:"SpaceFields"`
}

func extractCFRegion(target string) string {
	m := reCFRegion.FindStringSubmatch(target)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func detectCF(ctx *ProjectContext) {
	cfg := readCFConfig()
	if cfg == nil {
		cfCLIFallback(ctx)
		return
	}
	ctx.CFOrg = cfg.OrganizationFields.Name
	ctx.CFSpace = cfg.SpaceFields.Name
	ctx.CFRegion = extractCFRegion(cfg.Target)
}

func parseCFTargetOutput(output string) (org, space, target string) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "org:") {
			org = strings.TrimSpace(strings.TrimPrefix(line, "org:"))
		} else if strings.HasPrefix(line, "space:") {
			space = strings.TrimSpace(strings.TrimPrefix(line, "space:"))
		} else if strings.HasPrefix(line, "API endpoint:") {
			target = strings.TrimSpace(strings.TrimPrefix(line, "API endpoint:"))
		}
	}
	return
}

func cfCLIFallback(ctx *ProjectContext) {
	c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(c, "cf", "target").Output()
	if err != nil {
		return
	}
	org, space, target := parseCFTargetOutput(string(out))
	ctx.CFOrg = org
	ctx.CFSpace = space
	ctx.CFRegion = extractCFRegion(target)
}

func readCFConfig() *cfConfig {
	cfHome := os.Getenv("CF_HOME")
	if cfHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		cfHome = home
	}
	data, err := os.ReadFile(filepath.Join(cfHome, ".cf", "config.json"))
	if err != nil {
		return nil
	}
	var cfg cfConfig
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	if cfg.OrganizationFields.Name == "" {
		return nil
	}
	return &cfg
}

type btpConfig struct {
	TargetHierarchy struct {
		GlobalAccountSubdomain string `json:"GlobalAccountSubdomain"`
		SubaccountSubdomain    string `json:"SubaccountSubdomain"`
	} `json:"TargetHierarchy"`
	CLIServerURL string `json:"CLIServerURL"`
}

func extractBTPRegion(subdomain string) string {
	m := reBTPRegion.FindStringSubmatch(subdomain)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func detectBTP(ctx *ProjectContext) {
	cfg := readBTPConfig()
	if cfg == nil {
		btpCLIFallback(ctx)
		return
	}
	ctx.BTPSubaccount = cfg.TargetHierarchy.SubaccountSubdomain
	if ctx.BTPSubaccount == "" {
		btpCLIFallback(ctx)
		return
	}
	ctx.BTPRegion = extractBTPRegion(ctx.BTPSubaccount)
	ctx.BTPIsTrial = strings.Contains(strings.ToLower(ctx.BTPSubaccount), "trial")
}

func btpCLIFallback(ctx *ProjectContext) {
	c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(c, "btp", "--format", "json", "target").Output()
	if err != nil {
		return
	}
	var result struct {
		SubAccount struct {
			Subdomain string `json:"subdomain"`
		} `json:"subAccount"`
	}
	if json.Unmarshal(out, &result) != nil {
		return
	}
	if result.SubAccount.Subdomain == "" {
		return
	}
	ctx.BTPSubaccount = result.SubAccount.Subdomain
	ctx.BTPRegion = extractBTPRegion(ctx.BTPSubaccount)
	ctx.BTPIsTrial = strings.Contains(strings.ToLower(ctx.BTPSubaccount), "trial")
}

func readBTPConfig() *btpConfig {
	path := os.Getenv("BTP_CLIENTCONFIG")
	if path == "" {
		path = defaultBTPConfigPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg btpConfig
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	return &cfg
}

func defaultBTPConfigPath() string {
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			return filepath.Join(appdata, "SAP", "btp", "config.json")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	// BTP CLI v2.x uses ~/.config/btp/; older versions used ~/.config/.btp/
	primary := filepath.Join(home, ".config", "btp", "config.json")
	if fileExists(primary) {
		return primary
	}
	return filepath.Join(home, ".config", ".btp", "config.json")
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
