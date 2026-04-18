# New Adapters (Windsurf + Gemini Code Assist) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Windsurf and Gemini Code Assist file-inject adapters so `sap-devs inject` pushes SAP context into these tools.

**Architecture:** Pure YAML additions — two new adapter files in `content/adapters/`. The existing `LoadAdapters` function auto-discovers them. No Go code changes.

**Tech Stack:** YAML adapter definitions, Go build/vet verification

**Spec:** `docs/superpowers/specs/2026-04-18-new-adapters-design.md`

---

## File Structure

| Action | File                                      | Responsibility                                                      |
|--------|-------------------------------------------|---------------------------------------------------------------------|
| Create | `content/adapters/windsurf.yaml`          | Windsurf file-inject adapter (project-scope only)                   |
| Create | `content/adapters/gemini-code-assist.yaml` | Gemini Code Assist file-inject adapter (global + project, MCP)      |
| Modify | `TODO.md` (search for `## New Adapters`)  | Mark adapter TODOs as done/skipped                                  |

---

### Task 1: Create Windsurf adapter

**Files:**
- Create: `content/adapters/windsurf.yaml`

- [ ] **Step 1: Create windsurf.yaml**

```yaml
id: windsurf
name: Windsurf
type: file-inject
targets:
  - scope: project
    path: ".windsurf/rules/sap-developer-context.md"
    mode: replace-file
detect:
  - path: "~/.codeium/windsurf"
  - command: "windsurf --version"
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: clean exit, no errors

- [ ] **Step 3: Verify vet**

Run: `go vet ./...`
Expected: clean exit, no errors

- [ ] **Step 4: Verify adapter loads**

Run: `go run . inject --dry-run --tool windsurf`
Expected: output shows Windsurf adapter processing (or "not detected" if Windsurf isn't installed locally — either is fine, the adapter loaded)

- [ ] **Step 5: Verify status reporting**

Run: `go run . inject --status --tool windsurf`
Expected: output shows a status row for Windsurf (not injected yet is fine — confirms the adapter is recognized)

- [ ] **Step 6: Commit**

```bash
git add content/adapters/windsurf.yaml
git commit -m "feat(adapters): add Windsurf file-inject adapter"
```

---

### Task 2: Create Gemini Code Assist adapter

**Files:**
- Create: `content/adapters/gemini-code-assist.yaml`

- [ ] **Step 1: Create gemini-code-assist.yaml**

```yaml
id: gemini-code-assist
name: Gemini Code Assist
type: file-inject
targets:
  - scope: global
    path: "~/.gemini/GEMINI.md"
    mode: replace-section
    section: "SAP Developer Context"
  - scope: project
    path: "./GEMINI.md"
    mode: replace-section
    section: "SAP Developer Context"
detect:
  - path: "~/.gemini"
  - command: "gemini --version"
mcp_config:
  path: "~/.gemini/settings.json"
  format: json
  key: "mcpServers"
```

- [ ] **Step 2: Verify build and vet**

Run: `go build ./...` and `go vet ./...`
Expected: clean exit, no errors

- [ ] **Step 3: Verify adapter loads**

Run: `go run . inject --dry-run --tool gemini-code-assist`
Expected: output shows Gemini Code Assist adapter processing (or "not detected" — either confirms the adapter loaded)

- [ ] **Step 4: Verify status reporting**

Run: `go run . inject --status --tool gemini-code-assist`
Expected: output shows a status row for Gemini Code Assist

- [ ] **Step 5: Commit**

```bash
git add content/adapters/gemini-code-assist.yaml
git commit -m "feat(adapters): add Gemini Code Assist file-inject adapter"
```

---

### Task 3: Update TODO.md

**Files:**

- Modify: `TODO.md` (search for `## New Adapters` heading)

- [ ] **Step 1: Update the three adapter TODO sections**

Search for the `## New Adapters` heading in `TODO.md` and replace the three subsections (Zed, Windsurf, Gemini Code Assist) with:

```markdown
## New Adapters

### ~~Zed editor adapter~~ — Covered by existing Claude Code adapter

Zed reads project root files in priority order: `.rules`, `.cursorrules`, `.windsurfrules`, then `CLAUDE.md`, `GEMINI.md`, etc. Since our Claude Code adapter injects into `CLAUDE.md`, Zed users at project scope are already covered. A dedicated `.rules` file would override `CLAUDE.md` and any user-created `.rules`, so no separate adapter is needed.

---

### ~~Windsurf (Codeium) adapter~~ — Done

Implemented in `content/adapters/windsurf.yaml`. Project-scope `file-inject` targeting `.windsurf/rules/sap-developer-context.md`. Global scope skipped (single shared file, 6k limit).

---

### ~~Gemini Code Assist adapter~~ — Done

Implemented in `content/adapters/gemini-code-assist.yaml`. Both global (`~/.gemini/GEMINI.md`) and project (`./GEMINI.md`) scopes via `replace-section`. MCP config wired to `~/.gemini/settings.json`.
```

- [ ] **Step 2: Commit**

```bash
git add TODO.md
git commit -m "docs: mark adapter TODOs as done/skipped"
```

---

### Note: CLAUDE.md — No update needed

The `CLAUDE.md` Adapter System section describes adapter types (`file-inject`, `clipboard-export`, `mcp-wire`) but does not enumerate individual adapters by name. Since `LoadAdapters` auto-discovers YAML files, no CLAUDE.md change is required.
