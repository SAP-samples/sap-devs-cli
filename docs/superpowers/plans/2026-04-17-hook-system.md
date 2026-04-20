# Hook System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a hook system that lets packs declare CLI commands to wire into AI tool lifecycle events (e.g. `sap-devs tip --markdown` on Claude Code `SessionStart`), with `sap-devs hook list/install/uninstall/status` managing installation.

**Architecture:** Mirrors the existing `mcp` system exactly. `hook.yaml` per pack → `HookDef` struct loaded by `LoadPack` → `FlattenHooks`/`FindHookDef` helpers → `WriteHookConfig`/`RemoveHookConfig`/`HookConfigInstalled` in `internal/adapter/hook_wire.go` → `cmd/hook.go`. `HookConfig` field added to `Adapter` struct; `claude-code.yaml` gets `hook_config` block. New `--markdown` and `--plain` flags on `sap-devs tip` produce hook-friendly output.

**Tech Stack:** Go 1.21+, `github.com/stretchr/testify`, `encoding/json`, cobra, YAML content files.

**Spec:** `docs/superpowers/specs/2026-04-17-hook-system-design.md`

**Worktree:** `d:/projects/sap-devs-cli/.worktrees/feat-hook-system` (branch `feat/hook-system`)

**Known deviations from `mcp` pattern (intentional):**
- `cmd/hook.go` uses plain string literals instead of `i18n.T()`. i18n wiring is deferred — the hook command is new and adding catalog keys up-front before strings stabilize adds churn. Add i18n in a follow-up once strings are settled.
- `hook install` (no args) installs all hooks for the active profile without requiring an explicit `--all` flag. This simplifies the common case. Unlike `mcp install`, there is no ambiguity — hooks have no required argument that makes the "no args = invalid" guard necessary.

---

## File Map

| File | Action | Responsibility |
| ---- | ------ | -------------- |
| `cmd/tip.go` | Modify | Add `--markdown` and `--plain` flags |
| `internal/content/pack.go` | Modify | Add `HookDef` struct, `Hooks []HookDef` field, load `hook.yaml` in `LoadPack` |
| `internal/content/hook.go` | Create | `FlattenHooks` and `FindHookDef` helpers |
| `internal/content/pack_test.go` | Modify | Add `hook.yaml` load tests |
| `internal/content/hook_test.go` | Create | Tests for `FlattenHooks` and `FindHookDef` |
| `internal/adapter/adapter.go` | Modify | Add `HookConfig` struct, `HookConfig *HookConfig` field to `Adapter` |
| `internal/adapter/hook_wire.go` | Create | `WriteHookConfig`, `RemoveHookConfig`, `HookConfigInstalled` |
| `internal/adapter/hook_wire_test.go` | Create | Tests for all three functions |
| `cmd/hook.go` | Create | `hook list/install/uninstall/status` command |
| `content/packs/base/hook.yaml` | Create | `tip-on-session-start` hook entry |
| `content/adapters/claude-code.yaml` | Modify | Add `hook_config` block |
| `content/schemas/hook.schema.json` | Create | JSON Schema for `hook.yaml` |
| `.vscode/settings.json` | Modify | Wire `hook.schema.json` to `**/packs/*/hook.yaml` |
| `docs/content-authoring.md` | Modify | Directory tree + new Hook Authoring section |
| `CLAUDE.md` | Modify | Add `hook` row to CLI commands table |

---

## Task 1: Add `--markdown` and `--plain` flags to `sap-devs tip`

**Files:**
- Modify: `cmd/tip.go`

- [ ] **Step 1: Write the failing tests**

Add a test file `cmd/tip_format_test.go` that calls the exported `FormatTip` function we are about to add. This fails to compile until `FormatTip` is exported.

```go
package cmd_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/SAP-samples/sap-devs-cli/cmd"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestFormatTip_Markdown(t *testing.T) {
	tip := content.Tip{Title: "Use cds watch", Content: "Run cds watch for live reload."}
	out := cmd.FormatTip(tip, true, false)
	assert.True(t, strings.HasPrefix(out, "## 💡 Use cds watch"), "must start with ## 💡")
	assert.NotContains(t, out, "\x1b[", "must have no ANSI escape sequences")
}

func TestFormatTip_Plain(t *testing.T) {
	tip := content.Tip{Title: "Use cds watch", Content: "Run cds watch for live reload."}
	out := cmd.FormatTip(tip, false, true)
	assert.False(t, strings.HasPrefix(out, "#"), "must not start with a heading")
	assert.NotContains(t, out, "\x1b[", "must have no ANSI escape sequences")
	assert.Contains(t, out, "Use cds watch")
}

func TestFormatTip_DefaultReturnsEmpty(t *testing.T) {
	tip := content.Tip{Title: "Use cds watch", Content: "Run cds watch for live reload."}
	out := cmd.FormatTip(tip, false, false)
	assert.Empty(t, out, "default format returns empty string (caller uses glamour)")
}
```

> Note: `FormatTip` must be exported (capital F) so the `_test` package can call it. The test is in `package cmd_test` which can only access exported symbols from `package cmd`.

- [ ] **Step 2: Verify tests fail to compile**

```bash
cd d:/projects/sap-devs-cli/.worktrees/feat-hook-system
go build ./... 2>&1 | head -10
```

Expected: compile error — `cmd_test.FormatTip` undefined.

- [ ] **Step 3: Add the flags and exported format helper to `cmd/tip.go`**

At the top of `cmd/tip.go`, add two package-level flag vars after the imports:

```go
var tipMarkdown bool
var tipPlain bool
```

Add an **exported** helper function after the `tipCmd` var (exported so `cmd_test` package can call it directly):

```go
// FormatTip formats a tip for non-interactive output. Returns empty string for
// the default case (caller uses glamour rendering instead).
func FormatTip(tip content.Tip, markdown, plain bool) string {
	if markdown {
		return fmt.Sprintf("## 💡 %s\n\n%s\n", tip.Title, tip.Content)
	}
	if plain {
		return fmt.Sprintf("%s\n\n%s\n", tip.Title, tip.Content)
	}
	return ""
}
```

Replace the rendering block inside `tipCmd.RunE` (the block starting `md := fmt.Sprintf(...)` through `fmt.Print(rendered)`) with:

```go
		if tipMarkdown || tipPlain {
			fmt.Fprint(cmd.OutOrStdout(), FormatTip(tip, tipMarkdown, tipPlain))
			return nil
		}
		md := fmt.Sprintf("## 💡 %s\n\n%s\n", tip.Title, tip.Content)
		rendered, err := glamour.Render(md, "dark")
		if err != nil {
			fmt.Printf("💡 %s\n\n%s\n", tip.Title, tip.Content)
			return nil
		}
		fmt.Print(rendered)
```

In the `init()` function of `cmd/tip.go`, add before `tipCmd.AddCommand(...)`:

```go
	tipCmd.Flags().BoolVar(&tipMarkdown, "markdown", false, "output raw Markdown (no ANSI rendering)")
	tipCmd.Flags().BoolVar(&tipPlain, "plain", false, "output plain text (no Markdown or ANSI)")
```

- [ ] **Step 4: Build to verify compilation**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Smoke-test manually**

```bash
SAP_DEVS_DEV=1 go run . tip --markdown 2>&1 | head -5
SAP_DEVS_DEV=1 go run . tip --plain 2>&1 | head -5
```

Expected: `--markdown` output starts with `## 💡`; `--plain` output starts with the tip title (no `#`).

- [ ] **Step 6: Build and vet**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
cd d:/projects/sap-devs-cli/.worktrees/feat-hook-system
git add cmd/tip.go cmd/tip_format_test.go
git commit -m "feat(tip): add --markdown and --plain output flags"
```

---

## Task 2: Add `HookDef` to `Pack` and load `hook.yaml` in `LoadPack`

**Files:**
- Modify: `internal/content/pack.go`
- Modify: `internal/content/pack_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/content/pack_test.go` after `TestLoadPack_AdditiveDefaults`:

```go
func TestLoadPack_HooksLoadedWhenPresent(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: base\nname: Base\ndescription: Base pack\ntags: []\nprofiles: []\nweight: 0\nbase: true\n"
	hookYAML := `- id: tip-on-session-start
  event: sessionStart
  command: "sap-devs tip --markdown"
  tools:
    - claude-code
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hook.yaml"), []byte(hookYAML), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	require.Len(t, p.Hooks, 1)
	assert.Equal(t, "tip-on-session-start", p.Hooks[0].ID)
	assert.Equal(t, "sessionStart", p.Hooks[0].Event)
	assert.Equal(t, "sap-devs tip --markdown", p.Hooks[0].Command)
	assert.Equal(t, []string{"claude-code"}, p.Hooks[0].Tools)
	assert.Equal(t, "base", p.Hooks[0].PackID)
}

func TestLoadPack_HooksEmptyWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Empty(t, p.Hooks)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd d:/projects/sap-devs-cli/.worktrees/feat-hook-system
go build ./... 2>&1 | head -20
```

Expected: compile error — `p.Hooks` undefined.

- [ ] **Step 3: Add `HookDef` struct and `Hooks` field to `Pack`**

In `internal/content/pack.go`, add `HookDef` after the `Tip` struct (around line 77):

```go
// HookDef declares a hook command to wire into an AI tool's event system.
type HookDef struct {
	ID      string   `yaml:"id"`
	Event   string   `yaml:"event"`
	Command string   `yaml:"command"`
	Tools   []string `yaml:"tools"`
	PackID  string   // set at load time, not in YAML
}
```

In the `Pack` struct, add `Hooks []HookDef` after `PreambleMD string`:

```go
	PreambleMD string
	Hooks      []HookDef
```

- [ ] **Step 4: Load `hook.yaml` in `LoadPack`**

In `internal/content/pack.go`, add this block inside `LoadPack` after the preamble loading block (after the `pack.PreambleMD = string(data)` line):

```go
	if data, err := os.ReadFile(filepath.Join(packDir, "hook.yaml")); err == nil {
		_ = yaml.Unmarshal(data, &pack.Hooks)
		for i := range pack.Hooks {
			pack.Hooks[i].PackID = pack.ID
		}
	}
```

- [ ] **Step 5: Build to verify compilation**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Build and vet**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add internal/content/pack.go internal/content/pack_test.go
git commit -m "feat(content): add HookDef struct and load hook.yaml in LoadPack"
```

---

## Task 3: Add `FlattenHooks` and `FindHookDef` helpers

**Files:**
- Create: `internal/content/hook.go`
- Create: `internal/content/hook_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/content/hook_test.go`:

```go
package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestFlattenHooks_ReturnsAllHooks(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base", Hooks: []content.HookDef{
			{ID: "tip-on-session-start", Event: "sessionStart", Command: "sap-devs tip --markdown", Tools: []string{"claude-code"}, PackID: "base"},
		}},
		{ID: "cap", Hooks: []content.HookDef{
			{ID: "cap-hook", Event: "sessionStart", Command: "sap-devs tip --plain", Tools: []string{"cursor"}, PackID: "cap"},
		}},
	}
	hooks := content.FlattenHooks(packs)
	require.Len(t, hooks, 2)
	assert.Equal(t, "tip-on-session-start", hooks[0].ID)
	assert.Equal(t, "cap-hook", hooks[1].ID)
}

func TestFlattenHooks_EmptyPacks(t *testing.T) {
	hooks := content.FlattenHooks(nil)
	assert.Empty(t, hooks)
}

func TestFlattenHooks_PackWithNoHooks(t *testing.T) {
	packs := []*content.Pack{{ID: "base"}}
	hooks := content.FlattenHooks(packs)
	assert.Empty(t, hooks)
}

func TestFindHookDef_Found(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base", Hooks: []content.HookDef{
			{ID: "tip-on-session-start", Event: "sessionStart", PackID: "base"},
		}},
	}
	h := content.FindHookDef(packs, "tip-on-session-start")
	require.NotNil(t, h)
	assert.Equal(t, "tip-on-session-start", h.ID)
}

func TestFindHookDef_NotFound(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base", Hooks: []content.HookDef{
			{ID: "tip-on-session-start", Event: "sessionStart", PackID: "base"},
		}},
	}
	h := content.FindHookDef(packs, "nonexistent")
	assert.Nil(t, h)
}

func TestFindHookDef_NilPacks(t *testing.T) {
	h := content.FindHookDef(nil, "anything")
	assert.Nil(t, h)
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go build ./... 2>&1 | head -20
```

Expected: compile error — `content.FlattenHooks` and `content.FindHookDef` undefined.

- [ ] **Step 3: Create `internal/content/hook.go`**

```go
package content

// FlattenHooks returns all HookDef entries from all packs in order.
func FlattenHooks(packs []*Pack) []HookDef {
	var out []HookDef
	for _, p := range packs {
		out = append(out, p.Hooks...)
	}
	return out
}

// FindHookDef returns the first HookDef with the given ID across packs, or nil.
func FindHookDef(packs []*Pack, id string) *HookDef {
	for _, p := range packs {
		for i := range p.Hooks {
			if p.Hooks[i].ID == id {
				return &p.Hooks[i]
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Build and vet**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/content/hook.go internal/content/hook_test.go
git commit -m "feat(content): add FlattenHooks and FindHookDef helpers"
```

---

## Task 4: Add `HookConfig` to `Adapter` and `hook_wire.go`

**Files:**
- Modify: `internal/adapter/adapter.go`
- Create: `internal/adapter/hook_wire.go`
- Create: `internal/adapter/hook_wire_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/adapter/hook_wire_test.go`:

```go
package adapter_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/adapter"
)

func TestWriteHookConfig_CreatesFileWhenAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	err := adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false)
	require.NoError(t, err)
	installed, err := adapter.HookConfigInstalled(path, "hooks.SessionStart", "sap-devs tip --markdown")
	require.NoError(t, err)
	assert.True(t, installed)
}

func TestWriteHookConfig_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))
	require.NoError(t, adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))

	// Parse and count entries — must be exactly 1
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var root map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &root))
	hooks := root["hooks"].(map[string]interface{})
	entries := hooks["SessionStart"].([]interface{})
	assert.Len(t, entries, 1)
}

func TestWriteHookConfig_PreservesExistingKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	// Write a pre-existing key
	existing := `{"mcpServers":{"existing":{"command":"npx","args":[]}}}`
	require.NoError(t, os.WriteFile(path, []byte(existing), 0644))

	require.NoError(t, adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var root map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &root))
	assert.NotNil(t, root["mcpServers"], "existing key must be preserved")
	assert.NotNil(t, root["hooks"], "hooks key must be added")
}

func TestRemoveHookConfig_RemovesEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))

	require.NoError(t, adapter.RemoveHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))

	installed, err := adapter.HookConfigInstalled(path, "hooks.SessionStart", "sap-devs tip --markdown")
	require.NoError(t, err)
	assert.False(t, installed)
}

func TestRemoveHookConfig_CleansUpEmptyArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))
	require.NoError(t, adapter.RemoveHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))

	// Use HookConfigInstalled rather than manual JSON traversal — more stable
	installed, err := adapter.HookConfigInstalled(path, "hooks.SessionStart", "sap-devs tip --markdown")
	require.NoError(t, err)
	assert.False(t, installed, "command must not be present after removal")
}

func TestRemoveHookConfig_NoopWhenNotInstalled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	err := adapter.RemoveHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false)
	assert.NoError(t, err)
}

func TestHookConfigInstalled_TrueWhenInstalled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))
	installed, err := adapter.HookConfigInstalled(path, "hooks.SessionStart", "sap-devs tip --markdown")
	require.NoError(t, err)
	assert.True(t, installed)
}

func TestHookConfigInstalled_FalseWhenAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	installed, err := adapter.HookConfigInstalled(path, "hooks.SessionStart", "sap-devs tip --markdown")
	require.NoError(t, err)
	assert.False(t, installed)
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go build ./... 2>&1 | head -20
```

Expected: compile error — `adapter.WriteHookConfig` etc. undefined.

- [ ] **Step 3: Add `HookConfig` to `internal/adapter/adapter.go`**

After the `MCPConfig` struct, add:

```go
// HookConfig defines where to write hook command entries.
type HookConfig struct {
	Path   string `yaml:"path"`
	Format string `yaml:"format"` // "json" only for now
	Key    string `yaml:"key"`    // dot-separated JSON path, e.g. "hooks.SessionStart"
}
```

In the `Adapter` struct, add `HookConfig *HookConfig` after `MCPConfig *MCPConfig`:

```go
	MCPConfig  *MCPConfig  `yaml:"mcp_config,omitempty"`
	HookConfig *HookConfig `yaml:"hook_config,omitempty"`
```

- [ ] **Step 4: Create `internal/adapter/hook_wire.go`**

```go
package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteHookConfig adds a hook command entry to the tool's settings JSON.
// key is a dot-separated JSON path, e.g. "hooks.SessionStart".
// Idempotent: if the command is already present, it is a no-op.
// When dryRun is true, prints what would happen and returns without writing.
func WriteHookConfig(settingsPath, key, command string, dryRun bool) error {
	if dryRun {
		fmt.Printf("[dry-run] would add hook %q to %s[%s]\n", command, settingsPath, key)
		return nil
	}

	root, err := readJSONFile(settingsPath)
	if err != nil {
		return err
	}

	arr := navigateToArray(root, key)
	if hookCommandPresent(arr, command) {
		return nil // idempotent
	}

	entry := map[string]interface{}{
		"matcher": "",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": command,
			},
		},
	}
	arr = append(arr, entry)
	setNestedArray(root, key, arr)

	return writeJSONFile(settingsPath, root)
}

// RemoveHookConfig removes a hook command entry from the tool's settings JSON.
// No-op if the file does not exist or the entry is not present.
// When dryRun is true, prints what would happen and returns without writing.
func RemoveHookConfig(settingsPath, key, command string, dryRun bool) error {
	if dryRun {
		fmt.Printf("[dry-run] would remove hook %q from %s[%s]\n", command, settingsPath, key)
		return nil
	}

	data, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse %s: %w", settingsPath, err)
	}

	arr := navigateToArray(root, key)
	if len(arr) == 0 {
		return nil
	}

	var filtered []interface{}
	for _, item := range arr {
		if !entryMatchesCommand(item, command) {
			filtered = append(filtered, item)
		}
	}

	if len(filtered) == 0 {
		deleteNestedKey(root, key)
	} else {
		setNestedArray(root, key, filtered)
	}

	return writeJSONFile(settingsPath, root)
}

// HookConfigInstalled reports whether the command appears in the settings JSON.
func HookConfigInstalled(settingsPath, key, command string) (bool, error) {
	data, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("parse %s: %w", settingsPath, err)
	}
	return hookCommandPresent(navigateToArray(root, key), command), nil
}

// --- private helpers ---

func readJSONFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]interface{}), nil
	}
	if err != nil {
		return nil, err
	}
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return root, nil
}

func writeJSONFile(path string, root map[string]interface{}) error {
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// navigateToArray walks the dot-separated key path and returns the array at
// the leaf, creating intermediate objects as needed. Returns nil if not found.
func navigateToArray(root map[string]interface{}, key string) []interface{} {
	parts := strings.SplitN(key, ".", 2)
	head := parts[0]
	if len(parts) == 1 {
		// Leaf: return existing array or empty
		if v, ok := root[head]; ok {
			if arr, ok := v.([]interface{}); ok {
				return arr
			}
		}
		return nil
	}
	// Intermediate: descend into nested map
	sub, ok := root[head]
	if !ok || sub == nil {
		sub = make(map[string]interface{})
		root[head] = sub
	}
	m, ok := sub.(map[string]interface{})
	if !ok {
		return nil
	}
	return navigateToArray(m, parts[1])
}

// setNestedArray walks the dot-separated key path and sets the array at the leaf.
func setNestedArray(root map[string]interface{}, key string, arr []interface{}) {
	parts := strings.SplitN(key, ".", 2)
	head := parts[0]
	if len(parts) == 1 {
		root[head] = arr
		return
	}
	sub, ok := root[head]
	if !ok || sub == nil {
		sub = make(map[string]interface{})
		root[head] = sub
	}
	m, ok := sub.(map[string]interface{})
	if !ok {
		m = make(map[string]interface{})
		root[head] = m
	}
	setNestedArray(m, parts[1], arr)
}

// deleteNestedKey removes the leaf key at the dot-separated path.
func deleteNestedKey(root map[string]interface{}, key string) {
	parts := strings.SplitN(key, ".", 2)
	head := parts[0]
	if len(parts) == 1 {
		delete(root, head)
		return
	}
	sub, ok := root[head]
	if !ok {
		return
	}
	m, ok := sub.(map[string]interface{})
	if !ok {
		return
	}
	deleteNestedKey(m, parts[1])
	if len(m) == 0 {
		delete(root, head)
	}
}

// hookCommandPresent returns true if any entry in arr has a nested hooks[]
// entry whose command matches the given command string.
func hookCommandPresent(arr []interface{}, command string) bool {
	for _, item := range arr {
		if entryMatchesCommand(item, command) {
			return true
		}
	}
	return false
}

// entryMatchesCommand returns true if the matcher-group entry contains a
// hook with the given command.
func entryMatchesCommand(item interface{}, command string) bool {
	m, ok := item.(map[string]interface{})
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]interface{})
	if !ok {
		return false
	}
	for _, h := range hooks {
		hm, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		if hm["command"] == command {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Build and vet**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/adapter.go internal/adapter/hook_wire.go internal/adapter/hook_wire_test.go
git commit -m "feat(adapter): add HookConfig struct and hook_wire.go"
```

---

## Task 5: Create `cmd/hook.go`

**Files:**
- Create: `cmd/hook.go`

- [ ] **Step 1: Create `cmd/hook.go`**

```go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/adapter"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Manage AI tool lifecycle hooks from pack definitions",
}

// --- hook list ---

var hookListAll bool

var hookListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		var packs []*content.Pack
		if hookListAll {
			packs, err = loader.LoadPacks(nil, i18n.ActiveLang)
			if err != nil {
				return err
			}
		} else {
			paths, err2 := xdg.New()
			if err2 != nil {
				return err2
			}
			profileCfg, err2 := config.LoadProfile(paths.ConfigDir)
			if err2 != nil {
				return err2
			}
			if profileCfg.ID == "" {
				return fmt.Errorf("no active profile — run `sap-devs profile set`")
			}
			activeProfile, err2 := loader.FindProfile(profileCfg.ID)
			if err2 != nil {
				return err2
			}
			if activeProfile == nil {
				return fmt.Errorf("profile %q not found", profileCfg.ID)
			}
			packs, err = loader.LoadPacks(activeProfile, i18n.ActiveLang)
			if err != nil {
				return err
			}
		}
		hooks := content.FlattenHooks(packs)
		if len(hooks) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No hooks found.")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-16s %-32s %s\n", "ID", "PACK", "EVENT", "COMMAND", "TOOLS")
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 95))
		for _, h := range hooks {
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-16s %-32s %s\n",
				h.ID, h.PackID, h.Event, h.Command, strings.Join(h.Tools, ", "))
		}
		return nil
	},
}

// --- hook status ---

var hookStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show which hooks are installed in your AI tool configs",
	RunE: func(cmd *cobra.Command, args []string) error {
		allAdapters, err := loadAdapters()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		hooks := content.FlattenHooks(packs)
		if len(hooks) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No hooks found.")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-14s %s\n", "ID", "PACK", "ADAPTER", "STATUS")
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 65))

		for _, h := range hooks {
			for _, toolID := range h.Tools {
				a := findAdapterByID(allAdapters, toolID)
				if a == nil || a.HookConfig == nil {
					continue
				}
				path, err := adapter.ExpandHome(a.HookConfig.Path)
				if err != nil {
					continue
				}
				installed, err := adapter.HookConfigInstalled(path, a.HookConfig.Key, h.Command)
				status := "✗ not installed"
				if err == nil && installed {
					status = "✓ installed"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-10s %-14s %s\n", h.ID, h.PackID, toolID, status)
			}
		}
		return nil
	},
}

// --- hook install ---

var hookInstallDryRun bool

var hookInstallCmd = &cobra.Command{
	Use:   "install [id]",
	Short: "Wire a hook into your AI tool configs",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		allAdapters, err := loadAdapters()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		if len(args) == 1 {
			return hookInstallOne(loader, allAdapters, args[0])
		}
		return hookInstallAll(loader, allAdapters)
	},
}

func hookInstallOne(loader *content.ContentLoader, allAdapters []adapter.Adapter, id string) error {
	packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
	if err != nil {
		return err
	}
	h := content.FindHookDef(packs, id)
	if h == nil {
		return fmt.Errorf("hook %q not found", id)
	}
	toolSet := make(map[string]bool)
	for _, t := range h.Tools {
		toolSet[t] = true
	}
	detected := detectHookAdapters(allAdapters, toolSet)
	if len(detected) == 0 {
		return fmt.Errorf("no detected AI tools support hook %q (tools: %s)", id, strings.Join(h.Tools, ", "))
	}
	if len(detected) > 1 {
		fmt.Printf("Detected AI tools for hook %q:\n", id)
		for i, a := range detected {
			p, _ := adapter.ExpandHome(a.HookConfig.Path)
			fmt.Printf("  %d. %s  (%s)\n", i+1, a.Name, p)
		}
	}
	chosen, err := pickHookAdapters(detected)
	if err != nil {
		return err
	}
	for _, a := range chosen {
		path, err := adapter.ExpandHome(a.HookConfig.Path)
		if err != nil {
			return err
		}
		if err := adapter.WriteHookConfig(path, a.HookConfig.Key, h.Command, hookInstallDryRun); err != nil {
			return fmt.Errorf("install hook to %s: %w", a.Name, err)
		}
		if !hookInstallDryRun {
			fmt.Printf("✓ Registered hook %q in %s\n", h.ID, path)
		}
	}
	return nil
}

func hookInstallAll(loader *content.ContentLoader, allAdapters []adapter.Adapter) error {
	paths, err := xdg.New()
	if err != nil {
		return err
	}
	profileCfg, err := config.LoadProfile(paths.ConfigDir)
	if err != nil {
		return err
	}
	if profileCfg.ID == "" {
		return fmt.Errorf("no active profile — run `sap-devs profile set`")
	}
	activeProfile, err := loader.FindProfile(profileCfg.ID)
	if err != nil {
		return err
	}
	if activeProfile == nil {
		return fmt.Errorf("profile %q not found", profileCfg.ID)
	}
	packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
	if err != nil {
		return err
	}
	hooks := content.FlattenHooks(packs)
	if len(hooks) == 0 {
		fmt.Println("No hooks to install.")
		return nil
	}
	// Collect union of all tool IDs
	toolSet := make(map[string]bool)
	for _, h := range hooks {
		for _, t := range h.Tools {
			toolSet[t] = true
		}
	}
	detected := detectHookAdapters(allAdapters, toolSet)
	if len(detected) == 0 {
		return fmt.Errorf("no detected AI tools support these hooks")
	}
	fmt.Println("Detected AI tools:")
	for i, a := range detected {
		p, _ := adapter.ExpandHome(a.HookConfig.Path)
		fmt.Printf("  %d. %s  (%s)\n", i+1, a.Name, p)
	}
	chosen, err := pickHookAdapters(detected)
	if err != nil {
		return err
	}
	installed := 0
	for _, h := range hooks {
		for _, a := range chosen {
			if !containsString(h.Tools, a.ID) {
				continue
			}
			path, err := adapter.ExpandHome(a.HookConfig.Path)
			if err != nil {
				return err
			}
			if err := adapter.WriteHookConfig(path, a.HookConfig.Key, h.Command, hookInstallDryRun); err != nil {
				return fmt.Errorf("install hook %s to %s: %w", h.ID, a.Name, err)
			}
			if !hookInstallDryRun {
				fmt.Printf("✓ Registered hook %q in %s\n", h.ID, path)
				installed++
			}
		}
	}
	if !hookInstallDryRun {
		fmt.Printf("Installed %d hook(s) into %d tool(s).\n", installed, len(chosen))
	}
	return nil
}

// --- hook uninstall ---

var hookUninstallCmd = &cobra.Command{
	Use:   "uninstall [id]",
	Short: "Remove a hook from your AI tool configs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		allAdapters, err := loadAdapters()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		h := content.FindHookDef(packs, args[0])
		if h == nil {
			return fmt.Errorf("hook %q not found", args[0])
		}
		for _, toolID := range h.Tools {
			a := findAdapterByID(allAdapters, toolID)
			if a == nil || a.HookConfig == nil {
				continue
			}
			path, err := adapter.ExpandHome(a.HookConfig.Path)
			if err != nil {
				return err
			}
			if err := adapter.RemoveHookConfig(path, a.HookConfig.Key, h.Command, false); err != nil {
				return fmt.Errorf("uninstall hook from %s: %w", a.Name, err)
			}
			fmt.Printf("✓ Removed hook %q from %s\n", h.ID, path)
		}
		return nil
	},
}

// --- shared helpers ---

// detectHookAdapters returns adapters that have hook_config, whose IDs are in
// toolSet, and are detected as installed on this machine.
func detectHookAdapters(adapters []adapter.Adapter, toolSet map[string]bool) []adapter.Adapter {
	var out []adapter.Adapter
	for _, a := range adapters {
		if a.HookConfig == nil {
			continue
		}
		if toolSet != nil && !toolSet[a.ID] {
			continue
		}
		if adapter.Detect(a) {
			out = append(out, a)
		}
	}
	return out
}

// pickHookAdapters prompts the user to select adapters; single adapter skips the prompt.
func pickHookAdapters(adapters []adapter.Adapter) ([]adapter.Adapter, error) {
	if len(adapters) == 1 {
		return adapters, nil
	}
	fmt.Print("Install into (comma-separated numbers or 'all'): ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(line)
	if strings.ToLower(line) == "all" {
		return adapters, nil
	}
	var chosen []adapter.Adapter
	for _, part := range strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == ' ' }) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > len(adapters) {
			return nil, fmt.Errorf("invalid selection: %q", part)
		}
		chosen = append(chosen, adapters[n-1])
	}
	if len(chosen) == 0 {
		return nil, fmt.Errorf("no adapters selected")
	}
	return chosen, nil
}

// findAdapterByID returns the adapter with the given ID, or nil.
func findAdapterByID(adapters []adapter.Adapter, id string) *adapter.Adapter {
	for i := range adapters {
		if adapters[i].ID == id {
			return &adapters[i]
		}
	}
	return nil
}

func init() {
	hookListCmd.Flags().BoolVar(&hookListAll, "all", false, "list hooks from all packs (default: active profile only)")
	hookInstallCmd.Flags().BoolVar(&hookInstallDryRun, "dry-run", false, "preview without writing config files")
	hookCmd.AddCommand(hookListCmd, hookInstallCmd, hookUninstallCmd, hookStatusCmd)
	rootCmd.AddCommand(hookCmd)
}
```

- [ ] **Step 2: Build and vet**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 3: Smoke-test**

```bash
SAP_DEVS_DEV=1 go run . hook --help
SAP_DEVS_DEV=1 go run . hook list --all
```

Expected: help text shows subcommands; `hook list --all` prints the table header (may show no rows if content not synced — that's fine in dev mode).

- [ ] **Step 4: Commit**

```bash
git add cmd/hook.go
git commit -m "feat(cmd): add hook list/install/uninstall/status command"
```

---

## Task 6: Add content files — `hook.yaml` and adapter `hook_config`

**Files:**
- Create: `content/packs/base/hook.yaml`
- Modify: `content/adapters/claude-code.yaml`

- [ ] **Step 1: Create `content/packs/base/hook.yaml`**

```yaml
- id: tip-on-session-start
  event: sessionStart
  command: "sap-devs tip --markdown"
  tools:
    - claude-code
```

- [ ] **Step 2: Add `hook_config` to `content/adapters/claude-code.yaml`**

Add after the `mcp_config` block:

```yaml
hook_config:
  path: "~/.claude/settings.json"
  format: json
  key: "hooks.SessionStart"
```

- [ ] **Step 3: Verify the hook is visible**

```bash
SAP_DEVS_DEV=1 go run . hook list --all
```

Expected output:
```
ID                             PACK       EVENT            COMMAND                          TOOLS
-----------------------------------------------------------------------------------------------
tip-on-session-start           base       sessionStart     sap-devs tip --markdown          claude-code
```

- [ ] **Step 4: Dry-run install**

```bash
SAP_DEVS_DEV=1 go run . hook install --dry-run
```

Expected: prints `[dry-run] would add hook "sap-devs tip --markdown" to ~/.claude/settings.json[hooks.SessionStart]` (no file written).

- [ ] **Step 5: Build and vet**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add content/packs/base/hook.yaml content/adapters/claude-code.yaml
git commit -m "content(base): add tip-on-session-start hook; wire hook_config in claude-code adapter"
```

---

## Task 7: Add JSON Schema and VS Code wiring

**Files:**
- Create: `content/schemas/hook.schema.json`
- Modify: `.vscode/settings.json`

- [ ] **Step 1: Create `content/schemas/hook.schema.json`**

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Pack Hook Definitions",
  "description": "Schema for sap-devs hook.yaml files (top-level array)",
  "type": "array",
  "items": {
    "type": "object",
    "required": ["id", "event", "command", "tools"],
    "additionalProperties": false,
    "properties": {
      "id": { "type": "string", "description": "Unique hook identifier within the pack" },
      "event": {
        "type": "string",
        "enum": ["sessionStart"],
        "description": "Tool-neutral event name"
      },
      "command": { "type": "string", "description": "Shell command to run when the event fires" },
      "tools": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Adapter IDs this hook applies to, e.g. [claude-code]"
      }
    }
  }
}
```

- [ ] **Step 2: Add schema wiring to `.vscode/settings.json`**

In `.vscode/settings.json`, add `hook.yaml` association inside the `yaml.schemas` object:

```json
"./content/schemas/hook.schema.json":       "**/packs/*/hook.yaml"
```

The file should look like:

```json
{
  "yaml.schemas": {
    "./content/schemas/pack.schema.json":      "**/packs/*/pack.yaml",
    "./content/schemas/resources.schema.json": "**/packs/*/resources.yaml",
    "./content/schemas/tools.schema.json":     "**/packs/*/tools.yaml",
    "./content/schemas/mcp.schema.json":       "**/packs/*/mcp.yaml",
    "./content/schemas/hook.schema.json":      "**/packs/*/hook.yaml",
    "./content/schemas/profile.schema.json":   "**/profiles/*.yaml"
  }
}
```

- [ ] **Step 3: Build and vet**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add content/schemas/hook.schema.json .vscode/settings.json
git commit -m "chore(schema): add hook.schema.json and wire in VS Code settings"
```

---

## Task 8: Documentation updates

**Files:**
- Modify: `docs/content-authoring.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add `hook.yaml` to the pack directory structure tree in `docs/content-authoring.md`**

Find the tree block (around line 13). It currently ends with:
```
└── mcp.yaml           # MCP server definitions wired by `sap-devs mcp install`
```

Change to:
```
├── mcp.yaml           # MCP server definitions wired by `sap-devs mcp install`
└── hook.yaml          # Hook commands wired by `sap-devs hook install`
```

- [ ] **Step 2: Add `## Hook Authoring` section to `docs/content-authoring.md`**

Find the `## The \`### Agent Instructions\` Pattern` section. Add a new `## Hook Authoring` section **after** it (after the section's last paragraph):

```markdown
## Hook Authoring

A pack may include an optional `hook.yaml` file. Each entry declares a shell command to wire into an AI tool's lifecycle event system (e.g. run `sap-devs tip --markdown` every time Claude Code starts a new session).

### `hook.yaml` schema

```yaml
- id: tip-on-session-start     # Unique within the pack
  event: sessionStart          # Tool-neutral event name
  command: "sap-devs tip --markdown"  # Command to run when the event fires
  tools:                       # Adapter IDs that support this hook
    - claude-code
```

| Field     | Type     | Description                                                             |
| --------- | -------- | ----------------------------------------------------------------------- |
| `id`      | string   | Unique identifier. Used by `sap-devs hook install <id>`.                |
| `event`   | string   | Tool-neutral event. Supported values: `sessionStart`.                   |
| `command` | string   | Shell command. Keep it fast (< 200 ms) — it runs on every event fire.  |
| `tools`   | []string | Adapter IDs that support this hook (must have `hook_config` in YAML).  |

### Event values

| `event`        | Claude Code hook key      | When it fires                                |
| -------------- | ------------------------- | -------------------------------------------- |
| `sessionStart` | `hooks.SessionStart`      | Once when a new session starts or resumes    |

### Authoring constraints

- **Keep `command` fast** — hooks run synchronously on every event. Avoid network calls in the hook command itself; `sap-devs tip --markdown` reads from cache and exits in < 100 ms.
- **No headings in output** — hook output is read directly by the AI tool; headings in stdout may confuse context injection.
- **`tools` must match a configured adapter** — if the adapter YAML does not have a `hook_config` block, the hook is silently skipped during install.

### Installing hooks

```bash
sap-devs hook install                      # install all hooks for active profile
sap-devs hook install tip-on-session-start # install a specific hook
sap-devs hook status                       # check what's installed
sap-devs hook uninstall tip-on-session-start
```

### Example: the base pack's session tip hook

`content/packs/base/hook.yaml` ships with one hook:

```yaml
- id: tip-on-session-start
  event: sessionStart
  command: "sap-devs tip --markdown"
  tools:
    - claude-code
```

When installed, Claude Code runs `sap-devs tip --markdown` at every session start and the Markdown output is available to the agent as session context — delivering a daily SAP developer tip as a session greeting.
```

- [ ] **Step 3: Add adapter-author `hook_config` documentation to `docs/content-authoring.md`**

In `docs/content-authoring.md`, find the Hook Authoring section added in Step 2. At the end of the section (before the next `---` divider), add a new subsection:

```markdown
### Adding `hook_config` to an adapter YAML

To make a new AI tool's adapter support hook installation, add a `hook_config` block to its YAML in `content/adapters/<id>.yaml` alongside the existing `mcp_config`:

```yaml
hook_config:
  path: "~/.tool/settings.json"   # path to the tool's settings file (tilde expanded)
  format: json                     # "json" only for now
  key: "hooks.SessionStart"        # dot-separated JSON path to the hook array
```

The `key` field is a dot-separated path that `WriteHookConfig` navigates dynamically. For Claude Code, the value is `"hooks.SessionStart"` — the `SessionStart` hook array in `~/.claude/settings.json`. For a new tool, check its documentation for the equivalent hook event key.

Only adapters with a `hook_config` block can be targeted by `hook install`. Adapters without it are silently skipped.
```

- [ ] **Step 4: Add `hook` row to the CLI commands table in `CLAUDE.md`**

In `CLAUDE.md`, find the CLI commands table. After the `mcp` row:
```
| `mcp list/install/status` | Browse and wire SAP MCP servers into AI tool configs |
```

Add:
```
| `hook list/install/uninstall/status` | Wire AI tool lifecycle hooks from pack definitions |
```

- [ ] **Step 5: Build and vet**

```bash
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add docs/content-authoring.md CLAUDE.md
git commit -m "docs: document hook.yaml authoring, hook_config adapter field, and CLI commands table"
```

---

## Verification

- [ ] **Final build and vet**

```bash
cd d:/projects/sap-devs-cli/.worktrees/feat-hook-system
go build ./... && go vet ./...
```

Expected: clean.

- [ ] **Tip flags work**

```bash
SAP_DEVS_DEV=1 go run . tip --markdown 2>&1 | head -3
SAP_DEVS_DEV=1 go run . tip --plain 2>&1 | head -3
```

Expected: `--markdown` starts with `## 💡`; `--plain` starts with tip title (no `#`).

- [ ] **Hook list shows the base hook**

```bash
SAP_DEVS_DEV=1 go run . hook list --all
```

Expected: table row for `tip-on-session-start`.

- [ ] **Hook status shows not-installed (before install)**

```bash
SAP_DEVS_DEV=1 go run . hook status
```

Expected: `tip-on-session-start` row with `✗ not installed`.

- [ ] **Dry-run install prints what would be written**

```bash
SAP_DEVS_DEV=1 go run . hook install --dry-run
```

Expected: `[dry-run] would add hook "sap-devs tip --markdown" to ~/.claude/settings.json[hooks.SessionStart]`.
