# Design: `tip install` / `tip uninstall` and `internal/shellhook`

**Date:** 2026-04-14
**Status:** Approved

---

## Overview

`sap-devs init` already offers to append `sap-devs tip` to the user's shell profile. This feature extracts that logic into a reusable `internal/shellhook` package and exposes it as `sap-devs tip install` / `sap-devs tip uninstall` so users who skipped the step during `init` can add or remove the hook at any time.

---

## Goals

- Allow users to add `sap-devs tip` to their shell startup outside of `init`.
- Allow users to remove the hook cleanly.
- Handle all common shell profiles across Linux, macOS, and Windows.
- Make the hook logic reusable for future commands (e.g., `doctor`).

## Non-Goals

- Writing to profiles that do not already exist (no profile creation).
- Supporting shells beyond zsh, bash, and PowerShell.
- WSL profiles (treated as a separate Linux environment).

---

## Package: `internal/shellhook`

### Responsibility

Append or remove a single-line shell hook (plus an optional comment) across all shell profile files found on the current platform.

### Profile Matrix

| Platform | Profiles checked (in order) |
|---|---|
| Linux | `~/.zshrc`, `~/.bashrc`, `~/.bash_profile`, `~/.zprofile` |
| macOS | `~/.zshrc`, `~/.bashrc`, `~/.bash_profile`, `~/.zprofile` |
| Windows | `%USERPROFILE%\Documents\PowerShell\Microsoft.PowerShell_profile.ps1`, `%USERPROFILE%\.bashrc`, `%USERPROFILE%\.bash_profile` |

Only profiles that **already exist** are modified. If no profiles are found, `Add` returns an error; `Remove` returns with no results (not an error).

### API

```go
// Result describes what happened to a single profile file.
type Result struct {
    Path    string
    Updated bool // false = already present (Add) or line not found (Remove)
}

// Add appends comment and line to every existing profile that does not
// already contain line. Returns one Result per candidate profile found.
func Add(line, comment string) ([]Result, error)

// Remove strips every occurrence of line (and an immediately preceding
// comment matching the injected comment) from all existing profiles.
// Returns one Result per profile where a match was found.
func Remove(line string) ([]Result, error)
```

### Behaviour Details

**Add:**
1. Resolve candidate profile paths for the current platform.
2. Skip paths that do not exist on disk.
3. For each existing path: if `line` is already present (exact substring match), mark `Updated: false` and skip.
4. Otherwise append `\n<comment>\n<line>\n` and mark `Updated: true`.
5. If no existing profiles were found, return an error indicating manual steps.

**Remove:**
1. Resolve candidate profile paths for the current platform.
2. For each existing path: scan lines, drop any line equal to `line` and any immediately preceding line matching `# SAP *`.
3. If the file changed, rewrite it and mark `Updated: true`.
4. No error if nothing was found.

---

## CLI: `sap-devs tip install` / `sap-devs tip uninstall`

Both are subcommands registered under the existing `tip` command in `cmd/tip.go`.

### `tip install`

Calls `shellhook.Add("sap-devs tip", "# SAP developer tips")`.

Prints a summary line per profile:

```
✓ Updated ~/.zshrc
✓ Updated ~/.bash_profile
  ~/.bashrc — already configured
```

If no profiles were found, prints a manual fallback message:

```
No shell profile found. Add this line to your shell profile manually:
  sap-devs tip
```

### `tip uninstall`

Calls `shellhook.Remove("sap-devs tip")`.

Prints a summary line per profile touched:

```
✓ Removed from ~/.zshrc
  ~/.bash_profile — not configured
```

If nothing was removed, prints:

```
'sap-devs tip' was not found in any shell profile.
```

---

## Changes to `cmd/init.go`

Remove the `addShellHook()` function and replace its call site with `shellhook.Add("sap-devs tip", "# SAP developer tips")`. The init step now benefits from the full platform-aware profile matrix automatically.

---

## Error Handling

- File read/write errors are returned to the caller and surfaced as CLI errors.
- Partial success (some profiles updated, one failed) reports the successful updates and returns the error for the failed one.

---

## Testing

Unit tests in `internal/shellhook/shellhook_test.go` using a temporary directory as home:

| Case | Add | Remove |
|---|---|---|
| Single profile exists, line absent | appends, Updated=true | no-op |
| Single profile exists, line present | skips, Updated=false | removes, Updated=true |
| Multiple profiles exist | updates all absent ones | removes from all |
| No profiles exist | returns error | returns empty results, no error |
| Windows PowerShell profile | appends, Updated=true | removes, Updated=true |

---

## File Checklist

| Action | Path |
|---|---|
| Create | `internal/shellhook/shellhook.go` |
| Create | `internal/shellhook/shellhook_test.go` |
| Modify | `cmd/tip.go` — add `install` and `uninstall` subcommands |
| Modify | `cmd/init.go` — replace `addShellHook()` with `shellhook.Add(...)` |
| Modify | `docs/user/user-guide.md` — document `tip install` / `tip uninstall` |
