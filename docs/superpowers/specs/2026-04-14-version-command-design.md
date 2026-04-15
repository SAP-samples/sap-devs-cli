# Version Command Enhancement — Design Spec

**Date:** 2026-04-14  
**Status:** Approved

## Overview

Improve `sap-devs version` to be more useful for both end users and CLI developers. Add a `--verbose` flag for rich build metadata, and always print a dev-build explanation when the version is `"dev"`.

## Goals

- Default output stays a single line (scriptable, no regression)
- `dev` builds print an explanatory note so users understand why they see `"dev"`
- `--verbose` / `-v` shows Go version, OS, and arch for bug reports and diagnostics

## Non-Goals

- No new packages or abstractions
- No changes to other commands
- No build-date injection (not currently in the build pipeline)

## Design

### File

All changes are confined to `cmd/version.go`.

### Default output

No flags — print `Version` on one line, then if `Version == "dev"` write a hint to **stderr**:

```
dev
(dev build: built without -ldflags version injection — auto-update is disabled)
```

For a real build:

```
v0.0.1-132-g666c39c
```

The hint is on stderr so scripts that parse `sap-devs version` are unaffected.

### Verbose output (`--verbose` / `-v`)

Print a labelled multi-line block to stdout. For a real build:

```
sap-devs v0.0.1-132-g666c39c
  go:      go1.22.3
  os/arch: windows/amd64
```

For a dev build:

```
sap-devs dev
  go:      go1.22.3
  os/arch: windows/amd64
(dev build: built without -ldflags version injection — auto-update is disabled)
```

The dev hint is still on stderr in verbose mode.

### Implementation

- Add a `--verbose` / `-v` local flag to `versionCmd` via `versionCmd.Flags().BoolVarP` (bound to a package-level `var verbose bool`, consistent with other flags in the codebase)
- Read the flag inside the `Run` closure via the bound variable (not `cmd.Flags().GetBool`)
- The dev hint (`fmt.Fprintf(os.Stderr, ...)`) is written after the stdout block so stream ordering is deterministic
- Import `runtime` for `runtime.Version()`, `runtime.GOOS`, `runtime.GOARCH`; import `os` for `os.Stderr`
- No new packages; no changes outside `cmd/version.go`

### Error handling

`runtime` calls never fail — no error paths needed.

### Testing

- `go build ./...` + `go vet ./...` locally (Windows Defender blocks test binaries)
- CI (`go test ./...` on ubuntu-latest) is authoritative
- Manual smoke test: run `.\sap-devs.exe version` and `.\sap-devs.exe version --verbose` on a dev build and a version-tagged build
