# Tutorial Image URL Resolution

**Date:** 2026-04-18
**Status:** Approved

## Problem

Tutorial markdown contains relative image paths (`![alt](27.png)`). Glamour renders these as non-actionable text: `Image: alt → /27.png`. Users cannot view the images.

## Solution

Resolve relative image paths to full GitHub raw content URLs during parsing, so glamour renders the full URL inline — copyable in any terminal, and auto-linked in OSC8-capable terminals:

```
Image: Run Java → https://raw.githubusercontent.com/sap-tutorials/Tutorials/main/tutorials/java-hana-setup/27.png
```

## Design

### New function in `internal/tutorials/parser.go`

```go
const rawBaseURL = "https://raw.githubusercontent.com"

func ResolveImageURLs(content, repo, branch, slug string) string
```

- Regex matches `![...](path)` where `path` does not start with `http://` or `https://`
- Replaces path with `{rawBaseURL}/sap-tutorials/{repo}/{branch}/tutorials/{slug}/{path}`
- Strips leading `/` from the relative path before joining
- Leaves absolute URLs untouched
- Paths containing `../` are left unchanged (GitHub raw CDN does not resolve traversals)

### Integration

Called inside `Parse()` on each step's content after `normalizeOptionBlocks()`. `Parse()` gains a `branch` parameter (current signature: `Parse(md, slug, repo string)`).

Both V1 and V2 parsers benefit. Both static and TUI render paths benefit. Content is resolved at parse time and cached with full URLs on first `tutorial show`.

### Call-site changes

- `cmd/tutorials.go` — pass `branch` to `Parse()` (already available in scope)
- `internal/tutorials/parser_test.go` — update existing test calls with new `branch` argument

### Handled image path formats

| Source | Resolved |
|--------|----------|
| `![alt](27.png)` | `![alt](https://raw.githubusercontent.com/sap-tutorials/{repo}/{branch}/tutorials/{slug}/27.png)` |
| `![alt](/27.png)` | Same (leading `/` stripped) |
| `![alt](images/foo.png)` | `...tutorials/{slug}/images/foo.png` |
| `![alt](../sibling/x.png)` | Unchanged (traversal not resolved) |
| `![alt](https://example.com/x.png)` | Unchanged |

### Scope

- ~15 lines added to `parser.go` for `ResolveImageURLs` + `rawBaseURL` constant
- One-line signature change to `Parse()` and `ParseFrontmatterOnly` (adds `branch`)
- One-line change in `cmd/tutorials.go` to pass `branch`
- Test updates in `parser_test.go` (mechanical — add `branch` arg to 5 call sites)
- No new files, no new dependencies

## Alternatives Considered

1. **Pre-process at each render site** — Two call sites to maintain, easy to miss one.
2. **Custom glamour image renderer** — Overkill for URL rewriting; glamour's extension API is limited.
