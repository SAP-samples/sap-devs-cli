# Content Authoring Guide

This guide covers how to write `context.md` files that use dynamic markers, how to reason about token budgets, and how to test your changes locally before syncing.

For the full pack structure reference (adapters, profiles, translations), see [docs/content/content-guide.md](content/content-guide.md).

---

## Pack Directory Structure

Each pack lives in `content/packs/<pack-id>/`. All files are optional except `pack.yaml`.

```
content/packs/<pack-id>/
├── pack.yaml          # Pack metadata (id, name, tags, weight, profiles)
├── context.md         # AI context text injected into coding tools
├── context.<lang>.md  # Localised AI context (e.g. context.de.md)
├── preamble.md        # AI preamble (base pack only)
├── constraints.md     # Numbered constraint list — things agents should NOT do
├── constraints.<lang>.md  # Localised constraints
├── tips.md            # H2-delimited tips shown by `sap-devs tip`
├── tips.<lang>.md     # Localised tips
├── tools.yaml         # Tool version requirements checked by `sap-devs doctor`
├── resources.yaml     # Curated links shown by `sap-devs resources`
├── mcp.yaml           # MCP server definitions wired by `sap-devs mcp install`
├── hook.yaml          # Hook commands wired by `sap-devs hook install`
└── samples.yaml       # Canonical code sample references shown by `sap-devs samples`
```

Key points:

- `context.md` is the primary AI context file. Keep it concise — every line you add is injected into the AI's context window on every `sap-devs inject` run.
- `tips.md` tips are shown one at a time by `sap-devs tip`; they are not injected wholesale, so they can be longer.
- `tools.yaml` and `resources.yaml` are structured YAML lists; see [docs/content/content-guide.md](content/content-guide.md) for their schemas.
- `samples.yaml` lists canonical code sample references (GitHub file URLs). Samples with `inject: true` are included in the AI context as a "Canonical Patterns" table.

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

### preamble.md

A base pack may include an optional `preamble.md` file. When present, its content is rendered **before all pack `context.md` content** — immediately after the dynamic runtime section.

**Rendered output order:**

1. `# SAP Developer Context` header + profile line
2. `## sap-devs Runtime Context` (dynamic — version, packs, available commands)
3. **Preamble** — from `base/preamble.md` (this file)
4. **`## Constraints`** — consolidated from all active packs' `constraints.md`
5. Base pack `context.md`
6. Technology pack `context.md` files (cap, abap, btp-core, …)

*Implementation note:* The preamble and `ContextMD` are emitted in two separate loops. The base pack's `ContextMD` is still rendered in the second loop with all other packs — not in the preamble loop. This prevents double-emission.

**Authoring constraints:**

- Keep it ≤ 3 lines — it is injected into every AI tool config on every `sap-devs inject` run.
- No Markdown headings — it appears before pack content and must not create heading hierarchy collisions.
- No locale variants — the preamble is intentionally language-neutral (command names don't translate).

**Token budget:** The preamble is exempt from adapter token-budget trimming (same as base pack `ContextMD`). Every byte is unconditionally injected. Keep it short.

**Layer override:** Only the official base pack's `preamble.md` is used. User, company, and project layer packs cannot override or augment the preamble. The render loop guards on `Pack.Base == true`; only base packs have their `PreambleMD` emitted. An additive pack targeting `id: base` also cannot modify `PreambleMD` — `MergeWith` preserves scalar fields from the base pack.

---

**Authoring contract:** Keep base pack content minimal. Every byte in a base pack is consumed in every inject, for every user, regardless of their configured token budget.

> **`minimal` profile and base packs:** The `minimal` built-in profile includes base packs only. Keeping base pack content lean is therefore a direct budget lever for users who select `minimal` — every extra byte in a base pack is added to the `minimal` profile footprint.

---

## Constraints

A pack may include an optional `constraints.md` file. Its content is a numbered markdown list of things AI agents should NOT do when working with that pack's technology domain.

### Format

```markdown
1. Never write raw SQL — always use `cds.ql` or CQL
2. Never use `req.user` without a `@requires` annotation
```

No YAML, no frontmatter — raw numbered markdown. Each line is one constraint.

### Rendered output

All constraints from all active packs are consolidated into a single `## Constraints` section, placed after the preamble and before the first pack's `context.md` content.

### Localization

Two-step resolution: `constraints.{lang}.md` → `constraints.md`. Unlike `context.md`, there is no `constraints.expanded.md` step.

### Additive layers

`constraints.md` participates in additive merge the same way as `context.md`: company/user/project layer constraints are appended (or prepended, based on `additive_position`) to the official constraints.

### Authoring guidelines

- Keep each constraint to one line — they are rendered as a numbered list.
- Start each constraint with "Never" to make the prohibition clear.
- Include the correct alternative after "—" so agents know what to do instead.
- Universal constraints (e.g. credential storage) belong in the base pack's `constraints.md`.
- Technology-specific constraints belong in the domain pack.

---

## Editor Setup

For inline validation and autocomplete when editing pack YAML files, install the [YAML extension by Red Hat](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) in VS Code. Schema wiring is already configured in `.vscode/settings.json` — open any `pack.yaml`, `resources.yaml`, `tools.yaml`, `mcp.yaml`, or profile YAML file and you'll get field suggestions and error highlighting automatically.

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
| `constraints.md` | Your content is appended or prepended to the official constraints |
| `tips.md` | Both sets of tips are kept; yours are added in the configured position |
| `resources.yaml` | Entries with matching `id` replace the official entry; new IDs are appended |
| `tools.yaml` | Entries with matching `id` replace the official entry; new IDs are appended |
| `mcp.yaml` | Entries with matching `id` replace the official entry; new IDs are appended |
| `samples.yaml` | Entries with matching `id` replace the official entry; new IDs are appended |
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

> **Note:** The general "prefer `sap-devs` commands over web search" instruction lives in `content/packs/base/preamble.md` and is injected automatically into every profile. Per-pack `### Agent Instructions` sections should contain only pack-specific command hints — for example, `--pack cap` flag variants for the CAP pack. See `content/packs/base/preamble.md` for the canonical example.

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

| Field     | Type     | Description                                                            |
| --------- | -------- | ---------------------------------------------------------------------- |
| `id`      | string   | Unique identifier. Used by `sap-devs hook install <id>`.               |
| `event`   | string   | Tool-neutral event. Supported values: `sessionStart`.                  |
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

### Adding `hook_config` to an adapter YAML

To make a new AI tool's adapter support hook installation, add a `hook_config` block to its YAML in `content/adapters/<id>.yaml` alongside the existing `mcp_config`:

```yaml
hook_config:
  path: "~/.tool/settings.json"   # path to the tool's settings file (tilde expanded)
  format: json                     # "json" only for now
  key: "hooks.SessionStart"        # dot-separated JSON path to the hook array
```

The `key` field is a dot-separated path that `WriteHookConfig` navigates dynamically. For Claude Code, the value is `"hooks.SessionStart"`. Only adapters with a `hook_config` block can be targeted by `hook install`. Adapters without it are silently skipped.

---

## YouTube Content (`youtube.yaml`)

A pack may include an optional `youtube.yaml` file. It declares video sources from SAP's YouTube channels, which are fetched during sync and cached alongside the pack. Videos are browsable via the `sap-devs videos` command.

### `youtube.yaml` schema

```yaml
- id: sapdevs-main              # Unique identifier
  type: playlist                # 'playlist' or 'video'
  name: SAP Developers Channel  # Display name
  playlist_id: "PLk0..."        # For playlist type: YouTube playlist ID
  tags: [tutorial, sap]         # Optional tags for filtering
```

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `id` | string | yes | Unique identifier within the pack. Used by `sap-devs videos` queries. |
| `type` | string | yes | Either `playlist` or `video`. Determines whether to fetch a playlist or a single video. |
| `name` | string | yes | Display name shown in `sap-devs videos list` output. |
| `playlist_id` | string | required if `type: playlist` | YouTube playlist ID (e.g. `PLk0Iym...`). Required for playlist sources. |
| `video_id` | string | required if `type: video` | YouTube video ID (e.g. `dQw4w9WgXcQ`). Required for video sources. |
| `tags` | string[] | no | Tags for filtering in `sap-devs videos search`. Merged with API-provided tags. |

### Source types

**`type: playlist`** — fetches videos from a public YouTube playlist at sync time via RSS or YouTube Data API v3 (if authenticated). The playlist ID is extracted from YouTube URLs like `https://www.youtube.com/playlist?list=PLk0...`.

**`type: video`** — static reference to a single video. No API call is made during sync; the video ID is validated at fetch time.

### Example: youtube.yaml sources

```yaml
- id: sapdevs-tutorials
  type: playlist
  name: SAP Developers Tutorials
  playlist_id: PLk0Iym00000...
  tags: [tutorial, sap]

- id: sapdevs-cap-intro
  type: video
  name: CAP in 10 Minutes
  video_id: dQw4w9WgXcQ
  tags: [cap, intro]
```

### How videos are fetched and cached

During `sap-devs sync`, playlist sources are expanded:

1. The sync engine fetches the playlist's RSS feed (no authentication required), or uses the YouTube Data API v3 if an API key is configured.
2. Each video's metadata (title, duration, published date) is extracted.
3. Video data is cached as JSON at `~/.cache/sap-devs/youtube/<pack-id>/<source-id>.json`.
4. If the fetch fails, the previous cached data (if any) is preserved.

The `sap-devs videos` command reads the cached video data and allows browsing, searching, and opening videos in the browser.

### Token budget

YouTube metadata is not injected into the AI context window (unlike `context.md` content). Videos are only browsable via the CLI, so there is no token budget impact.

### Limitations

- **Playlists require public URLs.** Private playlists cannot be fetched.
- **RSS feeds may lag.** YouTube playlist RSS feeds are updated periodically; new videos may take a few hours to appear.
- **Static videos are not validated until use.** If a `video_id` is invalid, the error appears when the user tries to open it, not during sync.

---

## Discovery Center (`discovery.yaml`)

Each pack may include a `discovery.yaml` file that references curated SAP Discovery Center content. Unlike other pack YAML files (which are top-level arrays), `discovery.yaml` uses a top-level object:

```yaml
profile_filters:
  products: ["1006"]      # Discovery Center product IDs
  categories: ["appdev"]  # Category codes (appdev, intgn, dataanalytics, aicatg)
  focus_tags: ["4"]       # Focus tag IDs

missions:
  - id: 4327              # Integer — Discovery Center mission ID
    name: Develop a Full-Stack CAP Application
    featured: true         # Optional — appears first in listings

services:
  - id: 73554e5a-6885-...  # UUID — ServiceDetails ID from /servicecatalog/
    name: SAP Cloud Application Programming Model
    featured: true

guidance:
  - id: realize-application-dev-best-practices  # Slug — guidance node ID
    name: Application Development Best Practices
```

`profile_filters` controls automatic filtering when the user has an active profile. The filter values map to Discovery Center API filter parameters.

Schema: `content/schemas/discovery.schema.json`

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
