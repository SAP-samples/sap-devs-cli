package project

import (
	"strings"
	"testing"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// findBySeverity returns all findings with the given severity.
func findBySeverity(findings []Finding, severity string) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Severity == severity {
			out = append(out, f)
		}
	}
	return out
}

// findByCategory returns all findings with the given category.
func findByCategory(findings []Finding, category string) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Category == category {
			out = append(out, f)
		}
	}
	return out
}

// findByMessage returns all findings whose Message contains substr.
func findByMessage(findings []Finding, substr string) []Finding {
	var out []Finding
	for _, f := range findings {
		if strings.Contains(f.Message, substr) {
			out = append(out, f)
		}
	}
	return out
}

// TestCheck_DefaultEnvNotGitignored expects a "practice" error when
// default-env.json exists but is not in .gitignore.
func TestCheck_DefaultEnvNotGitignored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {"@sap/cds": "9.6.2"},
		"devDependencies": {"@sap/cds-dk": "9.6.2"},
		"scripts": {"lint": "cds lint ."}
	}`)
	writeFile(t, dir, "default-env.json", `{}`)
	// No .gitignore — so default-env.json is not gitignored.

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}

	findings := Check(ctx, dir, nil)

	practice := findByCategory(findings, "practice")
	if len(practice) == 0 {
		t.Fatal("expected a practice finding, got none")
	}
	errors := findBySeverity(practice, "error")
	if len(errors) == 0 {
		t.Errorf("expected practice finding with severity=error, got %+v", practice)
	}
	if len(findByMessage(errors, "default-env.json")) == 0 {
		t.Errorf("expected message to mention default-env.json, got %+v", errors)
	}
}

// TestCheck_DefaultEnvGitignored expects no practice error when default-env.json
// is present but listed in .gitignore.
func TestCheck_DefaultEnvGitignored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {"@sap/cds": "9.6.2"},
		"devDependencies": {"@sap/cds-dk": "9.6.2"},
		"scripts": {"lint": "cds lint ."}
	}`)
	writeFile(t, dir, "default-env.json", `{}`)
	writeFile(t, dir, ".gitignore", "node_modules\ndefault-env.json\n")

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}

	findings := Check(ctx, dir, nil)

	for _, f := range findings {
		if f.Category == "practice" && f.Severity == "error" {
			t.Errorf("unexpected practice error: %s", f.Message)
		}
	}
}

// TestCheck_VersionStaleness expects a version finding when the project's
// @sap/cds is behind the version declared in pack metadata.
func TestCheck_VersionStaleness(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {"@sap/cds": "9.4.0"},
		"devDependencies": {"@sap/cds-dk": "9.4.0"},
		"scripts": {"lint": "cds lint ."}
	}`)
	writeFile(t, dir, ".gitignore", "node_modules\n")

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.CAPVersion != "9.4.0" {
		t.Fatalf("CAPVersion = %q, want 9.4.0", ctx.CAPVersion)
	}

	packs := []*content.Pack{
		{
			ID:       "cap",
			Versions: map[string]string{"@sap/cds": "9.8.0"},
		},
	}

	findings := Check(ctx, dir, packs)

	versionFindings := findByCategory(findings, "version")
	if len(versionFindings) == 0 {
		t.Fatal("expected a version finding, got none")
	}
	if len(findByMessage(versionFindings, "9.4.0")) == 0 {
		t.Errorf("expected version finding to mention current version 9.4.0, got %+v", versionFindings)
	}
	if len(findByMessage(versionFindings, "9.8.0")) == 0 {
		t.Errorf("expected version finding to mention latest version 9.8.0, got %+v", versionFindings)
	}
}

// TestCheck_MissingLintScript expects a constraint warning when package.json
// has no "lint" script.
func TestCheck_MissingLintScript(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {"@sap/cds": "9.8.0"},
		"devDependencies": {"@sap/cds-dk": "9.8.0"},
		"scripts": {"start": "cds-serve"}
	}`)
	writeFile(t, dir, ".gitignore", "node_modules\n")

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}

	findings := Check(ctx, dir, nil)

	constraint := findByCategory(findings, "constraint")
	if len(constraint) == 0 {
		t.Fatal("expected a constraint finding, got none")
	}
	warnings := findBySeverity(constraint, "warning")
	if len(warnings) == 0 {
		t.Errorf("expected constraint finding with severity=warning, got %+v", constraint)
	}
	if len(findByMessage(warnings, "lint")) == 0 {
		t.Errorf("expected warning to mention lint, got %+v", warnings)
	}
}

// TestCheck_NoFindingsForCleanProject expects zero findings for a fully
// up-to-date CAP project with lint script and default-env.json gitignored.
func TestCheck_NoFindingsForCleanProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {"@sap/cds": "9.8.0"},
		"devDependencies": {"@sap/cds-dk": "9.8.0"},
		"scripts": {"lint": "cds lint .", "start": "cds-serve"}
	}`)
	writeFile(t, dir, ".gitignore", "node_modules\ndefault-env.json\n")

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}

	packs := []*content.Pack{
		{
			ID:       "cap",
			Versions: map[string]string{"@sap/cds": "9.8.0"},
		},
	}

	findings := Check(ctx, dir, packs)

	// Filter out the dependency warning for default-env.json not in gitignore —
	// in a clean project the .gitignore already contains it so there should be no
	// dependency findings either, and no practice/constraint/version findings.
	for _, f := range findings {
		t.Errorf("unexpected finding: category=%s severity=%s message=%s", f.Category, f.Severity, f.Message)
	}
}
