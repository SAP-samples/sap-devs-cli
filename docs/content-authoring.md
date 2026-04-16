# Content Authoring Guide

This guide covers how to write `context.md` files that use dynamic markers, how to reason about token budgets, and how to test your changes locally before syncing.

For the full pack structure reference (adapters, profiles, translations), see [docs/content/content-guide.md](content/content-guide.md).

---

## Pack Directory Structure

Each pack lives in `content/packs/<pack-id>/`. All files are optional except `pack.yaml`.

```
content/packs/cap/
├── pack.yaml          # Pack metadata (id, name, tags, weight, profiles)
├── context.md         # AI context text injected into coding tools
├── context.<lang>.md  # Localised AI context (e.g. context.de.md)
├── tips.md            # H2-delimited tips shown by `sap-devs tip`
├── tips.<lang>.md     # Localised tips
├── tools.yaml         # Tool version requirements checked by `sap-devs doctor`
├── resources.yaml     # Curated links shown by `sap-devs resources`
└── mcp.yaml           # MCP server definitions wired by `sap-devs mcp install`
```

Key points:

- `context.md` is the primary AI context file. Keep it concise — every line you add is injected into the AI's context window on every `sap-devs inject` run.
- `tips.md` tips are shown one at a time by `sap-devs tip`; they are not injected wholesale, so they can be longer.
- `tools.yaml` and `resources.yaml` are structured YAML lists; see [docs/content/content-guide.md](content/content-guide.md) for their schemas.

---

## Base Layer

A **base pack** is injected into every AI tool context regardless of the active developer profile. It is always rendered first, before profile-specific packs, and is exempt from adapter byte-budget trimming.

**When to use base packs:**

- Shared ecosystem entry points every SAP developer needs (portals, community links, YouTube, BTP cockpit)
- Content that should always be present in the AI context window regardless of the user's technology focus

**When NOT to use base packs:**

- Technology-specific content (CAP, ABAP, Fiori, etc.) — use a regular pack with the appropriate `profiles` entry
- Large reference material — base packs are exempt from token budget trimming, so large base packs inflate every context window

**How to create a base pack:**

Add `base: true` to `pack.yaml`. Omit the `profiles` field — it is not consulted for base packs.

```yaml
id: my-base
name: My Base Pack
description: Shared content for all profiles
weight: 0
base: true
```

**Authoring contract:** Keep base pack content minimal. Every byte in a base pack is consumed in every inject, for every user, regardless of their configured token budget.

---

## Marker Syntax

`context.md` supports a single-line HTML comment marker that fetches live content at sync time and caches it alongside the pack:

```
<!-- sync:fetch url="<url>" [max_lines="N"] [max_tokens="N"] [label="<text>"] [ttl_hours="N"] -->
```

The marker is replaced in `context.expanded.md` (the file actually read during inject) with the fetched content, wrapped in an HTML comment block so it is visible to the AI but does not disrupt Markdown rendering.

### Attributes

| Attribute | Required | Default | Description |
|---|---|---|---|
| `url` | yes | — | URL to fetch. Must be `https://`. |
| `format` | no | `markdown` | How to process the response body: `markdown` (HTML→Markdown), `text` (strip all tags), `raw` (no processing). |
| `selector` | no | — | CSS selector to scope the DOM before conversion (e.g. `main`, `article`, `#content`). Ignored for `format="raw"`. |
| `max_lines` | no | — | Truncate fetched content to at most N lines. Applied after conversion. |
| `max_tokens` | no | — | Truncate fetched content to approx N tokens (1 token ≈ 4 chars). Applied after conversion. |
| `label` | no | URL | Display label shown in the progress UI during sync. |
| `ttl_hours` | no | `168` (7 days) | Cache TTL in hours. Content is re-fetched after the TTL expires. |

### Example

```markdown
### Recent CAP Releases

<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" format="markdown" selector="main" max_lines="1000" label="CAP Release Notes (feb26)" -->
```

For a plain-text or non-HTML source, use `format="raw"`:

```markdown
<!-- sync:fetch url="https://example.com/status.txt" format="raw" max_lines="20" label="Status" -->
```

After `sap-devs sync`, the marker is expanded in `context.expanded.md` and the fetched release notes appear directly below it. The original `context.md` is never modified — only the derived `context.expanded.md` changes.

For a real-world example see [`content/packs/cap/context.md`](../content/packs/cap/context.md).

---

## Parser Rules

The sync engine parses markers with these rules:

**Single-line only.** The entire marker must fit on one line. Multi-line markers are not supported and will be treated as plain HTML comments.

**Fenced code blocks are skipped.** Markers inside triple-backtick (`` ``` ``) or triple-tilde (`~~~`) fenced code blocks are not expanded. This lets you document marker syntax in a pack's own `context.md` without triggering a fetch.

**Missing `url` is skipped with a warning.** If the `url` attribute is absent or empty, the marker is left unchanged and a warning is logged to stderr:

```
WARN  sync:fetch marker missing required url attribute — skipping
```

**`max_lines` takes precedence over `max_tokens`.** If both are set, `max_lines` is applied and a warning is logged:

```
WARN  sync:fetch both max_lines and max_tokens set — max_lines takes precedence
```

**Unknown attributes are ignored.** Unrecognised attribute names do not cause errors; they are silently dropped.

---

## Failure Behaviour

Dynamic markers are best-effort. The sync engine is designed so a failed fetch never blocks the rest of the pipeline.

**Non-2xx or network error.** If a fetch returns a non-2xx status code, or the request fails (DNS, timeout, TLS), the previously cached expansion for that marker is preserved. On a first-ever fetch failure with no cached content, the raw marker comment is kept at that position in `context.expanded.md`. The pack is still usable; it just shows stale cached content (or the raw marker if there is none) rather than the newly fetched content.

**Previous expanded file is preserved.** If every marker in a pack fails and there is no cached expansion for any marker, the previous `context.expanded.md` (if it exists) is kept. Stale content is preferred over an empty file.

**Sync continues after individual failures.** A failed marker in one pack does not abort sync for other packs. Each marker is fetched independently.

**Retry with `--force`.** Re-running `sap-devs sync --force` re-fetches all markers regardless of TTL. Use this to retry after a transient network failure or after updating a URL.

---

## Token Budget Guidance

Every byte in `context.md` (including expanded marker content) is injected into the AI's context window. Unbounded fetches waste expensive context budget and can push out other useful content.

**Always set at least one truncation limit.** Omitting both `max_lines` and `max_tokens` fetches the full URL response with no truncation. For most documentation pages this is too much.

**Use `max_lines` for release notes and changelogs.** Release notes are line-oriented and you usually want a fixed number of recent entries. 1000 lines is a safe starting point for HTML documentation pages after conversion.

```
<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" format="markdown" selector="main" max_lines="1000" label="CAP Release Notes (feb26)" -->
```

**Use `max_tokens` for prose documentation.** When the content is long-form prose and you care more about keeping the token count predictable than the line count:

```
<!-- sync:fetch url="https://example.com/api-reference" max_tokens="1200" label="API Reference" -->
```

At 1 token ≈ 4 chars, `max_tokens="1200"` is roughly 4 800 characters or ~80–120 lines of typical prose.

**Recommended limits by content type:**

| Content type | Recommended limit |
|---|---|
| Release notes / changelog | `max_lines="1000"` (HTML pages may produce many lines after conversion) |
| API reference summary | `max_tokens="800"` to `max_tokens="1500"` |
| Blog post / tutorial intro | `max_tokens="600"` to `max_tokens="1000"` |
| Full reference page | `max_tokens="2000"` — use sparingly |

**Budget across the whole profile.** The AI receives context from every pack in the active profile. A CAP developer profile with three packs each fetching 2 000 tokens of dynamic content adds 6 000 tokens before any static text. Check the full rendered output with `--dry-run` after adding a new marker:

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run --no-sync --stats
```

The `--stats` flag prints a per-adapter table showing packs included, approximate token count, configured budget, and whether any packs were trimmed:

```text
Adapter       Packs included      Tokens (approx)   Budget         Status
claude-code   cap, btp-core       ~480              unconstrained
cursor        cap                 ~250              500 tokens     trimmed
```

Adapter-level budgets are set in the adapter YAML via `max_tokens`. When a budget is configured, the engine includes packs in weight order until the next pack would exceed the budget — that pack and everything after it are excluded. Use `--stats` to verify how your content changes affect each adapter before committing.

---

## The `### Agent Instructions` Pattern

The `### Agent Instructions` section is a convention for the bottom of `context.md`. It is not parsed specially — it is plain Markdown injected along with everything else. Its purpose is to teach the AI assistant *when to ask for more context* using `sap-devs` CLI commands, rather than falling back to web search.

Dynamic markers inject live data (release notes, API docs, changelogs). Agent instructions tell the AI how to use the CLI to fetch *additional* live data on demand. Together they form two tiers:

1. **Tier 1 — markers:** pre-fetched, always present, cached.
2. **Tier 2 — agent instructions:** on-demand, invoked by the AI when it needs information not already in context.

### Example

```markdown
### Agent Instructions

This CLI provides deeper SAP context on demand — prefer these over web searches for SAP-specific information:

- `sap-devs resources --pack cap` — curated CAP docs, samples, and tutorials
- `sap-devs tip --pack cap` — CAP best practice tips
- `sap-devs sync` — refresh with latest CAP release notes and dynamic content
```

This tells the AI: before searching the web for CAP information, run `sap-devs resources --pack cap` to get a curated list of authoritative sources. The AI can execute these commands in its terminal and pipe the output back into its context.

For a complete example see [`content/packs/cap/context.md`](../content/packs/cap/context.md).

### Guidelines for Writing Agent Instructions

- List commands the AI can actually run in a terminal without side effects.
- Prefer `--pack <id>` flags so the AI gets targeted results.
- Include `sap-devs sync` so the AI knows how to refresh stale dynamic content.
- Keep the section short — 3–6 bullet points is enough. Long agent instruction blocks eat into the budget for actual content.

---

## Pack Author Guidance

`format` defaults to `"markdown"`. Both `format="markdown"` and `format="text"` pass the response through an HTML parser. **Always set `format="raw"` for any non-HTML source** — plain text files, JSON endpoints, RSS feeds. Passing non-HTML through the parser is safe (the parser is lenient) but may produce garbled or sparse output.

Use `selector` to scope conversion to the main content area of a page and exclude nav bars, sidebars, and footers. Common values:

| Site type | Recommended selector |
| --- | --- |
| Generic (try first) | `main` |
| Article / blog | `article` |
| VitePress docs | `main` |
| Role-based | `[role="main"]` |

If `selector` matches nothing, the full page body is used and a warning is printed to stderr. Test your selector by running `sap-devs sync --force` and inspecting the expanded file.

---

## Testing a New Marker Locally

Use dev mode (`SAP_DEVS_DEV=1`) to work against the local `./content/` tree instead of the cache. This means changes to `content/packs/cap/context.md` are picked up immediately without a real sync.

```bash
# Force a full sync to trigger marker expansion
SAP_DEVS_DEV=1 go run . sync --force

# Inspect the expanded output
# Windows:
#   %LOCALAPPDATA%\sap-devs\cache\official\content\packs\cap\context.expanded.md
# Linux/macOS:
#   ~/.cache/sap-devs/official/content/packs/cap/context.expanded.md

# Dry-run inject to see what would be written to AI tool configs
SAP_DEVS_DEV=1 go run . inject --dry-run --no-sync

# Dry-run inject with per-adapter token stats
SAP_DEVS_DEV=1 go run . inject --dry-run --no-sync --stats
```

### Typical iteration loop

1. Edit `context.md` to add or adjust a marker.
2. Run `SAP_DEVS_DEV=1 go run . sync --force` to expand the marker.
3. Open `context.expanded.md` in the cache and verify the fetched content looks right.
4. Run `SAP_DEVS_DEV=1 go run . inject --dry-run --no-sync --stats` to confirm the expanded content appears in the rendered output at a sensible length and check the per-adapter token count.
5. Adjust `max_lines` or `max_tokens` and repeat from step 2 if the content is too long or too short.

### Checking for warnings

Sync warnings (missing `url`, conflicting truncation flags, failed fetches) are printed to stderr. Run with output visible:

```bash
SAP_DEVS_DEV=1 go run . sync --force 2>&1 | grep -i warn
```

No output means no warnings. A clean sync with all markers expanded is the goal before committing.

### Build check

After editing any Go source alongside content changes, confirm nothing is broken:

```bash
go build ./...
go vet ./...
```

On Windows, `go test` is blocked by Windows Defender in the `.config` path — use `go build` and `go vet` locally; the CI pipeline on `ubuntu-latest` is the authoritative test runner.
