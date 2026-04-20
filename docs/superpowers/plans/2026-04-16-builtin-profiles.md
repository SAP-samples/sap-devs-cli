# Built-in Profiles (`all` and `minimal`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two hardcoded built-in profiles — `all` (every pack) and `minimal` (base packs only) — that appear in `sap-devs profile list`, can be selected with `sap-devs profile set`, and require no YAML files on disk.

**Architecture:** Built-in symbols (`BuiltinProfiles()`, `IsBuiltinProfile()`, `reservedProfileIDs`) live in `internal/content/profile.go`. Injection into `LoadProfiles()` and short-circuiting in `FindProfile()` are added to `internal/content/loader.go`. The `minimal` guard in `LoadPacks()` is a two-line addition. The `profile show` command uses `IsBuiltinProfile()` to branch display logic.

**Tech Stack:** Go 1.21+, testify (assert/require), cobra, existing `internal/content` and `cmd` packages.

> **Note for implementors:** The design spec uses lowercase `builtinProfiles()` in some places. This was a spec error — follow the plan. The functions must be exported (`BuiltinProfiles()` and `IsBuiltinProfile()`) because (a) tests live in `package content_test` and cannot access unexported symbols, and (b) `cmd/profile.go` calls `content.IsBuiltinProfile()` across package boundaries. Similarly, the spec's `profile show` guard shows `reservedProfileIDs[p.ID]` directly — that is also wrong (unexported). Use `content.IsBuiltinProfile(p.ID)` as shown in the plan.

---

## File Map

| File | Change |
| --- | --- |
| `internal/content/profile.go` | Add `BuiltinProfiles()`, `IsBuiltinProfile()`, `reservedProfileIDs` |
| `internal/content/loader.go` | Update `ContentLoader.LoadProfiles()`, `ContentLoader.FindProfile()`, `LoadPacks()` |
| `cmd/profile.go` | Extract `renderProfileShow()` helper; add built-in guard |
| `internal/i18n/catalogs/en.json` | Add `profile.show.builtin_note` |
| `internal/i18n/catalogs/de.json` | Add `profile.show.builtin_note` (German) |
| `docs/content/content-guide.md` | Add Built-in Profiles subsection under Profiles |
| `docs/content-authoring.md` | Add note in Base Layer section about `minimal` |
| `internal/content/profile_test.go` | Add 5 new tests |
| `internal/content/loader_test.go` | Add 4 new tests |
| `cmd/profile_test.go` | New file — 1 test for `renderProfileShow` |

---

### Task 1: Built-in profile symbols in `profile.go`

**Files:**
- Modify: `internal/content/profile.go`
- Test: `internal/content/profile_test.go`

**Context:** `profile.go` is the right home for built-in profile data since it already owns the `Profile` struct. Tests are in `package content_test` (see existing file). The package-level `LoadProfiles(profilesDir string)` in this file reads YAML from disk — **do not change it**. The injection into `ContentLoader.LoadProfiles()` is Task 2.

- [ ] **Step 1: Write the failing test**

Add to `internal/content/profile_test.go`:

```go
func TestBuiltinProfiles_ContainsAllAndMinimal(t *testing.T) {
	profiles := content.BuiltinProfiles()
	require.Len(t, profiles, 2)
	ids := map[string]bool{}
	for _, p := range profiles {
		ids[p.ID] = true
		assert.NotEmpty(t, p.Name, "built-in profile %s must have a Name", p.ID)
		assert.NotEmpty(t, p.Description, "built-in profile %s must have a Description", p.ID)
	}
	assert.True(t, ids["all"], "built-in profiles must include 'all'")
	assert.True(t, ids["minimal"], "built-in profiles must include 'minimal'")
}

func TestIsBuiltinProfile_ReturnsTrueForReservedIDs(t *testing.T) {
	assert.True(t, content.IsBuiltinProfile("all"))
	assert.True(t, content.IsBuiltinProfile("minimal"))
	assert.False(t, content.IsBuiltinProfile("cap-developer"))
	assert.False(t, content.IsBuiltinProfile(""))
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/content/... -run "TestBuiltinProfiles|TestIsBuiltinProfile" -v
```

Expected: FAIL — `content.BuiltinProfiles` and `content.IsBuiltinProfile` undefined.

- [ ] **Step 3: Implement — add symbols to `profile.go`**

Add after the `ApplyWeights` function at the bottom of `internal/content/profile.go`:

```go
// reservedProfileIDs is the set of IDs reserved for built-in profiles.
// File-backed profiles with these IDs are silently dropped by ContentLoader.
var reservedProfileIDs = map[string]bool{
	"all":     true,
	"minimal": true,
}

// BuiltinProfiles returns the two hardcoded built-in profiles.
// These profiles require no YAML file on disk.
func BuiltinProfiles() []*Profile {
	return []*Profile{
		{
			ID:          "all",
			Name:        "All Packs",
			Description: "All available packs across every content layer",
		},
		{
			ID:          "minimal",
			Name:        "Minimal",
			Description: "Base layer only — shared SAP ecosystem entry points, no technology-specific packs",
		},
	}
}

// IsBuiltinProfile reports whether id is a reserved built-in profile ID.
func IsBuiltinProfile(id string) bool {
	return reservedProfileIDs[id]
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/content/... -run "TestBuiltinProfiles|TestIsBuiltinProfile" -v
```

Expected: PASS (2 tests).

- [ ] **Step 5: Build check**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/content/profile.go internal/content/profile_test.go
git commit -m "feat(content): add BuiltinProfiles and IsBuiltinProfile to profile.go"
```

---

### Task 2: Inject built-ins in `ContentLoader.LoadProfiles()`

**Files:**
- Modify: `internal/content/loader.go:67-87`
- Test: `internal/content/profile_test.go`

**Context:** `ContentLoader.LoadProfiles()` in `loader.go` (lines 67–87) is the *method* on the `ContentLoader` struct. It iterates all content layer dirs and merges profiles into a `profileMap` by ID. The package-level `LoadProfiles(profilesDir string)` in `profile.go` reads one directory — **do not touch that function**. Built-ins must be injected here (in the method), after the loop, so they are appended exactly once regardless of how many content layers are active.

The existing `TestLoadProfiles_ReadsAllYAML` test calls the package-level function directly — it will not be affected by this change and must continue to pass.

- [ ] **Step 1: Write the failing tests**

Add to `internal/content/profile_test.go`:

```go
func TestContentLoaderLoadProfiles_IncludesBuiltins(t *testing.T) {
	// A loader with one official dir that has one file-backed profile.
	dir := t.TempDir()
	profilesDir := filepath.Join(dir, "profiles")
	require.NoError(t, os.MkdirAll(profilesDir, 0755))
	writeFile(t, filepath.Join(profilesDir, "cap-developer.yaml"),
		"id: cap-developer\nname: CAP Developer\npacks:\n  - id: cap\n    weight: 100\n")

	loader := &content.ContentLoader{OfficialDir: dir}
	profiles, err := loader.LoadProfiles()
	require.NoError(t, err)
	// 1 file-backed + 2 built-ins = 3 total
	assert.Len(t, profiles, 3)
	ids := map[string]bool{}
	for _, p := range profiles {
		ids[p.ID] = true
	}
	assert.True(t, ids["all"])
	assert.True(t, ids["minimal"])
	assert.True(t, ids["cap-developer"])
}

func TestContentLoaderLoadProfiles_BuiltinWinsOverFile(t *testing.T) {
	// A file named all.yaml must be dropped; the built-in wins.
	dir := t.TempDir()
	profilesDir := filepath.Join(dir, "profiles")
	require.NoError(t, os.MkdirAll(profilesDir, 0755))
	writeFile(t, filepath.Join(profilesDir, "all.yaml"),
		"id: all\nname: CUSTOM ALL\ndescription: custom\n")

	loader := &content.ContentLoader{OfficialDir: dir}
	profiles, err := loader.LoadProfiles()
	require.NoError(t, err)

	var allProfile *content.Profile
	for _, p := range profiles {
		if p.ID == "all" {
			allProfile = p
		}
	}
	require.NotNil(t, allProfile)
	// Built-in name wins — file-backed "CUSTOM ALL" is dropped.
	assert.Equal(t, "All Packs", allProfile.Name)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/content/... -run "TestContentLoaderLoadProfiles" -v
```

Expected: FAIL — built-ins not yet injected.

- [ ] **Step 3: Implement — update `ContentLoader.LoadProfiles()` in `loader.go`**

Replace the existing `LoadProfiles` method body (lines 67–87). The current closing `return result, nil` becomes:

```go
// LoadProfiles loads profiles from all configured layers (later layers override).
// Built-in profiles (all, minimal) are always appended last and cannot be
// shadowed by file-backed profiles with the same ID.
func (cl *ContentLoader) LoadProfiles() ([]*Profile, error) {
	profileMap := make(map[string]*Profile)
	for _, dir := range cl.activeDirs() {
		profilesDir := filepath.Join(dir, "profiles")
		profiles, err := LoadProfiles(profilesDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, p := range profiles {
			profileMap[p.ID] = p
		}
	}
	// Drop any file-backed profile whose ID is reserved for a built-in.
	result := make([]*Profile, 0, len(profileMap))
	for _, p := range profileMap {
		if !reservedProfileIDs[p.ID] {
			result = append(result, p)
		}
	}
	// Append built-ins last so file-backed profiles appear first in list output.
	return append(result, BuiltinProfiles()...), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/content/... -run "TestContentLoaderLoadProfiles" -v
```

Expected: PASS (2 new tests). Confirm the existing `TestLoadProfiles_ReadsAllYAML` still passes:

```bash
go test ./internal/content/... -run "TestLoadProfiles_ReadsAllYAML" -v
```

Expected: PASS.

- [ ] **Step 5: Build check**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/content/loader.go internal/content/profile_test.go
git commit -m "feat(content): inject built-in profiles in ContentLoader.LoadProfiles"
```

---

### Task 3: Short-circuit `ContentLoader.FindProfile()` for built-ins

**Files:**
- Modify: `internal/content/loader.go:90-101`
- Test: `internal/content/profile_test.go`

**Context:** `FindProfile(id string)` (lines 90–101 in `loader.go`) currently calls `cl.LoadProfiles()` and iterates the result. For built-in IDs this works but is unnecessary: we know the answer without reading any files. The short-circuit avoids I/O and makes the lookup O(1) for built-ins.

- [ ] **Step 1: Write the failing tests**

Add to `internal/content/profile_test.go`:

```go
func TestContentLoaderFindProfile_ReturnsBuiltinForAll(t *testing.T) {
	// No files anywhere — built-in must be found regardless.
	loader := &content.ContentLoader{OfficialDir: t.TempDir()}
	p, err := loader.FindProfile("all")
	require.NoError(t, err)
	require.NotNil(t, p, "FindProfile('all') must return non-nil")
	assert.Equal(t, "all", p.ID)
}

func TestContentLoaderFindProfile_ReturnsBuiltinForMinimal(t *testing.T) {
	loader := &content.ContentLoader{OfficialDir: t.TempDir()}
	p, err := loader.FindProfile("minimal")
	require.NoError(t, err)
	require.NotNil(t, p, "FindProfile('minimal') must return non-nil")
	assert.Equal(t, "minimal", p.ID)
}
```

- [ ] **Step 2: Run tests to confirm current behaviour**

```bash
go test ./internal/content/... -run "TestContentLoaderFindProfile_ReturnsBuiltin" -v
```

After Task 2, `ContentLoader.LoadProfiles()` already appends built-ins, so `FindProfile` finds them via that path — these tests may already PASS. Run to confirm, then proceed to Task 3 Step 3 regardless: the short-circuit is a correctness and performance improvement (eliminates unnecessary file I/O for reserved IDs) and must be implemented.

- [ ] **Step 3: Implement — update `ContentLoader.FindProfile()` in `loader.go`**

Replace the method (lines 90–101) with:

```go
// FindProfile returns a profile by ID from all layers, or nil if not found.
// Built-in profile IDs (all, minimal) are returned directly without file I/O.
func (cl *ContentLoader) FindProfile(id string) (*Profile, error) {
	if reservedProfileIDs[id] {
		for _, p := range BuiltinProfiles() {
			if p.ID == id {
				return p, nil
			}
		}
	}
	profiles, err := cl.LoadProfiles()
	if err != nil {
		return nil, err
	}
	for _, p := range profiles {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}
```

- [ ] **Step 4: Run all content tests**

```bash
go test ./internal/content/... -v
```

Expected: all tests PASS (no regressions).

- [ ] **Step 5: Build check**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/content/loader.go internal/content/profile_test.go
git commit -m "feat(content): short-circuit FindProfile for built-in profile IDs"
```

---

### Task 4: `LoadPacks` returns base packs only for `minimal`

**Files:**
- Modify: `internal/content/loader.go:50-63`
- Test: `internal/content/loader_test.go`

**Context:** `LoadPacks()` already pins base packs first via a partition loop (added in the base layer feature). The `minimal` guard is inserted after that partition — two lines before the final `return`. The `all` profile needs no code change: it already returns every pack (base + non-base) in weight order.

- [ ] **Step 1: Write the failing tests**

Add to `internal/content/loader_test.go`:

```go
func TestContentLoader_LoadPacks_MinimalProfile_BasePacksOnly(t *testing.T) {
	dir := makeTempPacksDir(t, map[string]string{
		"base":      "id: base\nname: Base\nweight: 0\nbase: true\n",
		"cap":       "id: cap\nname: CAP\nweight: 100\n",
	})
	loader := &content.ContentLoader{OfficialDir: dir}
	minimal := &content.Profile{ID: "minimal", Name: "Minimal"}
	packs, err := loader.LoadPacks(minimal, "")
	require.NoError(t, err)
	require.Len(t, packs, 1, "minimal profile must return only base packs")
	assert.Equal(t, "base", packs[0].ID)
}

func TestContentLoader_LoadPacks_AllProfile_AllPacksReturned(t *testing.T) {
	dir := makeTempPacksDir(t, map[string]string{
		"base": "id: base\nname: Base\nweight: 0\nbase: true\n",
		"cap":  "id: cap\nname: CAP\nweight: 100\n",
	})
	loader := &content.ContentLoader{OfficialDir: dir}
	all := &content.Profile{ID: "all", Name: "All Packs"}
	packs, err := loader.LoadPacks(all, "")
	require.NoError(t, err)
	require.Len(t, packs, 2, "all profile must return all packs")
	// base pack must still be first
	assert.Equal(t, "base", packs[0].ID)
	assert.Equal(t, "cap", packs[1].ID)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/content/... -run "TestContentLoader_LoadPacks_MinimalProfile|TestContentLoader_LoadPacks_AllProfile" -v
```

Expected: `TestContentLoader_LoadPacks_MinimalProfile_BasePacksOnly` FAIL (returns 2 packs, not 1). `TestContentLoader_LoadPacks_AllProfile_AllPacksReturned` may already pass.

- [ ] **Step 3: Implement — add `minimal` guard in `LoadPacks()`**

In `loader.go`, find the end of `LoadPacks()`. The current closing lines (after the partition loop) are:

```go
	return append(base, nonBase...), nil
```

Replace with:

```go
	// minimal profile: base packs only — no technology-specific content.
	if profile != nil && profile.ID == "minimal" {
		return base, nil
	}
	return append(base, nonBase...), nil
```

- [ ] **Step 4: Run all content tests**

```bash
go test ./internal/content/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Build check**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/content/loader.go internal/content/loader_test.go
git commit -m "feat(content): LoadPacks returns base packs only for minimal profile"
```

---

### Task 5: i18n keys and `profile show` built-in display

**Files:**
- Modify: `internal/i18n/catalogs/en.json`
- Modify: `internal/i18n/catalogs/de.json`
- Modify: `cmd/profile.go`
- Create: `cmd/profile_test.go`

**Context:** `profile show` currently prints a "Pack weights:" header followed by `p.Packs`. For built-in profiles, `p.Packs` is nil, leaving an orphaned header. The fix: extract the display portion of `profileShowCmd.RunE` into a package-level helper `renderProfileShow(out io.Writer, p *content.Profile, lang string)` — this makes it directly unit-testable from `cmd/profile_test.go` (which must use `package cmd` to access the unexported function).

The guard checks `content.IsBuiltinProfile(p.ID)` — robust against any file-backed profile that happens to have empty packs.

- [ ] **Step 1: Add i18n keys**

In `internal/i18n/catalogs/en.json`, add after the `"profile.show.pack_weights"` line:

```json
  "profile.show.builtin_note": "Built-in profile — pack selection is determined at runtime, not by a fixed list.",
```

In `internal/i18n/catalogs/de.json`, add after `"profile.show.pack_weights"`:

```json
  "profile.show.builtin_note": "Integriertes Profil — die Pack-Auswahl wird zur Laufzeit bestimmt, nicht durch eine feste Liste.",
```

- [ ] **Step 2: Write the failing test**

Create `cmd/profile_test.go`:

```go
package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestRenderProfileShow_BuiltinProfile_PrintsBuiltinNote(t *testing.T) {
	var buf bytes.Buffer
	p := &content.Profile{
		ID:          "all",
		Name:        "All Packs",
		Description: "All available packs across every content layer",
	}
	renderProfileShow(&buf, p, "en")
	out := buf.String()
	if !strings.Contains(out, "Built-in profile") {
		t.Errorf("expected 'Built-in profile' in output, got: %q", out)
	}
	if strings.Contains(out, "Pack weights") {
		t.Errorf("expected no 'Pack weights' header for built-in profile, got: %q", out)
	}
}

func TestRenderProfileShow_FileBacked_PrintsPackWeights(t *testing.T) {
	var buf bytes.Buffer
	p := &content.Profile{
		ID:          "cap-developer",
		Name:        "CAP Developer",
		Description: "Building cloud-native apps",
		Packs: []content.PackWeight{
			{ID: "cap", Weight: 100},
		},
	}
	renderProfileShow(&buf, p, "en")
	out := buf.String()
	if !strings.Contains(out, "Pack weights") {
		t.Errorf("expected 'Pack weights' header for file-backed profile, got: %q", out)
	}
	if strings.Contains(out, "Built-in profile") {
		t.Errorf("unexpected 'Built-in profile' text for file-backed profile, got: %q", out)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go build ./cmd/... 2>&1
```

Expected: compile error — `renderProfileShow` is undefined.

- [ ] **Step 4: Implement — extract `renderProfileShow` and update `profile show`**

In `cmd/profile.go`, add the helper function (place it before `init()`):

```go
// renderProfileShow writes the profile display to out.
// Extracted for unit testing.
func renderProfileShow(out io.Writer, p *content.Profile, lang string) {
	fmt.Fprint(out, i18n.Tf(lang, "profile.show.header", map[string]any{"Name": p.Name, "Description": p.Description}))
	if content.IsBuiltinProfile(p.ID) {
		fmt.Fprintln(out, i18n.T(lang, "profile.show.builtin_note"))
	} else {
		fmt.Fprintln(out, i18n.T(lang, "profile.show.pack_weights"))
		for _, pw := range p.Packs {
			fmt.Fprintf(out, "  %-20s %d\n", pw.ID, pw.Weight)
		}
	}
}
```

Add the `io` import to the import block in `cmd/profile.go` (add `"io"` alongside the existing imports).

Add the `content` import: `"github.com/SAP-samples/sap-devs-cli/internal/content"`.

In `profileShowCmd.RunE`, replace the existing display block:

```go
		fmt.Fprint(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "profile.show.header", map[string]any{"Name": p.Name, "Description": p.Description}))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "profile.show.pack_weights"))
		for _, pw := range p.Packs {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %d\n", pw.ID, pw.Weight)
		}
```

with:

```go
		renderProfileShow(cmd.OutOrStdout(), p, i18n.ActiveLang)
```

- [ ] **Step 5: Build and verify tests pass**

```bash
go build ./...
go vet ./...
```

Expected: no errors.

```bash
go test ./cmd/... -run "TestRenderProfileShow" -v
```

Expected: PASS (2 tests). Note: `go test ./cmd/...` may be blocked by Windows Defender on `.config` paths — if so, confirm via CI or run `go build ./...` + `go vet ./...` locally.

- [ ] **Step 6: Commit**

```bash
git add internal/i18n/catalogs/en.json internal/i18n/catalogs/de.json cmd/profile.go cmd/profile_test.go
git commit -m "feat(cmd): profile show prints built-in note for all/minimal profiles"
```

---

### Task 6: Documentation

**Files:**
- Modify: `docs/content/content-guide.md`
- Modify: `docs/content-authoring.md`

**No tests.** Verify formatting only.

**Context:**
- `docs/content/content-guide.md` — the Profiles section ends at line ~213 with `ApplyWeights()` description, followed by `---` and "Creating a New Pack". Add a "Built-in Profiles" subsection between those two.
- `docs/content-authoring.md` — the Base Layer section (lines 33–61) ends with the authoring contract. Add one sentence about `minimal`.

- [ ] **Step 1: Add Built-in Profiles subsection to `docs/content/content-guide.md`**

Insert after `ApplyWeights()` reorders packs..." line and before the `---` separator:

```markdown

### Built-in Profiles

Two profiles are built into the CLI and require no YAML file on disk:

| Profile | Behaviour |
| --- | --- |
| `all` | Includes every pack from every content layer, sorted by pack weight. Use for development or when working across multiple SAP domains. |
| `minimal` | Includes base packs only — no technology-specific content. Use for cost-conscious setups or AI tools with tight token budgets. |

Both profiles appear in `sap-devs profile list` and can be set with `sap-devs profile set all` or `sap-devs profile set minimal`.

**Reserved IDs:** The IDs `all` and `minimal` are reserved. Any file named `all.yaml` or `minimal.yaml` in a content layer is silently ignored — the built-in definition always takes precedence.
```

- [ ] **Step 2: Add `minimal` note to `docs/content-authoring.md`**

In the Base Layer section, append to the authoring contract paragraph (after "regardless of their configured token budget"):

```markdown

> **`minimal` profile and base packs:** The `minimal` built-in profile includes base packs only. Keeping base pack content lean is therefore a direct budget lever for users who select `minimal` — every extra byte in a base pack is added to the `minimal` profile footprint.
```

- [ ] **Step 3: Build check (catches any broken imports)**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add docs/content/content-guide.md docs/content-authoring.md
git commit -m "docs(content): document built-in all and minimal profiles"
```

---

## Completion Checklist

- [ ] `go build ./...` — clean
- [ ] `go vet ./...` — clean
- [ ] All new tests pass (CI on `ubuntu-latest` is authoritative — `go test` is blocked by Windows Defender locally)
- [ ] `sap-devs profile list` shows `all` and `minimal` alongside file-backed profiles
- [ ] `sap-devs profile set all` and `sap-devs profile set minimal` succeed
- [ ] `sap-devs profile show` with active profile `all` or `minimal` prints built-in note, not pack weights header
- [ ] `SAP_DEVS_DEV=1 go run . inject --dry-run` with `minimal` profile injects base pack only
