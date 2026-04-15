# HTML Content Conversion for sync:fetch Markers

**Date:** 2026-04-15
**Status:** Approved

## Problem

`sync:fetch` markers fetch URLs verbatim and store the raw response body in `context.expanded.md`. For HTML pages (the majority of documentation sources), this produces unusable output — full HTML including `<head>`, nav, sidebars, scripts, and footers is injected into the AI's context window.

## Goal

Allow pack authors to control how fetched content is processed before it is stored, with sensible defaults that work for modern documentation sites out of the box.

## Marker Syntax Changes

Two new optional attributes are added to the `sync:fetch` marker:

| Attribute | Values | Default | Description |
|---|---|---|---|
| `format` | `raw`, `text`, `markdown` | `markdown` | How to process the fetched response body |
| `selector` | any CSS selector string | — (whole page) | DOM element to extract before conversion |

`selector` is ignored when `format="raw"`.

### Examples

```
<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" format="markdown" selector="main" max_lines="1000" label="CAP Release Notes" -->
```

```
<!-- sync:fetch url="https://example.com/feed.txt" format="raw" max_lines="50" label="Status Feed" -->
```

## Architecture

### Approach

Conversion, DOM selection, and truncation form a pipeline that must run in order: **select → convert → truncate**. All three steps live inside `FetchMarker` in `internal/sync/marker.go`, with conversion extracted into a standalone `convertContent` helper for independent testability.

### `Marker` struct additions

```go
Format   string // "raw" | "text" | "markdown"; default "markdown"
Selector string // CSS selector; empty = whole body
```

`ScanMarkers` parses both new attributes. Unknown `format` values log a warning and fall back to `"markdown"`.

### `convertContent(body, format, selector string) (string, error)`

| format | behaviour |
|---|---|
| `raw` | returns body unchanged; selector ignored |
| `text` | parses HTML with `golang.org/x/net/html`, extracts text nodes, strips tags |
| `markdown` | optionally scopes to selector via `cascadia`, converts scoped HTML with `html-to-markdown/v2` |

### `FetchMarker` update

After reading the response body and before truncation:

```
body → convertContent(body, m.Format, m.Selector) → truncate
```

The existing truncation block in `FetchMarker` must execute on the output of `convertContent`, not on the raw body. This requires moving the `truncateLines`/`truncateTokens` calls to after the `convertContent` call.

### New dependencies

- `github.com/JohannesKaufmann/html-to-markdown/v2` — HTML-to-Markdown conversion
- `github.com/andybalholm/cascadia` — CSS selector engine (used alongside `golang.org/x/net/html`)

Both are lightweight with no significant transitive dependencies.

## Error Handling

All failures are best-effort — they log a warning to stderr and fall back gracefully. Sync never aborts due to a conversion error.

| Condition | Behaviour |
|---|---|
| `selector` matches no elements | Warn: `sync:fetch selector "..." matched no elements — using full body`; use full body |
| Invalid CSS selector syntax | Warn and use full body |
| Unknown `format` value | Warn: `sync:fetch unknown format "..." — defaulting to markdown`; use `markdown` |
| HTML parse error | `golang.org/x/net/html` is lenient; no error expected |
| Conversion error from `html-to-markdown` | Treat as fetch failure; preserve cached expansion |
| Non-HTML body passed to `text` or `markdown` format | `golang.org/x/net/html` is lenient and will parse it; return whatever partial output is produced with a warning |

## Content Updates

### `content/packs/cap/context.md`

Update the existing `sync:fetch` marker to use the new attributes:

```
<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" format="markdown" selector="main" max_lines="1000" label="CAP Release Notes (feb26)" -->
```

### `docs/content-authoring.md`

- Add `format` and `selector` to the attribute reference table
- Update all examples to use `max_lines="1000"`
- Add note that `selector` is ignored for `format="raw"`
- Update the recommended limits table default example

## Testing

Unit tests in `internal/sync/marker_test.go`:

- `convertContent` with `format="raw"` — body unchanged, selector ignored
- `convertContent` with `format="text"` — HTML tags stripped, text nodes preserved
- `convertContent` with `format="markdown"` — HTML converted to Markdown
- `convertContent` with `selector` matching an element — only that element's content converted
- `convertContent` with selector matching nothing — falls back to full body, returns warning
- `convertContent` with invalid CSS selector — falls back to full body, returns warning
- `ScanMarkers` parses `format` and `selector` attributes correctly
- `ScanMarkers` warns on unknown `format` value and defaults to `markdown`

## Pack Author Guidance

Pack authors **must** set `format="raw"` for any non-HTML target (plain text files, JSON endpoints, RSS feeds). Both `format="markdown"` and `format="text"` pass the response through an HTML parser — a non-HTML body may produce garbled or empty output. The content-authoring docs must make this explicit for both formats, not just `markdown`.

## Out of Scope

- Auto-detection of format from HTTP `Content-Type` header (may revisit if useful)
- Per-adapter format overrides
- Multiple selectors / selector fallback chains
