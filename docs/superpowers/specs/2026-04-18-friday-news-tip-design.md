# Friday News Tip Override — Design Spec

**Date:** 2026-04-18
**Status:** Approved
**Project:** sap-devs-cli

---

## Overview

On Fridays, `sap-devs tip` overrides the normal daily tip with the latest SAP Developer News episode fetched live from YouTube RSS. If the fetch fails, a static nudge with the playlist URL is shown instead. All existing tip flags (`--plain`, `--markdown`, `--new`) continue to work unchanged.

---

## Behaviour

| Condition | Output |
| --- | --- |
| Not Friday | Normal tip selection (no change) |
| Friday, fetch succeeds | Live tip: episode title + watch URL + trimmed description |
| Friday, fetch fails or returns empty | Static tip: hardcoded nudge + playlist watch URL |
| Friday, `--new` flag or `SAP_DEVS_DEV=1` | Normal tip selection (override bypassed) |

The override is bypassed when `useRandom` is true — `--new` and dev mode both set this — so users can always get a regular tip by passing `--new`.

---

## Architecture

All logic lives in `cmd/tip.go`. No changes to `internal/content` or the pack data model.

### New function: `fridayNewsOverride() *content.Tip`

```
fridayNewsOverride()
  ├─ time.Now().Weekday() != time.Friday → return nil
  ├─ youtube.FetchPlaylist(newsPlaylistRSS) fails or empty → return staticFridayTip()
  └─ success → return formatFridayTip(episodes[0])
```

Returns `nil` when no override applies; caller falls through to normal `SelectTip`.

### New function: `formatFridayTip(ep youtube.Episode) *content.Tip`

Pure function — no I/O, fully testable.

- `Title`: `"SAP Developer News — " + ep.Title`
- `Content`: `ep.URL + "\n\n" + trimmedDescription`
- Description is trimmed to ≤280 bytes; a `…` suffix is appended when truncated
- When `ep.Description` is empty, content is `ep.URL` only (no trailing blank line)

### Static fallback: `staticFridayTip() *content.Tip`

Hardcoded, no network call:

- `Title`: `"It's Friday — SAP Developer News is out!"`
- `Content`: `"Watch the latest episode:\n" + newsPlaylistURL`

### Call site in `tipCmd.RunE`

```go
if !useRandom {
    if tip := fridayNewsOverride(); tip != nil {
        // render and return, same path as normal tip
    }
}
// existing SelectTip logic unchanged
```

---

## Constants

`newsPlaylistURL` is added to `cmd/news.go` alongside the existing constants:

```go
newsPlaylistURL = "https://www.youtube.com/playlist?list=PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg"
```

`newsPlaylistRSS` (already defined in `cmd/news.go`) is reused for the live fetch — no duplication.

---

## Error Handling

`fridayNewsOverride` never returns an error. Any fetch failure silently produces the static fallback. This keeps `tipCmd.RunE` clean and ensures `sap-devs tip` always produces output on Fridays.

---

## Testing

Tests live in `cmd/tip_test.go` (package `cmd`, so unexported helpers are accessible).

| Test | What it verifies |
| --- | --- |
| `TestFormatFridayTip_ShortDescription` | Short description appears in full in Content |
| `TestFormatFridayTip_LongDescriptionTrimmed` | Description >280 bytes is trimmed with `…` |
| `TestFormatFridayTip_EmptyDescription` | Empty description → Content is URL only, no trailing blank line |

`fridayNewsOverride` itself is not unit-tested (weekday dependency + live HTTP), consistent with the rest of `cmd/` which does not unit-test network paths.

---

## Files to Create / Modify

| File | Change |
| --- | --- |
| `cmd/news.go` | Add `newsPlaylistURL` constant |
| `cmd/tip.go` | Add `fridayNewsOverride()`, `formatFridayTip()`, `staticFridayTip()`; call override in `tipCmd.RunE` |
| `cmd/tip_test.go` | Add 3 tests for `formatFridayTip` |

---

## Out of Scope

- `pinned_weekday` field in the tip data model (superseded by live fetch approach)
- Per-pack configuration of the Friday override
- `--force-friday` flag for testing without changing the system clock
