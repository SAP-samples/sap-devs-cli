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

Profile paths are resolved using `os.UserHomeDir()` on all platforms.

| Platform | Profiles checked (in order) |
| --- | --- |
| Linux | `~/.zshrc`, `~/.bashrc`, `~/.bash_profile`, `~/.zprofile` |
| macOS | `~/.zshrc`, `~/.bashrc`, `~/.bash_profile`, `~/.zprofile` |
| Windows | `~/Documents/PowerShell/Microsoft.PowerShell_profile.ps1`, `~/.bashrc`, `~/.bash_profile` |

`~` denotes the path returned by `os.UserHomeDir()`. Only profiles that **already exist** are modified. If no profiles are found, `Add` returns an error; `Remove` returns with no results (not an error).

### API

```go
// Result describes what happened to a single profile file.
type Result struct {
    Path    string
    Updated bool // false = already present (Add) or line not found (Remove)
}

// Add appends comment and line to every existing profile that does not
// already contain line as a complete line. Returns one Result per
// candidate profile found on disk.
func Add(line, comment string) ([]Result, error)

// Remove strips every occurrence of line (full-line match) and any
// immediately preceding line equal to comment from all existing profiles.
// Returns one Result per candidate profile found on disk (Updated=true
// if the file was changed, false if the line was not present).
func Remove(line, comment string) ([]Result, error)
```

### Behaviour Details

**Add:**

1. Resolve candidate profile paths for the current platform using `os.UserHomeDir()`.
2. Skip paths that do not exist on disk.
3. For each existing path: scan lines; if any line is exactly equal to `line` (full-line equality, trimming trailing newline), mark `Updated: false` and skip.
4. Otherwise append `\n<comment>\n<line>\n` and mark `Updated: true`.
5. If no existing profiles were found, return an error with manual steps. `line` is assumed to be a single printable line; format it with `%q` in the error string: `fmt.Sprintf("no shell profile found; add %q to your shell profile manually", line)`.
6. If any file write fails, collect the error. After processing all profiles, return all results accumulated so far plus a combined error using `errors.Join`.

**Remove:**

1. Resolve candidate profile paths for the current platform using `os.UserHomeDir()`.
2. For each existing path, whether or not it contains the target: include it in results (Updated=false by default).
3. Scan lines in a single pass; drop any line exactly equal to `line` and any immediately preceding line exactly equal to `comment` (i.e., only the comment that directly precedes the hook line is removed — a solitary `comment` line with no following `line` is left in place).
4. If the file changed, rewrite it and mark `Updated: true`.
5. If any file write fails, collect the error. After processing all profiles, return all results plus a combined error using `errors.Join`.

**Orphaned comment lines:** If a profile contains `comment` without an immediately following `line` (e.g., from a prior partial removal or double-append followed by a partial uninstall), that comment line is left in place. This is accepted behaviour; the implementation does not need to hunt for orphaned comments.

**Duplicate detection and removal use the same strategy — full-line equality** (after trimming the trailing newline). This ensures that a comment line such as `# runs sap-devs tip` does not trigger a false-positive match and that `Add` / `Remove` are symmetric.

---

## CLI: `sap-devs tip install` / `sap-devs tip uninstall`

Both are subcommands registered under the existing `tip` command in `cmd/tip.go`.

### `tip install`

Calls `shellhook.Add("sap-devs tip", "# SAP developer tips")`.

Prints a summary line per result:

```text
✓ Updated ~/.zshrc
✓ Updated ~/.bash_profile
  ~/.bashrc — already configured
```

If no profiles were found (error returned, results empty), prints:

```text
No shell profile found. Add this line to your shell profile manually:
  sap-devs tip
```

### `tip uninstall`

Calls `shellhook.Remove("sap-devs tip", "# SAP developer tips")`.

Prints a summary line per result:

```text
✓ Removed from ~/.zshrc
  ~/.bash_profile — not configured
```

If no results have `Updated: true`, also prints:

```text
'sap-devs tip' was not found in any shell profile.
```

---

## Changes to `cmd/init.go`

Remove the `addShellHook()` function and replace its call site with `shellhook.Add("sap-devs tip", "# SAP developer tips")`.

**Behaviour change:** The existing `addShellHook()` stops at the first existing profile. After this change, `init` will write to **all** existing profiles, consistent with the behaviour of `tip install`. This is intentional — users with multiple shells (e.g., zsh and Git Bash on the same machine) get the tip in all of them.

---

## Error Handling

- File read/write errors are collected across all profiles.
- After processing all profiles, any errors are combined with `errors.Join` and returned alongside the results accumulated so far.
- Callers receive partial results and can report what succeeded before surfacing the error.

---

## Testing

Unit tests in `internal/shellhook/shellhook_test.go` using a temporary directory substituted for home:

| Case | Add | Remove |
| --- | --- | --- |
| Single profile exists, line absent | appends, Updated=true | Updated=false (not found) |
| Single profile exists, line present | skips, Updated=false | removes, Updated=true |
| Line present as substring of another line | appends (no false positive), Updated=true | leaves file unchanged, Updated=false |
| Multiple profiles exist | updates all absent ones | removes from all that contain line |
| No profiles exist | returns error, empty results | returns empty results, no error |
| Comment+hook present; orphaned comment present | skips, Updated=false | removes all comment+hook pairs, leaves orphaned comment, Updated=true |
| Multiple comment+hook blocks in one file | skips, Updated=false | removes all pairs, Updated=true |
| Windows PowerShell profile path | appends, Updated=true | removes, Updated=true |

---

## File Checklist

| Action | Path |
| --- | --- |
| Create | `internal/shellhook/shellhook.go` |
| Create | `internal/shellhook/shellhook_test.go` |
| Modify | `cmd/tip.go` — add `install` and `uninstall` subcommands |
| Modify | `cmd/init.go` — replace `addShellHook()` with `shellhook.Add(...)` |
| Modify | `docs/user/user-guide.md` — document `tip install` / `tip uninstall` |
