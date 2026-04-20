# Additive Content Layers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow higher content layers (company, user, project) to mark a pack as `additive: true` so it augments an official pack with the same ID rather than replacing it.

**Architecture:** Add `Additive` and `AdditivePosition` fields to the `Pack` struct and `packMeta` YAML parser. Add a `MergeWith` method on `*Pack` plus three concrete helpers (`mergeResources`, `mergeTools`, `mergeMCPServers`) in a new `merge.go` file. Modify a single line in `loader.go` to invoke `MergeWith` when an additive pack is encountered. Add YAML schemas for all five content file types and document the feature in `docs/content-authoring.md`.

**Tech Stack:** Go 1.21+, `testify` (assert/require), JSON Schema Draft 7, YAML Language Server (VS Code Red Hat YAML extension)

**Build verification:** `go test` is blocked by Windows Defender locally. Use `go build ./... && go vet ./...` to verify correctness locally. CI (ubuntu-latest GitHub Actions) is the authoritative test runner.

---

## File Map

| Action | File | Responsibility |
| --- | --- | --- |
| Modify | `internal/content/pack.go` | Add `Additive`/`AdditivePosition` to `packMeta` and `Pack`; promote in `LoadPack` |
| Create | `internal/content/merge.go` | `MergeWith`, `unionStrings`, `mergeResources`, `mergeTools`, `mergeMCPServers` |
| Create | `internal/content/export_test.go` | Thin exported wrappers for external test access (compiled only during `go test`) |
| Create | `internal/content/merge_test.go` | Unit tests for all merge helpers and `MergeWith` |
| Modify | `internal/content/loader.go` | Additive branch in `LoadPacks` |
| Modify | `internal/content/loader_test.go` | 3-layer integration test |
| Create | `content/schemas/pack.schema.json` | JSON Schema for pack.yaml (includes new additive fields) |
| Create | `content/schemas/resources.schema.json` | JSON Schema for resources.yaml |
| Create | `content/schemas/tools.schema.json` | JSON Schema for tools.yaml |
| Create | `content/schemas/mcp.schema.json` | JSON Schema for mcp.yaml |
| Create | `content/schemas/profile.schema.json` | JSON Schema for profiles/*.yaml |
| Create | `.vscode/settings.json` | Wire YAML schemas to file globs |
| Modify | `docs/content-authoring.md` | VS Code setup note, pack.yaml field table rows, Additive Layers section |

---

## Task 1: Add `Additive` and `AdditivePosition` to `Pack` and `packMeta`

**Files:**
- Modify: `internal/content/pack.go`
- Test: `internal/content/pack_test.go`

- [ ] **Step 1: Write failing test for new fields loading from pack.yaml**

Add to `internal/content/pack_test.go`:

```go
func TestLoadPack_AdditiveFields(t *testing.T) {
    dir := t.TempDir()
    yaml := "id: cap\nname: CAP\ndescription: desc\ntags: []\nprofiles: []\nweight: 0\nadditive: true\nadditive_position: before\n"
    require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

    pack, err := content.LoadPack(dir, "")
    require.NoError(t, err)
    assert.True(t, pack.Additive)
    assert.Equal(t, "before", pack.AdditivePosition)
}

func TestLoadPack_AdditiveDefaults(t *testing.T) {
    dir := t.TempDir()
    yaml := "id: cap\nname: CAP\ndescription: desc\ntags: []\nprofiles: []\nweight: 0\n"
    require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

    pack, err := content.LoadPack(dir, "")
    require.NoError(t, err)
    assert.False(t, pack.Additive)
    assert.Equal(t, "", pack.AdditivePosition)
}
```

- [ ] **Step 2: Run build to confirm tests are wired (expect compile error)**

```bash
cd d:/projects/sap-devs-cli && go build ./...
```

Expected: **compile error** — `pack.Additive` and `pack.AdditivePosition` do not exist yet on the `Pack` struct. This is correct — the tests are red. Proceed to Step 3 to add the fields.

- [ ] **Step 3: Add fields to `packMeta` in `internal/content/pack.go`**

In the `packMeta` struct (after the `Base` field, around line 93):

```go
Additive         bool   `yaml:"additive,omitempty"`
AdditivePosition string `yaml:"additive_position,omitempty"`
```

- [ ] **Step 4: Add fields to `Pack` struct in `internal/content/pack.go`**

In the `Pack` struct (after the `Base` field, around line 20):

```go
Additive         bool
AdditivePosition string // "before" | "after"; normalised to "after" if empty
```

- [ ] **Step 5: Promote fields in `LoadPack` and normalise `AdditivePosition`**

In `LoadPack`, after `Base: meta.Base,` (around line 118):

```go
Additive:         meta.Additive,
AdditivePosition: meta.AdditivePosition,
```

Then after the pack struct literal closes, add normalisation — guarded so non-additive packs keep `AdditivePosition == ""`:

```go
if pack.Additive && pack.AdditivePosition != "before" {
    pack.AdditivePosition = "after"
}
```

- [ ] **Step 6: Build and vet**

```bash
cd d:/projects/sap-devs-cli && go build ./... && go vet ./...
```

Expected: no errors

- [ ] **Step 7: Commit**

```bash
cd d:/projects/sap-devs-cli && git add internal/content/pack.go internal/content/pack_test.go
git commit -m "feat(content): add Additive and AdditivePosition fields to Pack"
```

---

## Task 2: Implement merge helpers in `merge.go`

**Files:**
- Create: `internal/content/merge.go`
- Create: `internal/content/merge_test.go`

- [ ] **Step 1: Write failing tests for helpers**

Create `internal/content/merge_test.go`:

```go
package content_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestUnionStrings_DeduplicatesAndPreservesOrder(t *testing.T) {
    got := content.UnionStrings([]string{"a", "b"}, []string{"b", "c"})
    assert.Equal(t, []string{"a", "b", "c"}, got)
}

func TestUnionStrings_BothEmpty(t *testing.T) {
    assert.Equal(t, []string{}, content.UnionStrings(nil, nil))
}

func TestUnionStrings_OnlyA(t *testing.T) {
    assert.Equal(t, []string{"a"}, content.UnionStrings([]string{"a"}, nil))
}

func TestMergeResources_ReplacesOnMatchingID(t *testing.T) {
    base := []content.Resource{
        {ID: "cap/docs", Title: "CAP Docs", URL: "https://old.example", PackID: "cap"},
        {ID: "cap/community", Title: "Community", URL: "https://community.sap.com", PackID: "cap"},
    }
    additive := []content.Resource{
        {ID: "cap/docs", Title: "CAP Docs Updated", URL: "https://new.example", PackID: "company-cap"},
    }
    got := content.MergeResources(base, additive, "cap")
    assert.Len(t, got, 2)
    // Replaced entry uses additive values
    assert.Equal(t, "CAP Docs Updated", got[0].Title)
    assert.Equal(t, "https://new.example", got[0].URL)
    // PackID re-stamped to base ID
    assert.Equal(t, "cap", got[0].PackID)
    // Unmatched base entry preserved
    assert.Equal(t, "cap/community", got[1].ID)
}

func TestMergeResources_AppendsNewIDs(t *testing.T) {
    base := []content.Resource{{ID: "cap/docs", Title: "Docs", PackID: "cap"}}
    additive := []content.Resource{{ID: "cap/new", Title: "New Resource", PackID: "company"}}
    got := content.MergeResources(base, additive, "cap")
    assert.Len(t, got, 2)
    assert.Equal(t, "cap/new", got[1].ID)
    assert.Equal(t, "cap", got[1].PackID)
}

func TestMergeResources_FreshSlice_NoAliasing(t *testing.T) {
    base := []content.Resource{{ID: "cap/docs", PackID: "cap"}}
    got := content.MergeResources(base, nil, "cap")
    got[0].Title = "mutated"
    assert.Empty(t, base[0].Title, "mutation must not affect original base slice")
}

func TestMergeTools_ReplacesOnMatchingID(t *testing.T) {
    base := []content.ToolDef{{ID: "nodejs", Name: "Node.js", Required: ">=18.0.0"}}
    additive := []content.ToolDef{{ID: "nodejs", Name: "Node.js", Required: ">=20.0.0"}}
    got := content.MergeTools(base, additive)
    assert.Len(t, got, 1)
    assert.Equal(t, ">=20.0.0", got[0].Required)
}

func TestMergeTools_AppendsNewIDs(t *testing.T) {
    base := []content.ToolDef{{ID: "nodejs"}}
    additive := []content.ToolDef{{ID: "bun"}}
    got := content.MergeTools(base, additive)
    assert.Len(t, got, 2)
}

func TestMergeMCPServers_ReplacesOnMatchingIDAndRestampsPackID(t *testing.T) {
    base := []content.MCPServer{{ID: "cap-mcp", Name: "Old", PackID: "cap"}}
    additive := []content.MCPServer{{ID: "cap-mcp", Name: "New", PackID: "company"}}
    got := content.MergeMCPServers(base, additive, "cap")
    assert.Len(t, got, 1)
    assert.Equal(t, "New", got[0].Name)
    assert.Equal(t, "cap", got[0].PackID)
}
```

- [ ] **Step 2: Run build to confirm production code still builds**

```bash
cd d:/projects/sap-devs-cli && go build ./...
```

Expected: **no errors** — `go build` does not compile `_test.go` files, so the new tests in `merge_test.go` are not verified here. Test compilation is checked in CI via `go test`. Proceed to Step 3 to create the helpers.

- [ ] **Step 3: Create `internal/content/merge.go` with all helpers**

```go
package content

// unionStrings returns a fresh deduplicated slice: all elements of a,
// then elements of b not already present in a. Order is preserved.
func unionStrings(a, b []string) []string {
    seen := make(map[string]bool, len(a))
    result := make([]string, 0, len(a)+len(b))
    for _, s := range a {
        if !seen[s] {
            seen[s] = true
            result = append(result, s)
        }
    }
    for _, s := range b {
        if !seen[s] {
            seen[s] = true
            result = append(result, s)
        }
    }
    return result
}

// mergeResources builds a fresh []Resource: starts with base entries, replaces
// any entry whose ID matches an additive entry, appends unmatched additive entries.
// PackID is re-stamped to packID on every entry in the result.
func mergeResources(base, additive []Resource, packID string) []Resource {
    result := make([]Resource, len(base))
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

// mergeTools builds a fresh []ToolDef: starts with base entries, replaces
// any entry whose ID matches an additive entry, appends unmatched additive entries.
func mergeTools(base, additive []ToolDef) []ToolDef {
    result := make([]ToolDef, len(base))
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
    return result
}

// mergeMCPServers builds a fresh []MCPServer: starts with base entries, replaces
// any entry whose ID matches an additive entry, appends unmatched additive entries.
// PackID is re-stamped to packID on every entry in the result.
func mergeMCPServers(base, additive []MCPServer, packID string) []MCPServer {
    result := make([]MCPServer, len(base))
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

Note: helpers are unexported (lowercase). The test file uses `package content_test` and calls exported wrappers. These wrappers live in `internal/content/export_test.go` — a standard Go pattern that compiles only during `go test`, so they never appear in the released binary. Create `internal/content/export_test.go`:

```go
package content

// Test-only exports — compiled only during go test; not part of the production binary.
func UnionStrings(a, b []string) []string                        { return unionStrings(a, b) }
func MergeResources(base, add []Resource, id string) []Resource  { return mergeResources(base, add, id) }
func MergeTools(base, add []ToolDef) []ToolDef                   { return mergeTools(base, add) }
func MergeMCPServers(base, add []MCPServer, id string) []MCPServer { return mergeMCPServers(base, add, id) }
```

Note the package declaration is `package content` (not `package content_test`) — `export_test.go` is special: it belongs to the internal package for symbol access, but is only built during testing.

- [ ] **Step 4: Build and vet**

```bash
cd d:/projects/sap-devs-cli && go build ./... && go vet ./...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
cd d:/projects/sap-devs-cli && git add internal/content/merge.go internal/content/export_test.go internal/content/merge_test.go
git commit -m "feat(content): add merge helpers for additive content layers"
```

---

## Task 3: Implement `Pack.MergeWith`

**Files:**
- Modify: `internal/content/merge.go` (add method)
- Modify: `internal/content/merge_test.go` (add MergeWith tests)

- [ ] **Step 1: Write failing tests for `MergeWith`**

Add to `internal/content/merge_test.go`:

```go
func makePack(id, name, context string, tips []content.Tip, resources []content.Resource) *content.Pack {
    return &content.Pack{
        ID:        id,
        Name:      name,
        ContextMD: context,
        Tips:      tips,
        Resources: resources,
        Tags:      []string{"base-tag"},
        Profiles:  []string{"cap-developer"},
        Overlaps:  []string{},
    }
}

func TestMergeWith_GuardReturnBaseWhenNotAdditive(t *testing.T) {
    base := makePack("cap", "CAP Official", "base context", nil, nil)
    notAdditive := &content.Pack{ID: "cap", Name: "Override", Additive: false}
    result := notAdditive.MergeWith(base)
    assert.Equal(t, base, result, "non-additive MergeWith must return base unchanged")
}

func TestMergeWith_ContextAfter(t *testing.T) {
    base := makePack("cap", "CAP", "base context", nil, nil)
    additive := &content.Pack{ID: "cap", ContextMD: "extra context", Additive: true, AdditivePosition: "after"}
    result := additive.MergeWith(base)
    assert.Equal(t, "base context\n\nextra context", result.ContextMD)
}

func TestMergeWith_ContextBefore(t *testing.T) {
    base := makePack("cap", "CAP", "base context", nil, nil)
    additive := &content.Pack{ID: "cap", ContextMD: "extra context", Additive: true, AdditivePosition: "before"}
    result := additive.MergeWith(base)
    assert.Equal(t, "extra context\n\nbase context", result.ContextMD)
}

func TestMergeWith_EmptyContextPreservesBase(t *testing.T) {
    base := makePack("cap", "CAP", "base context", nil, nil)
    additive := &content.Pack{ID: "cap", ContextMD: "", Additive: true, AdditivePosition: "after"}
    result := additive.MergeWith(base)
    assert.Equal(t, "base context", result.ContextMD)
}

func TestMergeWith_TipsAfter(t *testing.T) {
    baseTips := []content.Tip{{Title: "Base Tip"}}
    addTips := []content.Tip{{Title: "Additive Tip"}}
    base := makePack("cap", "CAP", "", baseTips, nil)
    additive := &content.Pack{ID: "cap", Tips: addTips, Additive: true, AdditivePosition: "after"}
    result := additive.MergeWith(base)
    require.Len(t, result.Tips, 2)
    assert.Equal(t, "Base Tip", result.Tips[0].Title)
    assert.Equal(t, "Additive Tip", result.Tips[1].Title)
}

func TestMergeWith_TipsBefore(t *testing.T) {
    baseTips := []content.Tip{{Title: "Base Tip"}}
    addTips := []content.Tip{{Title: "Additive Tip"}}
    base := makePack("cap", "CAP", "", baseTips, nil)
    additive := &content.Pack{ID: "cap", Tips: addTips, Additive: true, AdditivePosition: "before"}
    result := additive.MergeWith(base)
    require.Len(t, result.Tips, 2)
    assert.Equal(t, "Additive Tip", result.Tips[0].Title)
    assert.Equal(t, "Base Tip", result.Tips[1].Title)
}

func TestMergeWith_TipsNoAliasing(t *testing.T) {
    baseTips := []content.Tip{{Title: "Base Tip"}}
    base := makePack("cap", "CAP", "", baseTips, nil)
    additive := &content.Pack{ID: "cap", Additive: true, AdditivePosition: "after"}
    result := additive.MergeWith(base)
    result.Tips[0].Title = "mutated"
    assert.Equal(t, "Base Tip", base.Tips[0].Title, "mutation must not affect base tips")
}

func TestMergeWith_MetadataOverrideOnNonEmpty(t *testing.T) {
    base := makePack("cap", "CAP Official", "", nil, nil)
    base.Description = "Official description"
    base.Weight = 100
    additive := &content.Pack{
        ID: "cap", Name: "CAP Company", Description: "Company description",
        Weight: 150, Tags: []string{"extra"}, Additive: true, AdditivePosition: "after",
    }
    result := additive.MergeWith(base)
    assert.Equal(t, "CAP Company", result.Name)
    assert.Equal(t, "Company description", result.Description)
    assert.Equal(t, 150, result.Weight)
    assert.Contains(t, result.Tags, "base-tag")
    assert.Contains(t, result.Tags, "extra")
}

func TestMergeWith_MetadataEmptyFieldsPreserveBase(t *testing.T) {
    base := makePack("cap", "CAP Official", "", nil, nil)
    base.Description = "Official description"
    base.Weight = 100
    additive := &content.Pack{ID: "cap", Name: "", Description: "", Weight: 0, Additive: true, AdditivePosition: "after"}
    result := additive.MergeWith(base)
    assert.Equal(t, "CAP Official", result.Name)
    assert.Equal(t, "Official description", result.Description)
    assert.Equal(t, 100, result.Weight)
}

func TestMergeWith_ProfilesAndOverlapsTakenFromBase(t *testing.T) {
    base := makePack("cap", "CAP", "", nil, nil)
    base.Profiles = []string{"cap-developer"}
    base.Overlaps = []string{"btp-core"}
    additive := &content.Pack{
        ID: "cap", Additive: true, AdditivePosition: "after",
        Profiles: []string{"company-profile"}, Overlaps: []string{"other"},
    }
    result := additive.MergeWith(base)
    assert.Equal(t, []string{"cap-developer"}, result.Profiles)
    assert.Equal(t, []string{"btp-core"}, result.Overlaps)
}

func TestMergeWith_ProfilesNoAliasing(t *testing.T) {
    base := makePack("cap", "CAP", "", nil, nil)
    base.Profiles = []string{"cap-developer"}
    additive := &content.Pack{ID: "cap", Additive: true, AdditivePosition: "after"}
    result := additive.MergeWith(base)
    result.Profiles[0] = "mutated"
    assert.Equal(t, "cap-developer", base.Profiles[0])
}

func TestMergeWith_AdditiveIsFalseOnResult(t *testing.T) {
    base := makePack("cap", "CAP", "", nil, nil)
    additive := &content.Pack{ID: "cap", Additive: true, AdditivePosition: "after"}
    result := additive.MergeWith(base)
    assert.False(t, result.Additive)
}
```

- [ ] **Step 2: Build to confirm production code still builds**

```bash
cd d:/projects/sap-devs-cli && go build ./...
```

Expected: no errors — `go build` does not compile `_test.go` files; test wiring is verified in CI. Proceed to Step 3 to add `MergeWith`.

- [ ] **Step 3: Add `MergeWith` method to `merge.go`**

Add above the exported wrappers:

```go
// MergeWith returns a new *Pack that augments base with the content of a.
// If a.Additive is false, MergeWith is a no-op and returns base unchanged.
func (a *Pack) MergeWith(base *Pack) *Pack {
    if !a.Additive {
        return base
    }
    merged := *base // shallow copy of scalar fields; slices replaced below

    // Metadata: override on non-empty
    if a.Name != "" {
        merged.Name = a.Name
    }
    if a.Description != "" {
        merged.Description = a.Description
    }
    if a.Weight != 0 {
        merged.Weight = a.Weight
    }
    merged.Tags = unionStrings(base.Tags, a.Tags)
    // Profiles and Overlaps always come from base; produce fresh slices to avoid aliasing.
    merged.Profiles = append([]string(nil), base.Profiles...)
    merged.Overlaps = append([]string(nil), base.Overlaps...)

    // Context: position controls order. Empty additive ContextMD preserves base.
    if a.ContextMD != "" {
        if a.AdditivePosition == "before" {
            merged.ContextMD = a.ContextMD + "\n\n" + base.ContextMD
        } else {
            merged.ContextMD = base.ContextMD + "\n\n" + a.ContextMD
        }
    }

    // Tips: both kept; position controls order. Always fresh slice.
    if a.AdditivePosition == "before" {
        merged.Tips = append(append([]Tip(nil), a.Tips...), base.Tips...)
    } else {
        merged.Tips = append(append([]Tip(nil), base.Tips...), a.Tips...)
    }

    // Structured lists: additive replaces on matching ID, appends new.
    // PackID re-stamped to base pack's ID on Resources and MCPServers.
    merged.Resources = mergeResources(base.Resources, a.Resources, base.ID)
    merged.Tools = mergeTools(base.Tools, a.Tools)
    merged.MCPServers = mergeMCPServers(base.MCPServers, a.MCPServers, base.ID)

    merged.Additive = false
    return &merged
}
```

- [ ] **Step 4: Build and vet**

```bash
cd d:/projects/sap-devs-cli && go build ./... && go vet ./...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
cd d:/projects/sap-devs-cli && git add internal/content/merge.go internal/content/merge_test.go
git commit -m "feat(content): implement Pack.MergeWith for additive layer merging"
```

---

## Task 4: Wire additive merge into `LoadPacks`

**Files:**
- Modify: `internal/content/loader.go`
- Modify: `internal/content/loader_test.go`

- [ ] **Step 1: Write failing integration test**

Add to `internal/content/loader_test.go`:

```go
func TestContentLoader_LoadPacks_AdditiveLayer(t *testing.T) {
    // Official layer: cap pack with one tip and one resource
    official := makeLayerDir(t, map[string]packFixture{
        "cap": {
            yaml:     "id: cap\nname: CAP Official\ndescription: Official\ntags: [official]\nweight: 100\n",
            context:  "Official context",
            tips:     "## Official Tip\nOfficial tip content",
            resources: "- id: cap/docs\n  title: Official Docs\n  url: https://official.example\n  type: official-docs\n  tags: []\n",
        },
    })
    // Company layer: additive pack for cap — adds a tip and a resource
    company := makeLayerDir(t, map[string]packFixture{
        "cap": {
            yaml:     "id: cap\nname: \ndescription: \ntags: [company]\nweight: 0\nadditive: true\nadditive_position: after\n",
            context:  "Company context",
            tips:     "## Company Tip\nCompany tip content",
            resources: "- id: cap/company-guide\n  title: Company Guide\n  url: https://company.example\n  type: official-docs\n  tags: []\n",
        },
    })
    // Project layer: another additive pack — adds one more tip
    project := makeLayerDir(t, map[string]packFixture{
        "cap": {
            yaml: "id: cap\nname: CAP Project\ndescription: \ntags: []\nweight: 0\nadditive: true\nadditive_position: after\n",
            tips: "## Project Tip\nProject tip content",
        },
    })

    loader := &content.ContentLoader{
        OfficialDir: official,
        CompanyDir:  company,
        ProjectDir:  project,
    }
    packs, err := loader.LoadPacks(nil, "")
    require.NoError(t, err)

    cap := findPack(packs, "cap")
    require.NotNil(t, cap)

    // Name: company layer had empty name — base name preserved; project overrides
    assert.Equal(t, "CAP Project", cap.Name)

    // Context: official + company appended (after), then project has no context
    assert.Equal(t, "Official context\n\nCompany context", cap.ContextMD)

    // Tips: all three present in order (official, company, project)
    require.Len(t, cap.Tips, 3)
    assert.Equal(t, "Official Tip", cap.Tips[0].Title)
    assert.Equal(t, "Company Tip", cap.Tips[1].Title)
    assert.Equal(t, "Project Tip", cap.Tips[2].Title)

    // Resources: official + company appended
    require.Len(t, cap.Resources, 2)
    assert.Equal(t, "cap", cap.Resources[0].PackID)
    assert.Equal(t, "cap", cap.Resources[1].PackID)

    // Tags: union
    assert.Contains(t, cap.Tags, "official")
    assert.Contains(t, cap.Tags, "company")
}

func TestContentLoader_LoadPacks_AdditiveNoBase(t *testing.T) {
    // Additive pack with no matching base — becomes the base as-is, Additive cleared
    company := makeLayerDir(t, map[string]packFixture{
        "new-pack": {
            yaml: "id: new-pack\nname: New Pack\ndescription: desc\ntags: []\nweight: 50\nadditive: true\nadditive_position: after\n",
        },
    })
    loader := &content.ContentLoader{OfficialDir: t.TempDir(), CompanyDir: company}
    packs, err := loader.LoadPacks(nil, "")
    require.NoError(t, err)
    p := findPack(packs, "new-pack")
    require.NotNil(t, p)
    assert.False(t, p.Additive, "Additive flag must be cleared in no-base path")
}

// packFixture holds optional file content for makeLayerDir.
type packFixture struct {
    yaml      string
    context   string
    tips      string
    resources string
}

func makeLayerDir(t *testing.T, packs map[string]packFixture) string {
    t.Helper()
    root := t.TempDir()
    packsDir := filepath.Join(root, "packs")
    require.NoError(t, os.MkdirAll(packsDir, 0755))
    for id, f := range packs {
        dir := filepath.Join(packsDir, id)
        require.NoError(t, os.MkdirAll(dir, 0755))
        require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(f.yaml), 0644))
        if f.context != "" {
            require.NoError(t, os.WriteFile(filepath.Join(dir, "context.md"), []byte(f.context), 0644))
        }
        if f.tips != "" {
            require.NoError(t, os.WriteFile(filepath.Join(dir, "tips.md"), []byte(f.tips), 0644))
        }
        if f.resources != "" {
            require.NoError(t, os.WriteFile(filepath.Join(dir, "resources.yaml"), []byte(f.resources), 0644))
        }
    }
    return root
}
```

- [ ] **Step 2: Build to confirm test compiles**

```bash
cd d:/projects/sap-devs-cli && go build ./...
```

Expected: compiles — test will fail at runtime until loader is updated

- [ ] **Step 3: Update the merge line in `LoadPacks` (`internal/content/loader.go`)**

Replace line 39 (`packMap[pack.ID] = pack // later layers override`) with:

```go
if pack.Additive {
    if existing, ok := packMap[pack.ID]; ok {
        packMap[pack.ID] = pack.MergeWith(existing)
    } else {
        // No base found — treat additive pack as the base, but clear Additive flag.
        pack.Additive = false
        packMap[pack.ID] = pack
    }
} else {
    packMap[pack.ID] = pack // existing replace behaviour unchanged
}
```

- [ ] **Step 4: Build and vet**

```bash
cd d:/projects/sap-devs-cli && go build ./... && go vet ./...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
cd d:/projects/sap-devs-cli && git add internal/content/loader.go internal/content/loader_test.go
git commit -m "feat(content): apply additive merge in LoadPacks"
```

---

## Task 5: YAML Schemas

**Files:**
- Create: `content/schemas/pack.schema.json`
- Create: `content/schemas/resources.schema.json`
- Create: `content/schemas/tools.schema.json`
- Create: `content/schemas/mcp.schema.json`
- Create: `content/schemas/profile.schema.json`
- Create: `.vscode/settings.json`

No tests for JSON Schema files — correctness is verified by opening a pack YAML in VS Code and confirming autocomplete and validation appear.

- [ ] **Step 1: Create `content/schemas/pack.schema.json`**

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Pack Metadata",
  "description": "Schema for sap-devs pack.yaml files",
  "type": "object",
  "required": ["id", "name", "description", "tags"],
  "additionalProperties": false,
  "properties": {
    "id": {
      "type": "string",
      "description": "Unique pack identifier (lowercase slug, e.g. cap, abap, btp-core)"
    },
    "name": {
      "type": "string",
      "description": "Human-readable pack name"
    },
    "description": {
      "type": "string",
      "description": "Short pack description"
    },
    "tags": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Searchable tags for this pack"
    },
    "profiles": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Profile IDs that include this pack (informational)"
    },
    "weight": {
      "type": "integer",
      "default": 0,
      "description": "Default sort priority; higher values appear earlier in context"
    },
    "base": {
      "type": "boolean",
      "default": false,
      "description": "If true, this pack is injected into every profile and always appears first"
    },
    "overlaps": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Pack IDs whose content this pack subsumes (used for byte-budget deduplication)"
    },
    "locales": {
      "type": "object",
      "description": "Locale-specific overrides for name and description",
      "additionalProperties": {
        "type": "object",
        "additionalProperties": false,
        "properties": {
          "name": { "type": "string" },
          "description": { "type": "string" }
        }
      }
    },
    "additive": {
      "type": "boolean",
      "default": false,
      "description": "If true, this pack augments (rather than replaces) the lower-layer pack with the same ID"
    },
    "additive_position": {
      "type": "string",
      "enum": ["before", "after"],
      "default": "after",
      "description": "Where additive content is placed relative to the base pack's content"
    }
  },
  "if": {
    "properties": { "additive": { "const": true } }
  },
  "then": {
    "properties": {
      "additive_position": { "enum": ["before", "after"] }
    }
  },
  "else": {
    "not": { "required": ["additive_position"] }
  }
}
```

- [ ] **Step 2: Create `content/schemas/resources.schema.json`**

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Pack Resources",
  "description": "Schema for sap-devs resources.yaml files (top-level array)",
  "type": "array",
  "items": {
    "type": "object",
    "required": ["id", "title", "url", "type", "tags"],
    "additionalProperties": false,
    "properties": {
      "id": {
        "type": "string",
        "description": "Resource identifier in format <pack-id>/<slug>, e.g. cap/docs-official"
      },
      "title": { "type": "string", "description": "Human-readable resource title" },
      "url": { "type": "string", "format": "uri", "description": "Full URL to the resource" },
      "type": {
        "type": "string",
        "enum": ["official-docs", "sample", "community", "tutorial", "blog"],
        "description": "Resource classification"
      },
      "tags": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Searchable tags"
      },
      "advocate": {
        "type": "string",
        "description": "Optional contributor or advocate name"
      }
    }
  }
}
```

- [ ] **Step 3: Create `content/schemas/tools.schema.json`**

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Pack Tools",
  "description": "Schema for sap-devs tools.yaml files (top-level array)",
  "type": "array",
  "items": {
    "type": "object",
    "required": ["id", "name", "required", "detect", "install", "docs"],
    "additionalProperties": false,
    "properties": {
      "id": { "type": "string", "description": "Unique tool identifier, e.g. nodejs, cds-dk" },
      "name": { "type": "string", "description": "Human-readable tool name" },
      "required": {
        "type": "string",
        "description": "Semver constraint, e.g. >=18.0.0 or latest"
      },
      "detect": {
        "type": "object",
        "required": ["command", "pattern"],
        "additionalProperties": false,
        "properties": {
          "command": { "type": "string", "description": "Shell command to detect installed version" },
          "pattern": { "type": "string", "description": "Regex with one capture group to extract version string" }
        }
      },
      "install": {
        "type": "object",
        "additionalProperties": false,
        "properties": {
          "windows": { "type": "string" },
          "macos": { "type": "string" },
          "linux": { "type": "string" },
          "all": { "type": "string" }
        },
        "description": "Platform-specific install commands"
      },
      "docs": { "type": "string", "format": "uri", "description": "URL to tool documentation" }
    }
  }
}
```

- [ ] **Step 4: Create `content/schemas/mcp.schema.json`**

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Pack MCP Servers",
  "description": "Schema for sap-devs mcp.yaml files (top-level array)",
  "type": "array",
  "items": {
    "type": "object",
    "required": ["id", "name", "description", "install", "hosts"],
    "additionalProperties": false,
    "properties": {
      "id": { "type": "string", "description": "Unique MCP server identifier within the pack" },
      "name": { "type": "string", "description": "Human-readable MCP server name" },
      "description": { "type": "string", "description": "Short description shown by sap-devs mcp list" },
      "install": {
        "type": "object",
        "required": ["command", "args"],
        "additionalProperties": false,
        "properties": {
          "command": { "type": "string", "description": "Executable to run, e.g. npx" },
          "args": {
            "type": "array",
            "items": { "type": "string" },
            "description": "Arguments passed to the command"
          }
        }
      },
      "hosts": {
        "type": "array",
        "items": { "type": "string" },
        "description": "AI tool hosts that should register this server, e.g. [claude-code, cursor]"
      }
    }
  }
}
```

- [ ] **Step 5: Create `content/schemas/profile.schema.json`**

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Developer Profile",
  "description": "Schema for sap-devs profile YAML files",
  "type": "object",
  "required": ["id", "name", "description", "packs"],
  "additionalProperties": false,
  "properties": {
    "id": {
      "type": "string",
      "description": "Unique profile slug, e.g. cap-developer. Reserved: all, minimal"
    },
    "name": { "type": "string", "description": "Human-readable profile name" },
    "description": { "type": "string", "description": "Profile description" },
    "packs": {
      "type": "array",
      "description": "Ordered list of pack IDs and their weights for this profile",
      "items": {
        "type": "object",
        "required": ["id", "weight"],
        "additionalProperties": false,
        "properties": {
          "id": { "type": "string", "description": "Pack ID" },
          "weight": { "type": "integer", "description": "Sort priority within this profile; higher = earlier" }
        }
      }
    },
    "tip_tags": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Preferred tip tags for this profile (used by sap-devs tip)"
    }
  }
}
```

- [ ] **Step 6: Create `.vscode/settings.json`**

```json
{
  "yaml.schemas": {
    "./content/schemas/pack.schema.json":      "**/packs/*/pack.yaml",
    "./content/schemas/resources.schema.json": "**/packs/*/resources.yaml",
    "./content/schemas/tools.schema.json":     "**/packs/*/tools.yaml",
    "./content/schemas/mcp.schema.json":       "**/packs/*/mcp.yaml",
    "./content/schemas/profile.schema.json":   "**/profiles/*.yaml"
  }
}
```

- [ ] **Step 7: Build to confirm no impact on Go compilation**

```bash
cd d:/projects/sap-devs-cli && go build ./... && go vet ./...
```

Expected: no errors (JSON/settings files are not compiled)

- [ ] **Step 8: Commit**

```bash
cd d:/projects/sap-devs-cli && git add content/schemas/ .vscode/settings.json
git commit -m "feat(content): add YAML schemas and VS Code editor wiring"
```

---

## Task 6: Update `docs/content-authoring.md`

**Files:**
- Modify: `docs/content-authoring.md`

No automated tests — verify by reading the rendered output.

- [ ] **Step 1: Add VS Code setup note near the top**

After the introductory paragraph (before the "Pack Directory Structure" section), insert:

```markdown
## Editor Setup

For inline validation and autocomplete when editing pack YAML files, install the [YAML extension by Red Hat](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) in VS Code. Schema wiring is already configured in `.vscode/settings.json` — open any `pack.yaml`, `resources.yaml`, `tools.yaml`, `mcp.yaml`, or profile YAML file and you'll get field suggestions and error highlighting automatically.
```

- [ ] **Step 2: Add `additive` and `additive_position` field notes to the `pack.yaml` reference**

Open `docs/content/content-guide.md`. Locate the `### pack.yaml` section (around line 38). The existing fields are documented as `> **Note:**` paragraphs. Add two new notes after the `base` note (which ends around line 60):

```markdown
> **Note:** **`additive`** *(optional bool, default `false`)* — when `true`, this pack augments the lower-layer pack with the same `id` rather than replacing it. Only valid in company, user, or project layers. See [Additive Layers](../content-authoring.md#additive-layers).

> **Note:** **`additive_position`** *(optional string `"before"` | `"after"`, default `"after"`)* — controls where additive content is inserted relative to the base pack's content. Only meaningful when `additive: true`.
```

- [ ] **Step 3: Add Additive Layers section at the end of the doc**

Append the following section:

````markdown
---

## Additive Layers

By default, a pack in a higher layer (company, user, project) with the same `id` as an official pack **replaces** it entirely. This is fine when you want to fully customise a pack, but it means you must copy and maintain the whole official pack just to add a few tips or resources.

**Additive mode** lets you augment a lower-layer pack without copying it. Set `additive: true` in `pack.yaml` — your pack's content is merged on top of the official pack's content.

### When to use additive mode

- You want to add company-specific tips to an official pack without copying its context or tools
- You want to add internal resource links to an official pack's `resources.yaml`
- You want to update a tool's required version in your project without maintaining the full `tools.yaml`

### Position

`additive_position: after` (default) — your content appears after the official content.
`additive_position: before` — your content appears before the official content.

Use `before` for high-priority notes (e.g., "company policy requires X") that should precede the official guidance.

### Merge behaviour

| File | What happens |
| --- | --- |
| `context.md` | Your content is appended or prepended to the official context |
| `tips.md` | Both sets of tips are kept; yours are added in the configured position |
| `resources.yaml` | Entries with matching `id` replace the official entry; new IDs are appended |
| `tools.yaml` | Entries with matching `id` replace the official entry; new IDs are appended |
| `mcp.yaml` | Entries with matching `id` replace the official entry; new IDs are appended |
| `pack.yaml` metadata | `name`/`description` override if non-empty; `weight` overrides if non-zero; `tags` union-merged; `profiles`/`base`/`overlaps` always come from the official pack |

### No-base fallback

If no lower-layer pack with the same `id` exists, the additive pack is treated as the base pack. This lets you write additive packs defensively — they work correctly whether or not the official pack is present.

### Example: company additions to the CAP pack

```
.sap-devs/packs/cap/
├── pack.yaml       # additive: true
├── tips.md         # company-specific CAP tips
└── resources.yaml  # internal CAP reference links
```

`pack.yaml`:
```yaml
id: cap
name: ""            # empty — base name preserved
description: ""     # empty — base description preserved
tags: [internal]
weight: 0
additive: true
additive_position: after
```

`tips.md`:
```markdown
## Internal CAP Deployment Guide
Tags: cap,internal
Use our internal pipeline at https://pipeline.example.com/cap to deploy CAP apps to BTP.

## Company HANA Cloud Instance
Tags: cap,hana
Connect to the shared HANA Cloud instance at hana.internal for dev/test. See the wiki for credentials.
```

`resources.yaml`:
```yaml
- id: cap/internal-pipeline
  title: Internal CAP Deployment Pipeline
  url: https://pipeline.example.com/cap
  type: official-docs
  tags: [deployment, internal]
```

The final injected context will contain all official CAP content plus your company tips and resources.

### Limitations

- **Tips cannot be replaced by title.** Tips have no stable `id` field; additive tips are always appended or prepended. To replace an official tip, use a full replace-mode pack (omit `additive: true`).
- **`additive_position` applies globally** to the whole pack — you cannot mix before/after positions for different content types in the same pack.
- **Do not set `base: true`** in an additive pack. In merge mode, `base` is always taken from the official pack; in no-base mode it would make your pack inject into every profile, which is rarely what you want.
````

- [ ] **Step 4: Build to confirm no Go impact**

```bash
cd d:/projects/sap-devs-cli && go build ./... && go vet ./...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
cd d:/projects/sap-devs-cli && git add docs/content-authoring.md
git commit -m "docs(content): document additive layers feature and editor schema setup"
```

---

## Final Verification

- [ ] **Full build and vet**

```bash
cd d:/projects/sap-devs-cli && go build ./... && go vet ./...
```

Expected: no errors

- [ ] **Verify existing tests still compile** (CI will run them)

```bash
cd d:/projects/sap-devs-cli && go test -run TestLoadPack_ParsesAllFiles -count=1 ./internal/content/... 2>&1 | head -5
```

Note: `go test` may be blocked by Windows Defender — a compile error here is a real failure; a Defender block is expected and tests will pass in CI.

- [ ] **Open a pack.yaml in VS Code and confirm schema validation appears**

Open `content/packs/cap/pack.yaml`. You should see:
- Hover tooltips on fields like `id`, `name`, `weight`
- An error/warning if you add a typo field like `nme:`
- Autocomplete suggestions when typing `add` (should suggest `additive`)
