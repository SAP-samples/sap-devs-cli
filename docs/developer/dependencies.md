# Third-Party Dependencies Reference

This document catalogs every direct third-party dependency used by the `sap-devs` CLI, explaining what it does, why it was chosen, and where it is used. Indirect (transitive) dependencies are excluded unless they warrant special mention.

All dependencies are listed in [`go.mod`](../../go.mod). Go standard library and `golang.org/x/*` extended-standard packages are listed in a separate section at the end.

---

## CLI Framework

### cobra — `github.com/spf13/cobra` v1.10.2

| | |
|---|---|
| **What** | CLI command framework with subcommand routing, flag parsing, help generation, and shell completion |
| **Why** | Industry-standard Go CLI library; handles the entire command tree, flag binding, argument validation, and auto-generated help/usage text |
| **Where** | Every file in [`cmd/`](../../cmd/) — each command is a `*cobra.Command` registered on the root |
| **Sub-packages** | Root only |

### pflag — `github.com/spf13/pflag` v1.0.10

| | |
|---|---|
| **What** | POSIX-compliant flag parsing (drop-in replacement for Go's `flag` package) |
| **Why** | Required by cobra for flag binding; one direct import in tests for flag set manipulation |
| **Where** | Transitively via cobra in all commands; directly in [`cmd/config_token_test.go`](../../cmd/config_token_test.go) |
| **Sub-packages** | Root only |

---

## Terminal UI (TUI)

The TUI stack is built on the [Charm](https://charm.sh/) ecosystem. Five Charm packages are used directly, plus two versions of lipgloss (see note below).

### bubbletea — `github.com/charmbracelet/bubbletea` v1.3.10

| | |
|---|---|
| **What** | Elm-architecture terminal UI framework (Model → Update → View loop) |
| **Why** | Powers all interactive terminal experiences: progress displays, the content editor, the tutorial step-through TUI |
| **Where** | [`internal/ui/progress.go`](../../internal/ui/progress.go) (sync marker progress), [`internal/editor/editor.go`](../../internal/editor/editor.go) (content editor model), [`internal/editor/diff.go`](../../internal/editor/diff.go) (diff view model), [`internal/editor/list.go`](../../internal/editor/list.go) (list view model), [`cmd/tutorial_tui.go`](../../cmd/tutorial_tui.go) (interactive tutorial runner) |
| **Sub-packages** | Root only (aliased as `tea`) |

### huh — `charm.land/huh/v2` v2.0.3

| | |
|---|---|
| **What** | High-level form/prompt library built on bubbletea v2 |
| **Why** | Provides themed, accessible form inputs (text fields, selects, confirms, multi-selects) without hand-building bubbletea models for each form |
| **Where** | [`cmd/config_edit.go`](../../cmd/config_edit.go) (config editing form), [`cmd/content_edit.go`](../../cmd/content_edit.go) (content editing form), [`internal/editor/form.go`](../../internal/editor/form.go) (schema-driven form generation), [`internal/editor/editor.go`](../../internal/editor/editor.go) (editor form interactions), [`internal/editor/bulk.go`](../../internal/editor/bulk.go) (bulk editing forms), [`internal/theme/fiori.go`](../../internal/theme/fiori.go) (Fiori theme definition) |
| **Sub-packages** | Root only |

### glamour — `github.com/charmbracelet/glamour` v1.0.0

| | |
|---|---|
| **What** | Terminal markdown renderer with syntax highlighting and word wrapping |
| **Why** | Renders markdown content (tutorials, tips, learning paths) as styled terminal output instead of raw text |
| **Where** | [`cmd/tutorials.go`](../../cmd/tutorials.go), [`cmd/tutorial_tui.go`](../../cmd/tutorial_tui.go), [`cmd/tip.go`](../../cmd/tip.go), [`cmd/learning.go`](../../cmd/learning.go), [`cmd/learn_path.go`](../../cmd/learn_path.go) |
| **API used** | `glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(80))` and `glamour.Render(md, "dark")` |
| **Sub-packages** | Root only |

### lipgloss v1 — `github.com/charmbracelet/lipgloss` v1.1.1

| | |
|---|---|
| **What** | Terminal text styling and layout (colors, borders, padding, alignment) |
| **Why** | Used for all styled terminal output in the editor's list/diff views and theme style exports |
| **Where** | [`internal/theme/fiori.go`](../../internal/theme/fiori.go) (aliased as `lipglossv1` — style factory functions for selected rows, headers, layer badges, diff colors), [`internal/editor/diff.go`](../../internal/editor/diff.go), [`internal/editor/list.go`](../../internal/editor/list.go) |
| **Sub-packages** | Root only |

### lipgloss v2 — `charm.land/lipgloss/v2` v2.0.1

| | |
|---|---|
| **What** | Next-generation lipgloss with the charm.land module path, required by huh v2 |
| **Why** | huh v2 builds on the v2 style API; the SAP Fiori theme defines its color palette using v2 types |
| **Where** | [`internal/theme/fiori.go`](../../internal/theme/fiori.go) (color constants: `FioriBackground`, `FioriBlue`, etc.; border styles in `ThemeFiori()`) |
| **Sub-packages** | Root only |

> **Why two lipgloss versions?** The huh v2 form library (`charm.land/huh/v2`) depends on lipgloss v2 (`charm.land/lipgloss/v2`) for its theming system. Meanwhile, the editor's list and diff views were built with lipgloss v1 (`github.com/charmbracelet/lipgloss`), which bubbletea v1 depends on. Both coexist without conflict because Go treats different module paths as separate packages. In [`internal/theme/fiori.go`](../../internal/theme/fiori.go), v2 is the default import and v1 is aliased as `lipglossv1`.

---

## HTML & Markup Processing

### html-to-markdown — `github.com/JohannesKaufmann/html-to-markdown/v2` v2.5.0

| | |
|---|---|
| **What** | Converts HTML documents to clean Markdown text |
| **Why** | Two use cases: (1) converting fetched SAP Community blog post HTML into readable markdown for the `news read` command, (2) converting HTML fetched by sync markers into markdown for content injection |
| **Where** | [`internal/community/community.go`](../../internal/community/community.go) (`ExtractMarkdown` and `FetchPostContent`), [`internal/sync/convert.go`](../../internal/sync/convert.go) (`convertContent` for `"markdown"` format) |
| **API used** | `htmltomarkdown.ConvertString(html)` |
| **Sub-packages** | Root only (aliased as `htmltomarkdown`) |
| **Transitives** | Pulls in `github.com/JohannesKaufmann/dom` for DOM traversal |

### cascadia — `github.com/andybalholm/cascadia` v1.3.3

| | |
|---|---|
| **What** | CSS selector engine for Go's `html.Node` tree |
| **Why** | Sync markers can specify a CSS selector to scope HTML extraction to a specific DOM element before conversion (e.g., `selector=".main-content"`), avoiding navigation/footer noise |
| **Where** | [`internal/sync/convert.go`](../../internal/sync/convert.go) — `cascadia.Compile(selector)` and `cascadia.Query(doc, sel)` |
| **Sub-packages** | Root only |

### yaml.v3 — `gopkg.in/yaml.v3` v3.0.1

| | |
|---|---|
| **What** | YAML marshaling/unmarshaling with support for comments, anchors, and ordered maps |
| **Why** | All configuration and content files (pack.yaml, resources.yaml, tools.yaml, mcp.yaml, profiles, config, changelogs) use YAML format |
| **Where** | [`internal/config/config.go`](../../internal/config/config.go), [`internal/content/pack.go`](../../internal/content/pack.go), [`internal/content/profile.go`](../../internal/content/profile.go), [`internal/adapter/adapter.go`](../../internal/adapter/adapter.go), [`internal/editor/editor.go`](../../internal/editor/editor.go), [`internal/editor/merge.go`](../../internal/editor/merge.go), [`internal/scratch/scratch.go`](../../internal/scratch/scratch.go), [`internal/sync/changelog.go`](../../internal/sync/changelog.go), [`internal/tutorials/parser.go`](../../internal/tutorials/parser.go), [`cmd/content_list.go`](../../cmd/content_list.go), [`cmd/sync.go`](../../cmd/sync.go) |
| **Sub-packages** | Root only |

---

## MCP (Model Context Protocol)

### mcp-go — `github.com/mark3labs/mcp-go` v0.48.0

| | |
|---|---|
| **What** | Go SDK for the Model Context Protocol — provides server creation, tool registration, and stdio transport |
| **Why** | Implements `sap-devs mcp serve`, which exposes SAP developer knowledge (tips, resources, errors, samples, news, learning) as live MCP tools that AI agents can call on demand |
| **Where** | [`cmd/mcp_serve.go`](../../cmd/mcp_serve.go) (server startup), [`internal/mcpserver/server.go`](../../internal/mcpserver/server.go) (server configuration and tool registration), [`internal/mcpserver/tools_content.go`](../../internal/mcpserver/tools_content.go), [`internal/mcpserver/tools_resources.go`](../../internal/mcpserver/tools_resources.go), [`internal/mcpserver/tools_samples.go`](../../internal/mcpserver/tools_samples.go), [`internal/mcpserver/tools_news.go`](../../internal/mcpserver/tools_news.go), [`internal/mcpserver/tools_errors.go`](../../internal/mcpserver/tools_errors.go), [`internal/mcpserver/tools_learn.go`](../../internal/mcpserver/tools_learn.go) |
| **Sub-packages** | `github.com/mark3labs/mcp-go/mcp` (protocol types, tool definitions), `github.com/mark3labs/mcp-go/server` (server instance, handler registration, stdio transport) |

---

## System Integration

### go-keyring — `github.com/zalando/go-keyring` v0.2.8

| | |
|---|---|
| **What** | Cross-platform OS keychain abstraction (macOS Keychain, Windows Credential Manager, Linux Secret Service via D-Bus) |
| **Why** | Securely stores GitHub tokens and service-specific API keys without writing plaintext to disk; falls back to a `credentials` file (mode 0600) when the keychain is unavailable (e.g., headless CI) |
| **Where** | [`internal/credentials/credentials.go`](../../internal/credentials/credentials.go) — `goKeyring.Get()`, `goKeyring.Set()`, `goKeyring.Delete()`, `goKeyring.ErrNotFound` |
| **Sub-packages** | Root only (aliased as `goKeyring`) |
| **Platform deps** | Linux: `godbus/dbus/v5` for Secret Service. Windows: `danieljoos/wincred` for Credential Manager. |

### clipboard — `golang.design/x/clipboard` v0.7.1

| | |
|---|---|
| **What** | Cross-platform clipboard read/write (X11, Wayland, macOS, Windows) |
| **Why** | The `clipboard-export` adapter type copies rendered SAP developer context to the system clipboard for pasting into AI tools that don't support file injection |
| **Where** | [`internal/adapter/clipboard.go`](../../internal/adapter/clipboard.go) — `clipboard.Init()`, `clipboard.Write(clipboard.FmtText, data)` |
| **Sub-packages** | Root only |
| **Platform deps** | Linux: requires `libx11-dev` at build time and an X11/Wayland display at runtime. Falls back to printing to stdout when unavailable. |

### browser — `github.com/pkg/browser` v0.0.0-20240102092130-5ac0b6a4141c

| | |
|---|---|
| **What** | Opens URLs in the user's default browser (Linux: `xdg-open`, macOS: `open`, Windows: `rundll32`) |
| **Why** | Every browsable command (`resources`, `tutorials`, `samples`, `news`, `events`, `discovery`, `learning`, `influencers`, `videos`) has an `open` subcommand or `--open` flag |
| **Where** | [`cmd/resources.go`](../../cmd/resources.go), [`cmd/tutorials.go`](../../cmd/tutorials.go), [`cmd/samples.go`](../../cmd/samples.go), [`cmd/news.go`](../../cmd/news.go), [`cmd/events.go`](../../cmd/events.go), [`cmd/discovery.go`](../../cmd/discovery.go), [`cmd/discovery_services.go`](../../cmd/discovery_services.go), [`cmd/discovery_guidance.go`](../../cmd/discovery_guidance.go), [`cmd/learning.go`](../../cmd/learning.go), [`cmd/learn_path.go`](../../cmd/learn_path.go), [`cmd/influencers.go`](../../cmd/influencers.go), [`cmd/videos.go`](../../cmd/videos.go) |
| **Sub-packages** | Root only |

---

## Testing

### testify — `github.com/stretchr/testify` v1.11.1

| | |
|---|---|
| **What** | Test assertion library providing `assert` (soft failures) and `require` (hard failures) |
| **Why** | Standard Go testing assertions — cleaner than hand-rolling `if got != want { t.Fatalf(...) }` for every check |
| **Where** | All `*_test.go` files across `cmd/` and `internal/` packages (50+ test files) |
| **Sub-packages** | `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/require` |

---

## Go Extended Standard Library (`golang.org/x/*`)

These are official Go team packages that supplement the standard library. They follow Go's compatibility promise but are versioned separately.

### x/net — `golang.org/x/net` v0.47.0

| | |
|---|---|
| **What** | Extended networking packages including an HTML5-compliant tokenizer/parser |
| **Sub-package used** | `golang.org/x/net/html` |
| **Why** | The HTML parser is lenient (never fails on malformed HTML), making it suitable for parsing arbitrary web pages fetched by sync markers |
| **Where** | [`internal/sync/convert.go`](../../internal/sync/convert.go) — `html.Parse()`, `html.Render()`, `html.Node` tree traversal |
| **Also** | Transitively used by `html-to-markdown`, `glamour`, and `bluemonday` |

### x/sync — `golang.org/x/sync` v0.20.0

| | |
|---|---|
| **What** | Concurrency primitives beyond the standard library |
| **Sub-package used** | `golang.org/x/sync/errgroup` |
| **Why** | Bounded-concurrency parallel work with error propagation — used for concurrent tutorial sync (5 repos in parallel) and concurrent frontmatter fetch (10 in parallel) |
| **Where** | [`cmd/sync.go`](../../cmd/sync.go) — two `errgroup.WithContext()` groups with `SetLimit(5)` and `SetLimit(10)` |

### x/term — `golang.org/x/term` v0.37.0

| | |
|---|---|
| **What** | Terminal handling utilities |
| **Why** | `ReadPassword()` reads a line of input without echoing it to the terminal — used for interactive token input so credentials are not visible on screen |
| **Where** | [`cmd/config.go`](../../cmd/config.go), [`cmd/init.go`](../../cmd/init.go), [`cmd/inject.go`](../../cmd/inject.go) — all token prompt flows |

---

## Notable Transitive Dependencies

These are not directly imported but are worth knowing about because they carry platform-specific requirements or are critical to functionality.

| Package | Pulled in by | Purpose |
|---------|-------------|---------|
| `github.com/alecthomas/chroma/v2` | glamour | Syntax highlighting in terminal markdown rendering |
| `github.com/microcosm-cc/bluemonday` | glamour | HTML sanitization before markdown rendering |
| `github.com/yuin/goldmark` | glamour | Markdown parser (CommonMark-compliant) |
| `github.com/danieljoos/wincred` | go-keyring | Windows Credential Manager backend |
| `github.com/godbus/dbus/v5` | go-keyring | Linux D-Bus client for Secret Service keychain |
| `github.com/atotto/clipboard` | bubbletea | Internal clipboard for bubbletea's paste handling |
| `github.com/google/uuid` | mcp-go | UUID generation for MCP protocol messages |
| `golang.org/x/exp/shiny`, `golang.org/x/image`, `golang.org/x/mobile` | clipboard (golang.design) | Platform windowing/display backends for clipboard access |

---

## Summary by Category

| Category | Packages | Purpose |
|----------|----------|---------|
| **CLI framework** | cobra, pflag | Command routing, flag parsing, help generation |
| **Terminal UI** | bubbletea, huh, glamour, lipgloss v1, lipgloss v2 | Interactive forms, progress displays, markdown rendering, text styling |
| **Markup processing** | html-to-markdown, cascadia, yaml.v3, x/net (html) | HTML→Markdown conversion, CSS selector scoping, YAML config/content, HTML parsing |
| **MCP protocol** | mcp-go | MCP server for AI agent tool access |
| **System integration** | go-keyring, clipboard, browser, x/term | OS keychain, clipboard, browser launch, password input |
| **Concurrency** | x/sync (errgroup) | Bounded parallel work with error propagation |
| **Testing** | testify | Assertions in unit/integration tests |

## Dependency Count

| Type | Count |
|------|-------|
| Direct dependencies | 18 |
| Indirect (transitive) | 36 |
| **Total** | **54** |
