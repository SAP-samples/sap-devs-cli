# Friday Developer News Hook — Design Spec

**Date:** 2026-04-18
**Status:** Approved
**Project:** sap-devs-cli

---

## Overview

A new `sap-devs news hook` subcommand prints a Friday reminder message on Fridays and exits silently on all other days. A new hook entry in `content/packs/base/hook.yaml` wires it into Claude Code's `sessionStart` event. Users install it with `sap-devs hook install community/friday-developer-news`.

---

## Behaviour

| Condition | Output |
| --- | --- |
| Not Friday | No output, exit 0 |
| Friday | Reminder message printed to stdout, exit 0 |

The command never errors. Any day-of-week check failure (which cannot happen with `time.Weekday`) is not a concern.

---

## Architecture

### New subcommand: `sap-devs news hook`

Added to `cmd/news.go` as a subcommand of `newsCmd`.

```
newsHookCmd.RunE
  ├─ fridayHookMessage(time.Now().Weekday()) → ""   → return nil (no output)
  └─ fridayHookMessage(time.Now().Weekday()) → msg  → fmt.Fprintln(cmd.OutOrStdout(), msg)
```

#### New helper: `fridayHookMessage(day time.Weekday) string`

Pure function — no I/O, fully testable.

- Returns the reminder message string when `day == time.Friday`
- Returns `""` on all other days

**Message text (Friday):**

```
📺 It's Friday — a new SAP Developer News episode is likely out!

Would you like me to open the latest episode? Run `sap-devs news latest` or just say yes.
```

### New hook entry: `content/packs/base/hook.yaml`

Added alongside the existing `tip-on-session-start` entry:

```yaml
- id: community/friday-developer-news
  event: sessionStart
  command: "sap-devs news hook"
  tools:
    - claude-code
```

No new pack, no new adapter, no schema changes required.

---

## Installation

```bash
sap-devs hook install community/friday-developer-news
```

This follows the existing `hook install` flow:
1. Detects Claude Code as an installed adapter with `hook_config`
2. Appends `sap-devs news hook` to `~/.claude/settings.json` under `hooks.SessionStart`
3. Idempotent — skips if already present

---

## Error Handling

`newsHookCmd.RunE` never returns an error. The command always exits 0 regardless of day.

---

## Testing

Tests live in `cmd/news_test.go` (package `cmd`, so unexported helpers are accessible).

| Test | What it verifies |
| --- | --- |
| `TestFridayHookMessage_Friday` | Returns non-empty string on `time.Friday` |
| `TestFridayHookMessage_NotFriday` | Returns `""` on every non-Friday weekday |

`newsHookCmd.RunE` itself is not unit-tested directly — the logic is fully covered by `fridayHookMessage` tests.

---

## Files to Create / Modify

| File | Change |
| --- | --- |
| `cmd/news.go` | Add `newsHookCmd` subcommand and `fridayHookMessage` helper; register with `newsCmd` |
| `cmd/news_test.go` | New file — 2 tests for `fridayHookMessage` |
| `content/packs/base/hook.yaml` | Add `community/friday-developer-news` hook entry |

---

## Out of Scope

- `once_per_day` deduplication (not needed — always fires on Fridays)
- Live episode fetch (static message; AI runs `sap-devs news latest` on request)
- Support for adapters other than `claude-code` (none currently have `hook_config`)
- Weekday configuration (Friday is hardcoded — the show ships on Fridays)
