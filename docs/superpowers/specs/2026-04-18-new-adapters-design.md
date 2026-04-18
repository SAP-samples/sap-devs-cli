# New Adapters: Windsurf + Gemini Code Assist

**Date:** 2026-04-18
**Status:** Approved

## Summary

Add two new `file-inject` adapters to the sap-devs CLI so that `sap-devs inject` pushes SAP developer context into Windsurf and Gemini Code Assist. Both follow the established YAML-only adapter pattern — no Go code changes required.

Zed was evaluated and skipped: Zed reads `CLAUDE.md` at project root (among other files), so the existing Claude Code adapter already covers Zed users at project scope.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Zed adapter | Skip | Zed reads CLAUDE.md; .rules at root would override user's existing rules |
| Gemini naming | `gemini-code-assist` (separate from existing `gemini.yaml`) | Existing `gemini.yaml` is a clipboard-export for the Gemini chatbot; these are different tools |
| Windsurf scope | Project only | Global rules are a single shared file (~/.codeium/windsurf/memories/global_rules.md, 6k limit); overwriting it would destroy user content |
| Windsurf mode | `replace-file` | Each rule is its own .md file in `.windsurf/rules/`, like Cursor |
| Gemini mode | `replace-section` | GEMINI.md is a shared context file (like CLAUDE.md), so fenced markers preserve user content |

## Adapter Definitions

### Windsurf (`content/adapters/windsurf.yaml`)

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

- **Project-scope only** — avoids overwriting the shared global rules file
- **No preamble** — Windsurf rules are plain markdown (no frontmatter like Cursor's `.mdc`)
- **No `mcp_config`** — Windsurf manages MCP via its own UI, not a writable config file
- **Detection** — `~/.codeium/windsurf` directory (created on install) or `windsurf --version` CLI

### Gemini Code Assist (`content/adapters/gemini-code-assist.yaml`)

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

- **Both scopes** — global at `~/.gemini/GEMINI.md`, project at `./GEMINI.md`
- **`replace-section`** with `"SAP Developer Context"` fenced markers — preserves user content in GEMINI.md
- **MCP config** — Gemini CLI reads MCP servers from `~/.gemini/settings.json` under `mcpServers`
- **Detection** — `~/.gemini` directory or `gemini --version`

## What Doesn't Change

- **No Go code** — `LoadAdapters` auto-discovers any `.yaml` in `content/adapters/`
- **No i18n** — adapter names come from YAML `name` field
- **No schema changes** — all fields used are already in the adapter schema
- **Existing `gemini.yaml`** — untouched, remains as clipboard-export for the Gemini chatbot

## Testing

- `go build ./...` and `go vet ./...` — verify no build regressions
- `sap-devs inject --dry-run --tool windsurf` — verify Windsurf rendering
- `sap-devs inject --dry-run --tool gemini-code-assist` — verify Gemini rendering
- `sap-devs inject --status --tool windsurf` — verify status reporting
- `sap-devs inject --status --tool gemini-code-assist` — verify status reporting

## Documentation Updates

- Update `TODO.md`: mark Zed as covered by Claude Code adapter; mark Windsurf and Gemini Code Assist as done
- Refresh adapter list in `CLAUDE.md` if needed
