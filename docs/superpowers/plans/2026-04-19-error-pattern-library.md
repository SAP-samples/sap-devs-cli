# Error Pattern Library Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `known_errors.yaml` per pack with struct, loader, merge, schema, injection rendering, CLI commands, i18n, and seed data.

**Architecture:** Flat-array YAML (`known_errors.yaml`) per pack following the identical pattern used by `samples.yaml` — struct in `pack.go`, load in `LoadPack`, ID-based merge in `merge.go`, table rendering in `render.go`, helpers in `known_errors.go`, CLI in `cmd/known_errors.go`.

**Tech Stack:** Go, cobra, YAML, JSON Schema Draft-07

**Spec:** `docs/superpowers/specs/2026-04-19-error-pattern-library-design.md`

---

### Task 1: Data Model — KnownError struct and Pack field

**Files:**
- Modify: `internal/content/pack.go:14-57` (Pack struct)
- Modify: `internal/content/pack.go:128-138` (after Sample struct)

- [ ] **Step 1: Add KnownError struct after the Sample struct (line ~138)**

Add this after the `Sample` struct definition and before the `TutorialRef` struct:

```go
// KnownError is a common SAP error pattern with its cause and fix.
type KnownError struct {
	ID      string   `yaml:"id"`
	Pattern string   `yaml:"pattern"`
	Cause   string   `yaml:"cause"`
	Fix     string   `yaml:"fix"`
	Docs    string   `yaml:"docs,omitempty"`
	Tags    []string `yaml:"tags,omitempty"`
	PackID  string
}
```

- [ ] **Step 2: Add KnownErrors field to the Pack struct**

Add `KnownErrors []KnownError` to the Pack struct, after the `Samples` field (line ~37):

```go
Samples      []Sample
KnownErrors  []KnownError
TutorialRefs []TutorialRef
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: Compiles successfully.

- [ ] **Step 4: Commit**

```bash
git add internal/content/pack.go
git commit -m "feat(errors): add KnownError struct and Pack field"
```

---

### Task 2: Loader — Load known_errors.yaml in LoadPack

**Files:**
- Modify: `internal/content/pack.go:394-405` (LoadPack function, after samples.yaml loading)

- [ ] **Step 1: Add known_errors.yaml loading**

In `LoadPack`, after the `samples.yaml` loading block (around line 399) and before the `tutorials.yaml` block, add:

```go
if data, err := os.ReadFile(filepath.Join(packDir, "known_errors.yaml")); err == nil {
	_ = yaml.Unmarshal(data, &pack.KnownErrors)
	for i := range pack.KnownErrors {
		pack.KnownErrors[i].PackID = pack.ID
	}
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add internal/content/pack.go
git commit -m "feat(errors): load known_errors.yaml in LoadPack"
```

---

### Task 3: Merge — mergeKnownErrors function

**Files:**
- Modify: `internal/content/merge.go:61` (after `mergeSamples` call in MergeWith)
- Modify: `internal/content/merge.go` (new function at end of file)

- [ ] **Step 1: Add mergeKnownErrors function at end of merge.go**

After the `mergeLearningRefs` function, add:

```go
// mergeKnownErrors builds a fresh []KnownError: starts with base entries, replaces
// any entry whose ID matches an additive entry, appends unmatched additive entries.
// PackID is re-stamped to packID on every entry in the result.
func mergeKnownErrors(base, additive []KnownError, packID string) []KnownError {
	result := make([]KnownError, len(base))
	copy(result, base)
	for _, a := range additive {
		replaced := false
		for i, b := range result {
			if b.ID == a.ID {
				result[i] = a
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, a)
		}
	}
	for i := range result {
		result[i].PackID = packID
	}
	return result
}
```

- [ ] **Step 2: Add merge call in MergeWith**

In `MergeWith`, after the `mergeSamples` call (line ~61), add:

```go
merged.KnownErrors = mergeKnownErrors(base.KnownErrors, a.KnownErrors, base.ID)
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: Compiles successfully.

- [ ] **Step 4: Commit**

```bash
git add internal/content/merge.go
git commit -m "feat(errors): add mergeKnownErrors for additive layers"
```

---

### Task 4: Helper functions — FlattenKnownErrors, FilterKnownErrors, etc.

**Files:**
- Create: `internal/content/known_errors.go`

- [ ] **Step 1: Create known_errors.go with all helper functions**

Create `internal/content/known_errors.go`:

```go
package content

import "strings"

// FlattenKnownErrors collects all known errors from all packs into a single slice.
func FlattenKnownErrors(packs []*Pack) []KnownError {
	var out []KnownError
	for _, p := range packs {
		out = append(out, p.KnownErrors...)
	}
	return out
}

// FilterKnownErrorsByTags returns errors with at least one tag matching any of the
// provided tags (OR semantics, case-insensitive).
func FilterKnownErrorsByTags(errors []KnownError, tags []string) []KnownError {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[strings.ToLower(strings.TrimSpace(t))] = true
	}
	var out []KnownError
	for _, e := range errors {
		for _, t := range e.Tags {
			if tagSet[strings.ToLower(t)] {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// FilterKnownErrorsByPack returns errors from the pack matching the given pack ID.
func FilterKnownErrorsByPack(packs []*Pack, packID string) []KnownError {
	for _, p := range packs {
		if p.ID == packID {
			return p.KnownErrors
		}
	}
	return nil
}

// FilterKnownErrors returns errors whose ID, Pattern, Cause, Fix, or any Tag
// contains query (case-insensitive substring match).
func FilterKnownErrors(errors []KnownError, query string) []KnownError {
	q := strings.ToLower(query)
	var out []KnownError
	for _, e := range errors {
		if strings.Contains(strings.ToLower(e.ID), q) ||
			strings.Contains(strings.ToLower(e.Pattern), q) ||
			strings.Contains(strings.ToLower(e.Cause), q) ||
			strings.Contains(strings.ToLower(e.Fix), q) {
			out = append(out, e)
			continue
		}
		for _, tag := range e.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// FindKnownError returns a pointer to the first error with an exact ID match, or nil.
func FindKnownError(errors []KnownError, id string) *KnownError {
	for i := range errors {
		if errors[i].ID == id {
			return &errors[i]
		}
	}
	return nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add internal/content/known_errors.go
git commit -m "feat(errors): add flatten, filter, and find helpers"
```

---

### Task 5: Injection rendering — Known Errors table in RenderContext

**Files:**
- Modify: `internal/content/render.go:86-104` (RenderContext, after Learning Journeys section)

- [ ] **Step 1: Add escapePipe helper at top of render.go**

After the existing `var` block (line ~19), add:

```go
// escapePipe replaces literal pipe characters with escaped pipes for Markdown tables.
func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
```

- [ ] **Step 2: Add Known Errors table rendering in RenderContext**

In `RenderContext`, after the Learning Journeys block (after line ~102, before the final `return`), add:

```go
var knownErrors []KnownError
for _, p := range packs {
	knownErrors = append(knownErrors, p.KnownErrors...)
}
if len(knownErrors) > 0 {
	b.WriteString("## Known Errors\n\n")
	b.WriteString("| Error Pattern | Cause | Fix |\n")
	b.WriteString("|---|---|---|\n")
	for _, e := range knownErrors {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
			escapePipe(e.Pattern), escapePipe(e.Cause), escapePipe(e.Fix)))
	}
	b.WriteString("\n")
}
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: Compiles successfully.

- [ ] **Step 4: Commit**

```bash
git add internal/content/render.go
git commit -m "feat(errors): render Known Errors table in injected context"
```

---

### Task 6: JSON Schema

**Files:**
- Create: `content/schemas/known_errors.schema.json`
- Modify: `.vscode/settings.json`

- [ ] **Step 1: Create the JSON schema file**

Create `content/schemas/known_errors.schema.json`:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Pack Known Errors",
  "description": "Schema for sap-devs known_errors.yaml files (top-level array)",
  "type": "array",
  "items": {
    "type": "object",
    "required": ["id", "pattern", "cause", "fix"],
    "additionalProperties": false,
    "properties": {
      "id": {
        "type": "string",
        "pattern": "^[a-z][a-z0-9-]*/[a-z][a-z0-9-]*$",
        "description": "Error identifier in format <pack-id>/<slug>, e.g. cap/no-default-db"
      },
      "pattern": {
        "type": "string",
        "description": "The error message pattern agents will encounter"
      },
      "cause": {
        "type": "string",
        "description": "Why this error occurs"
      },
      "fix": {
        "type": "string",
        "description": "How to resolve this error"
      },
      "docs": {
        "type": "string",
        "format": "uri",
        "description": "Link to relevant documentation"
      },
      "tags": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Filtering tags (e.g. database, auth, build)"
      }
    }
  }
}
```

- [ ] **Step 2: Wire the schema in .vscode/settings.json**

Add this entry to the `yaml.schemas` object:

```json
"./content/schemas/known_errors.schema.json": "**/packs/*/known_errors.yaml"
```

- [ ] **Step 3: Commit**

```bash
git add content/schemas/known_errors.schema.json .vscode/settings.json
git commit -m "feat(errors): add JSON Schema and VS Code validation wiring"
```

---

### Task 7: i18n catalog entries

**Files:**
- Modify: `internal/i18n/catalogs/en.json`
- Modify: `internal/i18n/catalogs/de.json`

- [ ] **Step 1: Add English i18n keys**

Add these entries to `en.json` (after the `samples.*` block):

```json
"errors.short": "Browse known SAP error patterns",
"errors.long": "Browse and search known SAP error patterns with their causes and fixes.",
"errors.list.short": "List known errors for your active profile",
"errors.search.short": "Search across all known error patterns",
"errors.search.no_results": "No errors found matching \"{{.Query}}\".",
"errors.list.no_profile": "no profile set — run 'sap-devs profile set <name>' first",
"errors.list.profile_not_found": "profile \"{{.ID}}\" not found — run 'sap-devs sync' to refresh content",
"errors.none": "No known errors found for your current profile.",
"errors.none_pack": "No known errors found for pack \"{{.Pack}}\".",
"errors.none_tags": "No known errors match tags {{.Tags}}.",
"errors.col_pattern": "PATTERN",
"errors.col_cause": "CAUSE",
"errors.col_pack": "PACK",
"errors.col_fix": "FIX",
"errors.col_docs": "DOCS"
```

- [ ] **Step 2: Add German i18n keys**

Add these entries to `de.json` (after the `samples.*` block):

```json
"errors.short": "Bekannte SAP-Fehlermuster durchsuchen",
"errors.long": "Bekannte SAP-Fehlermuster mit Ursachen und Lösungen durchsuchen.",
"errors.list.short": "Bekannte Fehler für dein aktives Profil auflisten",
"errors.search.short": "Alle bekannten Fehlermuster durchsuchen",
"errors.search.no_results": "Keine Fehler gefunden für \"{{.Query}}\".",
"errors.list.no_profile": "Kein Profil gesetzt — führe 'sap-devs profile set <name>' aus",
"errors.list.profile_not_found": "Profil \"{{.ID}}\" nicht gefunden — führe 'sap-devs sync' aus",
"errors.none": "Keine bekannten Fehler für dein aktuelles Profil gefunden.",
"errors.none_pack": "Keine bekannten Fehler für Pack \"{{.Pack}}\" gefunden.",
"errors.none_tags": "Keine bekannten Fehler für Tags {{.Tags}}.",
"errors.col_pattern": "MUSTER",
"errors.col_cause": "URSACHE",
"errors.col_pack": "PACK",
"errors.col_fix": "LÖSUNG",
"errors.col_docs": "DOKU"
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: Compiles successfully (embedded catalogs compile at build time).

- [ ] **Step 4: Commit**

```bash
git add internal/i18n/catalogs/en.json internal/i18n/catalogs/de.json
git commit -m "feat(errors): add i18n catalog entries for errors command"
```

---

### Task 8: CLI command — sap-devs errors list/search

**Files:**
- Create: `cmd/known_errors.go`
- Modify: `cmd/root.go` (not needed — init() in new file auto-registers)

- [ ] **Step 1: Create cmd/known_errors.go**

Create `cmd/known_errors.go` following the `cmd/samples.go` pattern:

```go
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var (
	errorsAll  bool
	errorsPack string
	errorsTags string
)

var errorsCmd = &cobra.Command{
	Use:   "errors",
	Short: i18n.T("en", "errors.short"),
	Long:  i18n.T("en", "errors.long"),
}

var errorsListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("en", "errors.list.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		var packs []*content.Pack

		if errorsPack != "" || errorsAll {
			packs, err = loader.LoadPacks(nil, i18n.ActiveLang)
			if err != nil {
				return err
			}
		} else {
			paths, err := xdg.New()
			if err != nil {
				return err
			}
			profileCfg, err := config.LoadProfile(paths.ConfigDir)
			if err != nil {
				return err
			}
			if profileCfg.ID == "" {
				return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "errors.list.no_profile"))
			}
			activeProfile, err := loader.FindProfile(profileCfg.ID)
			if err != nil {
				return err
			}
			if activeProfile == nil {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "errors.list.profile_not_found", map[string]any{"ID": profileCfg.ID}))
			}
			packs, err = loader.LoadPacks(activeProfile, i18n.ActiveLang)
			if err != nil {
				return err
			}
		}

		var errors []content.KnownError
		if errorsPack != "" {
			errors = content.FilterKnownErrorsByPack(packs, errorsPack)
		} else {
			errors = content.FlattenKnownErrors(packs)
		}

		if errorsTags != "" {
			tags := strings.Split(errorsTags, ",")
			errors = content.FilterKnownErrorsByTags(errors, tags)
		}

		if len(errors) == 0 {
			if errorsPack != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "errors.none_pack", map[string]any{"Pack": errorsPack}))
			} else if errorsTags != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "errors.none_tags", map[string]any{"Tags": errorsTags}))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "errors.none"))
			}
			return nil
		}
		printErrorTable(errors, true)
		return nil
	},
}

var errorsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T("en", "errors.search.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		errors := content.FilterKnownErrors(content.FlattenKnownErrors(packs), args[0])
		if len(errors) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "errors.search.no_results", map[string]any{"Query": args[0]}))
			return nil
		}
		printErrorDetails(errors)
		return nil
	},
}

func printErrorTable(errors []content.KnownError, showPack bool) {
	colPattern := i18n.T(i18n.ActiveLang, "errors.col_pattern")
	colCause := i18n.T(i18n.ActiveLang, "errors.col_cause")
	colPack := i18n.T(i18n.ActiveLang, "errors.col_pack")
	if showPack {
		fmt.Printf("%-45s %-12s %s\n", colPattern, colPack, colCause)
		fmt.Println(strings.Repeat("-", 100))
		for _, e := range errors {
			fmt.Printf("%-45s %-12s %s\n", truncate(e.Pattern, 44), e.PackID, truncate(e.Cause, 50))
		}
	} else {
		fmt.Printf("%-45s %s\n", colPattern, colCause)
		fmt.Println(strings.Repeat("-", 90))
		for _, e := range errors {
			fmt.Printf("%-45s %s\n", truncate(e.Pattern, 44), truncate(e.Cause, 50))
		}
	}
}

func printErrorDetails(errors []content.KnownError) {
	colPack := i18n.T(i18n.ActiveLang, "errors.col_pack")
	colCause := i18n.T(i18n.ActiveLang, "errors.col_cause")
	colFix := i18n.T(i18n.ActiveLang, "errors.col_fix")
	colDocs := i18n.T(i18n.ActiveLang, "errors.col_docs")
	for i, e := range errors {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("  %s\n", e.Pattern)
		fmt.Printf("  %s:  %s\n", colPack, e.PackID)
		fmt.Printf("  %s: %s\n", colCause, e.Cause)
		fmt.Printf("  %s:   %s\n", colFix, e.Fix)
		if e.Docs != "" {
			fmt.Printf("  %s:  %s\n", colDocs, e.Docs)
		}
	}
}

func init() {
	errorsListCmd.Flags().BoolVarP(&errorsAll, "all", "a", false, "show all errors regardless of profile")
	errorsListCmd.Flags().StringVarP(&errorsPack, "pack", "p", "", "filter to a specific pack")
	errorsListCmd.Flags().StringVarP(&errorsTags, "tags", "t", "", "comma-separated tags (OR match)")
	errorsCmd.AddCommand(errorsListCmd, errorsSearchCmd)
	rootCmd.AddCommand(errorsCmd)
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: Compiles successfully.

- [ ] **Step 3: Smoke test**

Run: `SAP_DEVS_DEV=1 go run . errors list`
Expected: Empty output or "No known errors found" (no seed data yet).

- [ ] **Step 4: Commit**

```bash
git add cmd/known_errors.go
git commit -m "feat(errors): add errors list and errors search CLI commands"
```

---

### Task 9: Seed data — CAP known_errors.yaml

**Files:**
- Create: `content/packs/cap/known_errors.yaml`

- [ ] **Step 1: Create CAP seed data**

Create `content/packs/cap/known_errors.yaml`:

```yaml
- id: cap/no-default-db
  pattern: "No 'default' database configured"
  cause: No database binding in cds.requires; common in new projects or missing .env
  fix: "Add `cds.requires.db.kind = sqlite` to package.json for local dev, or bind a HANA service instance for BTP deployment"
  docs: https://cap.cloud.sap/docs/node.js/databases
  tags: [database, local-dev]

- id: cap/eisdir
  pattern: "EISDIR: illegal operation on a directory"
  cause: cds build or cds watch targeting a directory instead of a file; often caused by stale gen/ folders
  fix: Check cds.build paths in package.json and remove stale gen/ folders with `rm -rf gen/`
  tags: [build, filesystem]

- id: cap/no-service-def
  pattern: "No service definition found in loaded models"
  cause: No .cds files found in srv/ or the cds model paths are misconfigured
  fix: Ensure at least one .cds file in srv/ contains a `service` definition, and check that package.json cds.roots includes the correct paths
  docs: https://cap.cloud.sap/docs/get-started/
  tags: [service, getting-started]

- id: cap/cannot-find-cds
  pattern: "Cannot find module '@sap/cds'"
  cause: "@sap/cds not installed; missing from dependencies or node_modules not populated"
  fix: "Run `npm install` to install dependencies, or `npm add @sap/cds` if not listed in package.json"
  tags: [dependencies, getting-started]

- id: cap/hdi-not-bound
  pattern: "No credentials configured for HDI container"
  cause: HDI container service not bound to the application; missing service binding on BTP or missing default-env.json locally
  fix: "Bind the HANA HDI container service to your app (`cf bind-service`), or create a default-env.json with `cds bind --exec -- npm start` for local hybrid testing"
  docs: https://cap.cloud.sap/docs/guides/databases-hana
  tags: [database, hana, deployment]

- id: cap/cds-dk-not-found
  pattern: "cds: command not found"
  cause: "@sap/cds-dk not installed globally or not in PATH"
  fix: "Install globally with `npm install -g @sap/cds-dk`, or use `npx cds` to run from local project"
  docs: https://cap.cloud.sap/docs/get-started/
  tags: [tools, getting-started]

- id: cap/draft-no-uuid
  pattern: "Draft requires key element of type UUID"
  cause: Draft-enabled entity uses a non-UUID key; Fiori draft handling requires UUID-typed keys
  fix: "Change the key type to `key ID : UUID;` or add `HasActiveEntity` and `HasDraftEntity` aspects"
  docs: https://cap.cloud.sap/docs/java/fiori-drafts
  tags: [draft, fiori, cds]
```

- [ ] **Step 2: Smoke test loading**

Run: `SAP_DEVS_DEV=1 go run . errors list --all`
Expected: Table showing 7 CAP error patterns.

- [ ] **Step 3: Smoke test search**

Run: `SAP_DEVS_DEV=1 go run . errors search "database"`
Expected: Detailed output for `cap/no-default-db` and `cap/hdi-not-bound`.

- [ ] **Step 4: Commit**

```bash
git add content/packs/cap/known_errors.yaml
git commit -m "feat(errors): seed CAP known_errors.yaml with 7 error patterns"
```

---

### Task 10: Seed data — ABAP known_errors.yaml

**Files:**
- Create: `content/packs/abap/known_errors.yaml`

- [ ] **Step 1: Create ABAP seed data**

Create `content/packs/abap/known_errors.yaml`:

```yaml
- id: abap/amdp-static
  pattern: "AMDP method must be static"
  cause: AMDP (ABAP Managed Database Procedure) methods must be declared as static class methods
  fix: "Change the method definition to `CLASS-METHODS` instead of `METHODS` in the class definition"
  docs: https://help.sap.com/doc/abapdocu_latest_index_htm/latest/en-US/index.htm?file=abenamdp.htm
  tags: [amdp, hana, method]

- id: abap/clean-core-access
  pattern: "Access to SAP object not permitted"
  cause: Using a non-released SAP object in ABAP Cloud / clean core; only tier-1 released APIs are allowed
  fix: Find the released API equivalent using the Released Objects app in ADT, or check api.sap.com for the released API
  docs: https://help.sap.com/docs/abap-cloud/abap-rap/released-apis
  tags: [clean-core, api, abap-cloud]

- id: abap/cds-activation
  pattern: "CDS view activation failed"
  cause: Syntax error or unresolvable dependency in the CDS view definition; common after transport or upgrade
  fix: Check the CDS view source for syntax errors, verify all referenced data sources exist and are active, run `Activate All` from ADT
  tags: [cds, activation, adt]

- id: abap/rap-draft-determine-action
  pattern: "Determination/validation is not allowed for draft instances"
  cause: RAP determination or validation runs on draft data where it is not expected
  fix: "Add `on save` trigger to the determination/validation, or check `%is_draft` in the implementation"
  docs: https://help.sap.com/docs/abap-cloud/abap-rap/determinations
  tags: [rap, draft, determination]

- id: abap/authority-check-failed
  pattern: "No authority for this action (PFCG role missing)"
  cause: User lacks the required PFCG authorization role for the operation
  fix: "Check transaction SU53 for the missing authorization object, then assign the correct role via PFCG"
  tags: [authorization, security, pfcg]
```

- [ ] **Step 2: Smoke test with pack filter**

Run: `SAP_DEVS_DEV=1 go run . errors list --all --pack abap`
Expected: Table showing 5 ABAP error patterns.

- [ ] **Step 3: Commit**

```bash
git add content/packs/abap/known_errors.yaml
git commit -m "feat(errors): seed ABAP known_errors.yaml with 5 error patterns"
```

---

### Task 11: Injection smoke test

**Files:** None (verification only)

- [ ] **Step 1: Test dry-run injection includes Known Errors table**

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run 2>&1 | grep -A 5 "Known Errors"`
Expected: Output contains `## Known Errors` followed by a Markdown table with error patterns.

- [ ] **Step 2: Verify pipe escaping works**

Verify that any error patterns containing `|` characters are escaped as `\|` in the table output. The current seed data does not contain pipes, but the escaping code is in place for future entries.

---

### Task 12: Documentation — CLAUDE.md update

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update the CLI Commands table**

In the `### CLI Commands` table in CLAUDE.md, add a row for `errors`:

```markdown
| `errors` | Browse known SAP error patterns; `errors list/search` |
```

Add it in alphabetical order (after `doctor`, before `events`).

- [ ] **Step 2: Update the Content Layer System section**

In the paragraph that lists YAML files per pack, add `known_errors.yaml` to the list:

Update the sentence in the `### Content Layer System` section that reads:
> `pack.yaml` (metadata), `context.md` (AI context text), `constraints.md` (AI constraint rules), `tips.md` (H2-delimited tips), `tools.yaml`, `resources.yaml`, `mcp.yaml`, `samples.yaml` (canonical code sample references).

To include `known_errors.yaml (common SAP error patterns)` in the list.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add errors command and known_errors.yaml to CLAUDE.md"
```

---

### Task 13: Final verification

**Files:** None (verification only)

- [ ] **Step 1: Full build**

Run: `go build ./...`
Expected: Clean build with no errors.

- [ ] **Step 2: go vet**

Run: `go vet ./...`
Expected: No issues.

- [ ] **Step 3: End-to-end test**

Run these commands sequentially:

```bash
SAP_DEVS_DEV=1 go run . errors list --all
SAP_DEVS_DEV=1 go run . errors list --all --pack cap
SAP_DEVS_DEV=1 go run . errors list --all --tags database
SAP_DEVS_DEV=1 go run . errors search "AMDP"
SAP_DEVS_DEV=1 go run . errors search "nonexistent-pattern"
SAP_DEVS_DEV=1 go run . inject --dry-run | grep "Known Errors"
```

Expected:
1. All 12 errors (7 CAP + 5 ABAP) listed
2. Only CAP errors listed
3. Only database-tagged errors listed
4. Detailed output for `abap/amdp-static`
5. "No errors found" message
6. `## Known Errors` appears in dry-run output
