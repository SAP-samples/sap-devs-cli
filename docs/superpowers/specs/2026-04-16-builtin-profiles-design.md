# Built-in Profiles (`all` and `minimal`) Design

**Date:** 2026-04-16

---

## Goal

Add two hardcoded built-in profiles — `all` and `minimal` — that appear in `sap-devs profile list`, can be selected with `sap-devs profile set`, and have well-defined pack inclusion behaviour without requiring any YAML file on disk.

---

## Background

Profiles are currently YAML files in `content/profiles/`. `LoadProfiles` reads all `*.yaml` files from the configured content layers; `FindProfile` iterates that list by ID. There is no concept of a built-in profile.

Two profiles are needed that cannot be expressed cleanly as files:

- **`all`** — include every pack from every content layer. A static `all.yaml` listing pack IDs would drift out of sync whenever a new pack is added.
- **`minimal`** — include base packs only. A static `minimal.yaml` with an empty `packs` list would be confusing and could be accidentally shadowed by a user/project layer file.

Both profiles must appear in `profile list` and be selectable via `profile set` exactly like file-backed profiles.

---

## Approach

Inject built-in profiles inside `ContentLoader` so all commands work without modification. Reserved IDs (`all`, `minimal`) cannot be shadowed by file-backed profiles.

---

## Design

### 1. Built-in profile objects (`internal/content/profile.go`)

A new package-level function returns the two built-in profiles:

```go
func builtinProfiles() []*Profile {
    return []*Profile{
        {
            ID:          "all",
            Name:        "All Packs",
            Description: "All available packs across every content layer",
        },
        {
            ID:          "minimal",
            Name:        "Minimal",
            Description: "Base layer only — shared SAP ecosystem entry points, no technology-specific packs",
        },
    }
}
```

Both have nil `Packs` and nil `TipTags` (no tip-tag preference; tip selection falls back to generic tips).

A package-level set of reserved IDs is used for guard checks:

```go
var reservedProfileIDs = map[string]bool{"all": true, "minimal": true}
```

### 2. `LoadProfiles` — append built-ins, built-ins win on ID conflict

After loading file-backed profiles, built-ins are appended. Any file-backed profile whose ID matches a reserved ID is silently dropped:

```go
func LoadProfiles(profilesDir string) ([]*Profile, error) {
    // ... existing file loading ...

    // Filter out any file-backed profiles that shadow a reserved built-in ID.
    var filtered []*Profile
    for _, p := range profiles {
        if !reservedProfileIDs[p.ID] {
            filtered = append(filtered, p)
        }
    }
    return append(filtered, builtinProfiles()...), nil
}
```

Built-ins appear at the end of the list — file-backed profiles (more persona-specific) appear first.

### 3. `FindProfile` — check reserved IDs first

```go
func (cl *ContentLoader) FindProfile(id string) (*Profile, error) {
    if reservedProfileIDs[id] {
        for _, p := range builtinProfiles() {
            if p.ID == id {
                return p, nil
            }
        }
    }
    // ... existing file-backed lookup ...
}
```

Bypasses file I/O entirely for `all` and `minimal`.

### 4. `LoadPacks` — `minimal` returns base packs only, `all` unchanged

**`all`:** No change needed. `LoadPacks` already returns every pack from every layer. `ApplyWeights` with an empty `Packs` list falls back to each pack's default `Weight`. Base packs are pinned first by the existing partition introduced in the base layer feature.

**`minimal`:** One targeted check after the base/nonBase partition:

```go
if profile != nil && profile.ID == "minimal" {
    return base, nil
}
return append(base, nonBase...), nil
```

Since base packs are exempt from `TrimPacks` byte-budget enforcement, adapters with a `max_tokens` budget still receive full base content when `minimal` is active.

### 5. `profile show` — built-in description instead of empty pack-weight table

`profile show` currently prints "Pack weights:" followed by the `p.Packs` list. For built-ins the list is nil, leaving a header with no items.

Guard on `len(p.Packs) == 0` (only built-ins have an empty list; all file-backed profiles have at least one pack):

```go
if len(p.Packs) == 0 {
    fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "profile.show.builtin_note"))
} else {
    // existing pack weight table
}
```

New i18n key `profile.show.builtin_note` → `"Built-in profile — pack selection is determined at runtime, not by a fixed list."` (same string works for both `all` and `minimal`).

### 6. Documentation updates

**`docs/content/content-guide.md`** — add a "Built-in Profiles" subsection under the Profiles section:

- Describes `all` (every pack, weight order) and `minimal` (base layer only).
- States that reserved IDs `all` and `minimal` cannot be defined in YAML; any file with those IDs is silently ignored.
- Notes that `profile list` shows them alongside file-backed profiles.

**`docs/content-authoring.md`** — add a note in the Base Layer section:

- The `minimal` profile is equivalent to the base layer only. Keeping base pack content lean directly limits the token footprint of the `minimal` profile.

---

## File Changelist

| File | Change |
|---|---|
| `internal/content/profile.go` | Add `builtinProfiles()`, `reservedProfileIDs`; update `LoadProfiles` to filter reserved IDs and append built-ins; update `FindProfile` to short-circuit on reserved IDs |
| `internal/content/loader.go` | Add `minimal` guard in `LoadPacks` after base/nonBase partition |
| `cmd/profile.go` | Add built-in guard in `profile show` to print `profile.show.builtin_note` instead of empty pack-weight table |
| `internal/i18n/catalogs/en.json` | Add `profile.show.builtin_note` key |
| `internal/i18n/catalogs/de.json` | Add `profile.show.builtin_note` key (German) |
| `docs/content/content-guide.md` | Add Built-in Profiles subsection under Profiles |
| `docs/content-authoring.md` | Add note in Base Layer section about `minimal` |
| `internal/content/profile_test.go` | New/extended: built-in tests (see Testing section) |
| `internal/content/loader_test.go` | Extended: `minimal` and `all` pack selection tests |

---

## Testing

All tests in `internal/content/`. `go test` is blocked locally by Windows Defender; CI on `ubuntu-latest` is the authoritative runner.

### `internal/content/profile_test.go`

- `TestBuiltinProfiles_ContainsAllAndMinimal` — `builtinProfiles()` returns exactly two entries with IDs `"all"` and `"minimal"`
- `TestLoadProfiles_IncludesBuiltins` — loading from a dir with only `cap-developer.yaml` returns 3 profiles total (file-backed + 2 built-ins)
- `TestLoadProfiles_BuiltinWinsOverFile` — a file named `all.yaml` on disk is dropped; the built-in `all` profile survives with its hardcoded Name/Description
- `TestFindProfile_ReturnsBuiltinForAll` — `FindProfile("all")` returns non-nil with no files on disk
- `TestFindProfile_ReturnsBuiltinForMinimal` — `FindProfile("minimal")` returns non-nil with no files on disk

### `internal/content/loader_test.go`

- `TestLoadPacks_MinimalProfile_BasePacksOnly` — with one base pack and one non-base pack loaded, `minimal` profile returns only the base pack
- `TestLoadPacks_AllProfile_AllPacksReturned` — `all` profile returns both base and non-base packs in weight order, base first

---

## Constraints

- Built-in profiles have nil `TipTags` — `sap-devs tip` with these profiles active uses unfiltered tip selection.
- `profile show` uses a single `builtin_note` i18n string for both `all` and `minimal` — the profile `Description` field already conveys the specific behaviour.
- `overlaps` deduplication in `TrimPacks` is unaffected — `minimal` bypasses `TrimPacks` entirely for non-base packs by returning early in `LoadPacks`, so no dedup interaction occurs.
