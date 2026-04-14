# sap-devs Content Guide

This guide covers how to add, update, and translate content for the `sap-devs` CLI — packs, adapters, and profiles.

---

## Overview: The Content Layer System

Content is loaded from up to four sources, merged by `id` with later layers overriding earlier ones:

| Layer | Location | Purpose |
|---|---|---|
| Official | `~/.cache/sap-devs/official/content/` (Linux), `~/Library/Caches/sap-devs/official/content/` (macOS), or `%LOCALAPPDATA%/sap-devs/cache/official/content/` (Windows) | Fetched from the official repo via `sap-devs sync` |
| Company | `~/.cache/sap-devs/company/content/` (Linux), `~/Library/Caches/sap-devs/company/content/` (macOS), or `%LOCALAPPDATA%/sap-devs/cache/company/content/` (Windows) | Optional; configured via `sap-devs config company <url>` |
| User | `~/.local/share/sap-devs/` (Linux), `~/Library/Application Support/sap-devs/data/` (macOS), `%LOCALAPPDATA%/sap-devs/data/` (Windows) | Per-user overrides |
| Project | `.sap-devs/` in the current working directory | Per-project overrides |

If two layers define a pack with the same `id`, the later layer wins completely.

---

## Pack Structure

Each pack lives in `content/packs/<pack-id>/`. All files are optional except `pack.yaml`.

```
content/packs/cap/
├── pack.yaml          # Pack metadata
├── context.md         # AI context text (English)
├── context.de.md      # German AI context text (optional)
├── tips.md            # Tips (English)
├── tips.de.md         # German tips (optional)
├── tools.yaml         # Tool version requirements
├── resources.yaml     # Curated resource links
└── mcp.yaml           # MCP server definitions
```

### `pack.yaml`

```yaml
id: cap                          # unique slug — used as the merge key
name: SAP Cloud Application Programming Model
description: Node.js and Java framework for building cloud-native business applications on BTP
tags: [cloud, btp, nodejs, java, odata, cds]
profiles: [cap-developer, btp-developer]   # informational metadata only — see note below
weight: 100                      # default weight; profiles can override this per-pack
locales:
  de:
    name: SAP Cloud Application Programming Model
    description: Node.js- und Java-Framework für Cloud-native Business-Anwendungen auf BTP
```

> **Note:** `weight` sets the default priority for this pack. Individual profiles can override it in their `packs` list (see [Profiles](#profiles) below).
>
> **Note:** The `profiles` field is informational metadata only. A pack is only active when it is explicitly listed in a profile's `packs` list. Adding a pack id to `profiles` here does not automatically include it in that profile.

### `context.md`

Free-form Markdown injected as AI context into tools. No special syntax. May be long-form reference material, code examples, or conventions.

### `context.<lang>.md`

Localised version of `context.md` for language `<lang>` (e.g. `context.de.md`). Falls back to `context.md` if absent for the active language.

### `tips.md`

H2-delimited tips. Each tip:

```markdown
## Use cds watch for local development
Tags: cap,nodejs
Run `cds watch` instead of `node server.js` — it reloads on every file change.

## Define managed entities for audit fields
Tags: cap,cds
Add `: managed` to your entities to get createdAt, createdBy, modifiedAt, modifiedBy for free.
```

- `## <Title>` — required
- `Tags: tag1,tag2` — optional, one line immediately after the heading
- Body — tip content, may be multi-line Markdown

### `tips.<lang>.md`

Localised version of `tips.md`. Same format.

### `tools.yaml`

```yaml
- id: nodejs
  name: Node.js
  required: ">=18.0.0"          # semver constraint
  detect:
    command: "node --version"
    pattern: "v(\\d+\\.\\d+\\.\\d+)"   # regex capturing the version string
  install:
    windows: "winget install OpenJS.NodeJS.LTS"
    macos: "brew install node@20"
    linux: "nvm install 20"
  docs: "https://nodejs.org"

- id: cds-dk
  name: SAP CDS CLI
  required: ">=7.0.0"
  detect:
    command: "cds --version"
    pattern: "@sap/cds: (\\d+\\.\\d+\\.\\d+)"
  install:
    all: "npm install -g @sap/cds-dk"   # "all" key applies to every platform
  docs: "https://cap.cloud.sap"
```

### `resources.yaml`

```yaml
- id: cap/docs-official           # <pack-id>/<slug>
  title: CAP Documentation
  url: https://cap.cloud.sap/docs
  type: official-docs             # official-docs | sample | community | tutorial | blog
  tags: [reference, getting-started]
```

### `mcp.yaml`

MCP server definitions for the pack. The file is a YAML list of `MCPServer` entries.

```yaml
- id: cap-mcp                        # unique slug within the pack
  name: CAP MCP Server
  description: MCP server for SAP CAP development
  install:
    command: "npx"                   # executable to run
    args: ["-y", "@sap/cap-mcp"]    # arguments passed to the command
  hosts: [claude-code, cursor]       # AI tools that should register this server
```

Fields:

- `id` — unique identifier (combined with `PackID` at load time)
- `name` — human-readable display name
- `description` — short description shown in `sap-devs mcp list`
- `install.command` — executable to run (e.g. `npx`, `node`, `python`)
- `install.args` — list of arguments passed to the command
- `hosts` — list of AI tool IDs that should register this server (used by `sap-devs mcp install`)

> **Note:** All existing `mcp.yaml` files in official packs are currently stubs pending Plan 2 implementation.

---

## Adapters

Adapters define how to push content into a specific AI tool. Files live in `content/adapters/<tool-id>.yaml`.

```yaml
id: claude-code
name: Claude Code
type: file-inject                 # file-inject | clipboard-export | mcp-wire
targets:
  - scope: global                 # global (user-level) or project (current dir)
    path: "~/.claude/CLAUDE.md"
    mode: replace-section         # replace-section | append
    section: "SAP Developer Context"
  - scope: project
    path: "./CLAUDE.md"
    mode: replace-section
    section: "SAP Developer Context"
detect:
  - command: "claude --version"   # run this; if it exits 0, tool is present
  - path: "~/.claude"             # or check if this path exists
mcp_config:
  path: "~/.claude/settings.json"
  format: json
  key: "mcpServers"
```

**Modes:**
- `replace-section` — replaces the named fenced section. The injected block is wrapped with `<!-- sap-devs:start:SAP Developer Context -->` and `<!-- sap-devs:end:SAP Developer Context -->` markers.
- `append` — defined in the adapter schema but not yet implemented; using it causes a runtime error. Do not use until engine support is added.

**Types:**
- `file-inject` — writes context to a config file. Used by `sap-devs inject`.
- `clipboard-export` — copies context to clipboard (global scope only).
- `mcp-wire` — registers MCP servers in the tool's JSON config. Used by `sap-devs mcp install`, not `inject`.

---

## Profiles

Profiles tag which packs belong to a developer persona. Files live in `content/profiles/<profile-id>.yaml`.

```yaml
id: cap-developer
name: CAP Developer
description: Building cloud-native apps with SAP CAP on BTP
packs:
  - id: cap
    weight: 100       # higher weight = appears earlier in rendered context
  - id: btp-core
    weight: 80
  - id: abap
    weight: 10
tip_tags: [cap, nodejs, odata, cds, btp]   # tips with these tags are preferred for this profile
```

`ApplyWeights()` reorders packs so higher-weight packs appear first in the rendered context injected into AI tools.

---

## Creating a New Pack

1. Create the directory:
   ```bash
   mkdir content/packs/<new-id>
   ```

2. Write `pack.yaml` with a unique `id`:
   ```yaml
   id: my-pack
   name: My Pack
   description: What this pack covers
   tags: [tag1, tag2]
   profiles: [cap-developer]   # or whichever profile(s) should include it
   ```

3. Add content files as needed (`context.md`, `tips.md`, `tools.yaml`, `resources.yaml`). All are optional.

4. Reference the pack in at least one profile:
   ```yaml
   # content/profiles/cap-developer.yaml
   packs:
     - id: my-pack
       weight: 50
   ```

5. Test with dev mode:
   ```bash
   SAP_DEVS_DEV=1 sap-devs inject --dry-run
   ```
   Verify the new context appears in the output.

---

## Updating Existing Content

Edit the file in the relevant layer. The official layer is under `content/packs/` in this repo. To override official content without modifying it:

- **User override:** Copy the pack to `~/.local/share/sap-devs/packs/<id>/` (Linux) and edit there.
- **Project override:** Copy to `.sap-devs/packs/<id>/` in the project directory.

The override pack must use the same `id` as the official pack. The later layer wins.

---

## Translation Guide

### How Language Resolution Works

The active language is resolved in this order:

1. `sap-devs config set language <tag>` — explicit config setting
2. `LANG` environment variable (e.g. `de_AT.UTF-8` → `de`)
3. `LC_ALL` environment variable
4. Fallback: `en`

Region and encoding suffixes are stripped: `de_AT.UTF-8` → `de`. If the resolved tag has no catalog, it falls back to `en`.

### Translating Pack Content Files

Add localised content files alongside the base files:

```
content/packs/cap/
├── context.md         # base (English)
├── context.de.md      # German translation
├── tips.md            # base (English)
└── tips.de.md         # German translation
```

The loader automatically picks the locale-specific file when the active language matches. Falls back to the base file if the locale file is absent.

### Translating Pack Metadata

Add a `locales` block to `pack.yaml`:

```yaml
locales:
  de:
    name: SAP Cloud Application Programming Model
    description: Node.js- und Java-Framework für Cloud-native Business-Anwendungen auf BTP
  fr:
    name: SAP Cloud Application Programming Model
    description: Framework Node.js et Java pour applications cloud natives sur BTP
```

### Translating CLI Strings

CLI strings live in `internal/i18n/catalogs/<lang>.json`. Copy `en.json` as a starting point — any missing keys fall back to English automatically.

```bash
cp internal/i18n/catalogs/en.json internal/i18n/catalogs/fr.json
# Edit fr.json with French translations
```

**Key naming convention:** `<command>.<subcommand>.<string_name>`

```json
{
  "inject.short": "Pousser le contexte SAP dans vos outils IA",
  "inject.done": "Contexte SAP injecté (portée {{.Scope}})."
}
```

Values may be plain strings or Go `text/template` expressions (e.g. `{{.Scope}}`).

**In Go code**, strings are looked up with:
- `i18n.T(lang, key)` — plain string
- `i18n.Tf(lang, key, data)` — template string, where `data` is `map[string]any`
- Pass `i18n.ActiveLang` as the `lang` argument

### Adding a New Language End-to-End

1. **Create the CLI catalog:**
   ```bash
   cp internal/i18n/catalogs/en.json internal/i18n/catalogs/<lang>.json
   ```
   Translate values. Leave any untranslated keys as-is (they fall back to English).

2. **Add localised content files** to each pack you want to translate:
   ```bash
   # For each pack:
   cp content/packs/<id>/context.md content/packs/<id>/context.<lang>.md
   cp content/packs/<id>/tips.md content/packs/<id>/tips.<lang>.md
   # Edit the new files with translations
   ```

3. **Add `locales` blocks** to each `pack.yaml` you translated.

4. **Test:**
   ```bash
   sap-devs config set language <lang>
   SAP_DEVS_DEV=1 sap-devs inject --dry-run
   SAP_DEVS_DEV=1 sap-devs tip
   ```
   Verify translated strings appear in output.

5. **Reset language** when done testing:
   ```bash
   sap-devs config set language en
   ```
