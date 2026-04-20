package project

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

// Finding is a single health-check result for the current project.
type Finding struct {
	Category string // "dependency", "version", "practice", "constraint"
	Severity string // "error", "warning", "info"
	Message  string
	Fix      string
}

// Check runs all health-check categories against the project and returns a
// (possibly empty) slice of findings.
func Check(ctx *ProjectContext, cwd string, packs []*content.Pack) []Finding {
	var findings []Finding
	findings = append(findings, checkDependencies(ctx, cwd)...)
	findings = append(findings, checkVersions(ctx, cwd, packs)...)
	findings = append(findings, checkPractices(ctx, cwd)...)
	findings = append(findings, checkConstraints(ctx, cwd)...)
	return findings
}

// checkDependencies warns about missing common CAP development dependencies.
func checkDependencies(ctx *ProjectContext, cwd string) []Finding {
	if !strings.HasPrefix(ctx.Type, "CAP") {
		return nil
	}

	var findings []Finding

	pkg, err := readPackageJSON(cwd)
	if err != nil {
		return nil
	}

	// Warn if @sap/cds-dk is absent from devDependencies (needed for cds CLI).
	if _, inDeps := pkg.Dependencies["@sap/cds-dk"]; !inDeps {
		if _, inDevDeps := pkg.DevDependencies["@sap/cds-dk"]; !inDevDeps {
			findings = append(findings, Finding{
				Category: "dependency",
				Severity: "warning",
				Message:  "@sap/cds-dk is not listed in devDependencies",
				Fix:      "npm add -D @sap/cds-dk",
			})
		}
	}

	// Warn if .gitignore does not mention default-env.json.
	if !isGitignored(cwd, "default-env.json") {
		findings = append(findings, Finding{
			Category: "dependency",
			Severity: "warning",
			Message:  "default-env.json is not listed in .gitignore",
			Fix:      `Add "default-env.json" to .gitignore`,
		})
	}

	return findings
}

// checkVersions compares the detected @sap/cds version against the latest
// version advertised in pack metadata.
func checkVersions(ctx *ProjectContext, cwd string, packs []*content.Pack) []Finding {
	if ctx.CAPVersion == "" {
		return nil
	}

	versions := collectVersions(packs)
	latest, ok := versions["@sap/cds"]
	if !ok {
		return nil
	}

	staleness := VersionStaleness(ctx.CAPVersion, latest)
	if staleness == "" {
		return nil
	}

	return []Finding{{
		Category: "version",
		Severity: staleness,
		Message:  "@sap/cds " + ctx.CAPVersion + " is outdated (latest: " + latest + ")",
		Fix:      "npm update @sap/cds @sap/cds-dk",
	}}
}

// checkPractices flags risky development practices.
func checkPractices(ctx *ProjectContext, cwd string) []Finding {
	if !strings.HasPrefix(ctx.Type, "CAP") {
		return nil
	}

	var findings []Finding

	// Error if default-env.json exists and is not gitignored — credential leak risk.
	if ctx.HasDefaultEnv && !isGitignored(cwd, "default-env.json") {
		findings = append(findings, Finding{
			Category: "practice",
			Severity: "error",
			Message:  "default-env.json exists but is not in .gitignore (credential leak risk)",
			Fix:      `Add "default-env.json" to .gitignore immediately`,
		})
	}

	return findings
}

// checkConstraints flags deviations from CAP best practices.
func checkConstraints(ctx *ProjectContext, cwd string) []Finding {
	if !strings.HasPrefix(ctx.Type, "CAP") {
		return nil
	}

	var findings []Finding

	pkg, err := readPackageJSON(cwd)
	if err != nil {
		return nil
	}

	// Warn if no "lint" script is present in package.json.
	if _, ok := pkg.Scripts["lint"]; !ok {
		findings = append(findings, Finding{
			Category: "constraint",
			Severity: "warning",
			Message:  `No "lint" script found in package.json`,
			Fix:      `Add "lint": "cds lint ." to the scripts section of package.json`,
		})
	}

	return findings
}

// collectVersions merges the Versions maps from all packs; the first pack that
// defines a key wins (higher-priority packs appear earlier in the slice).
func collectVersions(packs []*content.Pack) map[string]string {
	merged := make(map[string]string)
	for _, p := range packs {
		for k, v := range p.Versions {
			if _, exists := merged[k]; !exists {
				merged[k] = v
			}
		}
	}
	return merged
}

// isGitignored reports whether filename appears as an exact-match line in
// cwd/.gitignore.  Returns false if the file cannot be read.
func isGitignored(cwd, filename string) bool {
	f, err := os.Open(filepath.Join(cwd, ".gitignore"))
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == filename {
			return true
		}
	}
	return false
}

// readPackageJSON reads and parses package.json from cwd.
func readPackageJSON(cwd string) (packageJSON, error) {
	data, err := os.ReadFile(filepath.Join(cwd, "package.json"))
	if err != nil {
		return packageJSON{}, err
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return packageJSON{}, err
	}
	return pkg, nil
}
