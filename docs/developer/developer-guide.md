# sap-devs Developer Guide

This guide covers everything you need to build, test, and release the `sap-devs` CLI.

---

## Prerequisites

- **Go 1.26.1+** — [download](https://go.dev/dl/)
- **git**
- **Linux only:** `libx11-dev` (required by the clipboard dependency `golang.design/x/clipboard`)
  ```bash
  sudo apt-get install -y libx11-dev
  ```

---

## Clone & Build

```bash
git clone https://github.tools.sap/developer-relations/sap-devs-cli
cd sap-devs-cli

VERSION=$(git describe --tags --always --dirty)
go build -ldflags "-X github.tools.sap/developer-relations/sap-devs-cli/cmd.Version=${VERSION}" -o sap-devs .
```

This produces a `sap-devs` binary in the current directory. The module path is `github.tools.sap/developer-relations/sap-devs-cli`.

---

## Local Development

Set `SAP_DEVS_DEV=1` to load content from `./content/` instead of the user cache. This lets you iterate on content changes without syncing:

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run
```

Use `go run .` (rather than rebuilding) for rapid iteration during development.

---

## Linting & Static Analysis

```bash
go build ./...
go vet ./...
```

> **Windows note:** `go test` always fails locally because Windows Defender blocks execution of test binaries from `~/.config` paths. Use `go build` + `go vet` locally. CI is the authoritative test runner.

---

## Running Tests

```bash
# All packages
go test ./...

# Single package
go test ./internal/content/...
go test ./internal/i18n/...
```

CI runs on a self-hosted `Linux X64` runner and is the authoritative test runner. On Windows, tests may fail locally but pass in CI (Linux). A test failure in CI that passes locally indicates a genuine cross-platform bug.

---

## Project Layout

```
sap-devs-cli/
├── cmd/                    # Cobra command definitions (one file per command)
├── internal/
│   ├── adapter/            # Adapter engine — pushes context into AI tools
│   ├── config/             # Config file read/write
│   ├── content/            # Content loader — merges 4 content layers
│   ├── i18n/               # Internationalisation: language resolution, T(), Tf()
│   │   └── catalogs/       # JSON string catalogs per language (en.json, de.json, …)
│   ├── sync/               # Sync engine — fetches official/company repo zips
│   ├── update/             # Self-update logic
│   └── xdg/                # Platform-native config/cache/data paths
├── content/
│   ├── adapters/           # Adapter definitions (one YAML per AI tool)
│   ├── packs/              # Content packs (one directory per pack)
│   └── profiles/           # Developer persona profiles
├── .github/
│   ├── workflows/ci.yml    # Test + build on every push/PR
│   └── workflows/release.yml  # GoReleaser triggered by v* tags
├── .goreleaser.yml         # Cross-platform release configuration
├── go.mod / go.sum
└── main.go
```

---

## Architecture Overview

### Content Layer System

Content is loaded from up to four sources, merged by `id` with later layers overriding earlier ones:

1. **Official** — fetched from the official repo, cached at `~/.cache/sap-devs/official/`
2. **Company** — optional, set via `sap-devs config company <url>`, cached at `~/.cache/sap-devs/company/`
3. **User** — `~/.local/share/sap-devs/` (Linux), `%LOCALAPPDATA%/sap-devs/data/` (Windows)
4. **Project** — `.sap-devs/` in the current working directory

`ContentLoader` (`internal/content/loader.go`) manages the merge. `LoadPacks()` reads all `content/packs/<name>/` directories.

### Adapter System

Adapters (`content/adapters/<tool>.yaml`) define how to push context into a specific AI tool. Three types:

- **`file-inject`** — writes a fenced section into a config file (e.g. `~/.claude/CLAUDE.md`) using HTML comment markers. The section is identified by markers of the form `<!-- sap-devs:start:Section Name -->` and `<!-- sap-devs:end:Section Name -->`. Currently only `replace-section` mode is implemented (replaces an existing section or appends if not present); `append` mode is defined in the adapter schema but not yet active.
- **`clipboard-export`** — copies context to clipboard (global scope only).
- **`mcp-wire`** — registers MCP servers in the tool's JSON config (used by `mcp install`, not `inject`).

The `Engine` (`internal/adapter/engine.go`) iterates adapters, filters by `--tool` flag and scope (`global`/`project`), and dispatches to the appropriate handler.

### Profiles

Profiles (`content/profiles/`) are YAML files that tag which packs belong to a developer persona (e.g. `cap-developer`). `ApplyWeights()` reorders packs to prioritise those matching the active profile. The active profile is stored in `~/.config/sap-devs/profile.yaml`.

### Sync

`sap-devs sync` (`cmd/sync.go`) fetches the official repo as a `.zip` archive and extracts it to the cache. Per-category TTLs are tracked in `~/.cache/sap-devs/sync-state.json` via `sync.Engine` (`internal/sync/engine.go`). Use `--force` to ignore TTLs.

### i18n

The `internal/i18n` package resolves the active language and looks up strings from JSON catalogs embedded at build time:

- **Language resolution:** `config language` setting → `LANG` env var → `LC_ALL` env var → fallback `en`. Region suffixes stripped (`de_AT.UTF-8` → `de`).
- **CLI strings:** `internal/i18n/catalogs/<lang>.json`, keyed as `cmd.subcommand.string_name`.
- **Pack content:** `context.<lang>.md`, `tips.<lang>.md` alongside base files.
- **Functions:** `T(lang, key string)` for plain strings; `Tf(lang, key string, data map[string]any)` for Go `text/template` strings. Use `i18n.ActiveLang` as the `lang` argument.

`ActiveLang` is set once in `rootCmd.PersistentPreRunE` before any command body runs.

### Update Check

On every command invocation (except `update` and dev builds), a background goroutine checks GitHub for a newer release, at most once per 7 days (168h). The result is printed to stderr after the command completes, with a 3-second timeout.

### Platform Paths

`internal/xdg` resolves platform-native directories:

| Purpose | Linux | macOS | Windows |
|---|---|---|---|
| Config | `~/.config/sap-devs` | `~/Library/Application Support/sap-devs` | `%APPDATA%/sap-devs` |
| Cache | `~/.cache/sap-devs` | `~/Library/Caches/sap-devs` | `%LOCALAPPDATA%/sap-devs/cache` |
| Data | `~/.local/share/sap-devs` | `~/Library/Application Support/sap-devs/data` | `%LOCALAPPDATA%/sap-devs/data` |

XDG environment variables (`XDG_CONFIG_HOME`, `XDG_CACHE_HOME`, `XDG_DATA_HOME`) are honoured on Linux.

---

## Adding a Command

1. Create `cmd/<name>.go`.
2. Define a `*cobra.Command` with `Use`, `Short` (from `i18n.T`), and `RunE`.
3. Follow i18n key convention: `<command>.<subcommand>.short`, `<command>.<subcommand>.long`, etc. Add keys to `internal/i18n/catalogs/en.json`.
4. Register with `rootCmd.AddCommand()` (or the relevant parent) in the file's `init()`.
5. Add flags via `cmd.Flags().StringVar(...)` etc. after the command definition.

Example:

```go
var fooCmd = &cobra.Command{
    Use:   "foo <arg>",
    Short: i18n.T(i18n.ActiveLang, "foo.short"),
    RunE: func(cmd *cobra.Command, args []string) error {
        // implementation
        return nil
    },
}

func init() {
    rootCmd.AddCommand(fooCmd)
}
```

---

## Release Workflow

### Pre-release checklist

- [ ] CI is green on `main` (check `.github/workflows/ci.yml`)
- [ ] All tests pass
- [ ] `CHANGELOG` or commit history is clean and meaningful

### Tag and push

```bash
git tag v1.2.3
git push origin v1.2.3
```

The tag must match the pattern `v*`. Pushing the tag triggers the release workflow at `.github/workflows/release.yml`.

### What GoReleaser does

GoReleaser runs on `ubuntu-latest` and reads `.goreleaser.yml`:

| Platform | Architecture | Archive format |
|---|---|---|
| Linux | amd64, arm64 | `.tar.gz` |
| macOS | amd64, arm64 | `.tar.gz` |
| Windows | amd64 | `.zip` |

Windows arm64 is excluded. Archive naming: `sap-devs_<version>_<os>_<arch>.<ext>`.

Version is injected at build time:
```
-ldflags "-X github.tools.sap/developer-relations/sap-devs-cli/cmd.Version={{ .Version }}"
```

A `checksums.txt` (SHA256) is included in the release assets.

### After the release

1. Go to the GitHub Releases page and verify all platform artifacts are present.
2. Verify `checksums.txt` is attached.
3. Test by downloading and running `sap-devs --version` on at least one platform.

---

## Worktrees

Feature branch worktrees are stored in `.worktrees/` in the project root — **not** in `~/.config`. Windows Defender blocks execution of test binaries from `~/.config` paths.

```bash
# Create a worktree for a feature branch
git worktree add .worktrees/my-feature -b feature/my-feature
```
