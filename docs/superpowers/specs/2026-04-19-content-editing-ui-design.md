# Content Editing UI — Phase 1 Design

Interactive terminal-based YAML editor with schema-driven value help and validation for all content types.

## Commands

**Naming note:** `content` coexists with the existing `context` command (ephemeral scratch notes via `context add/list/clear`). The names are intentionally distinct — `content` manages structured YAML content files, `context` manages free-text session notes. The `content` noun accurately describes what this command operates on: the content layer system.

```text
sap-devs content
  ├── edit <file>     # open TUI editor
  ├── validate        # validate all content files against JSON schemas
  └── list            # show all content files across layers
```

## Scope

- **10 primary content types** (contributor-editable): pack.yaml, resources.yaml, influencers.yaml, event-types.yaml, event-instances.yaml, mcp.yaml, tools.yaml, hook.yaml, samples.yaml, known_errors.yaml
- 6 additional schemas exist (discovery, learning, paths, profile, tutorials, youtube) but are excluded from Phase 1 — these are reference/configuration files with cross-references to external APIs, not standalone content items that contributors typically hand-edit
- All 4 content layers: official, company, user, project
- Schema-driven value help (enums, formats, patterns, required fields, conditionals)
- Inline validation with error messages
- Hybrid list/form TUI using charmbracelet/huh on Bubbletea

## Architecture

### New packages

- `internal/schema/` — JSON Schema parser producing generic `FieldSpec` model
- `internal/editor/` — Bubbletea TUI with list view + huh form view
- `cmd/content.go` — `content` parent command with `edit`, `validate`, `list` subcommands

### Layer resolution for `content edit`

The editor auto-detects which layer to target based on the working directory:

| Context | Behavior |
| --- | --- |
| CWD is official repo checkout | Edit files in working tree (contributor mode) |
| CWD is company repo checkout | Edit files in working tree (contributor mode) |
| CWD has `.sap-devs/` | Edit project-layer files |
| Otherwise | Edit user-layer overrides in `~/.local/share/sap-devs/` |

**Detection heuristics:**

- Official repo: CWD contains `content/packs/` and git remote matches the official repo URL
- Company repo: git remote matches the URL from `sap-devs config company`
- Project layer: `.sap-devs/` directory exists in CWD

**Override editing UX:** When editing in user or project layer, the list view shows a merged view of all items across layers. Each item has a layer badge (official, company, user, project). Editing an inherited item auto-copies it to the target layer as an override. The written file only contains items belonging to that layer.

### `content edit` argument resolution

The `<file>` argument supports several formats:

| Input | Resolution |
| --- | --- |
| `resources.yaml` | Detect pack from CWD context; prompt with `huh.NewSelect` if ambiguous |
| `cap/resources.yaml` | Explicit pack + file |
| `./content/packs/cap/resources.yaml` | Direct path (contributor mode) |
| `.sap-devs/packs/mypack/resources.yaml` | Direct path (project layer) |

## Schema Engine (`internal/schema/`)

Parses JSON Schema files at runtime into a generic `FieldSpec` model. No per-type code — add a new schema and the editor works automatically.

### Core types

```go
type Schema struct {
    Type       string      // "object" or "array"
    ItemSpec   *ObjectSpec // for arrays: schema of each item
    ObjectSpec *ObjectSpec // for objects: top-level schema
}

type ObjectSpec struct {
    Fields   []FieldSpec
    Required []string
}

type FieldSpec struct {
    Key         string      // YAML key name
    Title       string      // human-readable (derived from key)
    Description string      // from schema "description"
    Type        string      // string, integer, boolean, array, object
    Required    bool

    // Value help
    Enum    []string // from "enum" — drives Select widget
    Format  string   // "uri", "date" — drives validation + placeholder
    Pattern string   // regex — drives validation
    Default any      // from "default"

    // Array items
    ItemType string   // for arrays: type of items
    ItemEnum []string // for arrays of enums: drives MultiSelect
    MinItems int
    MaxItems int

    // Nested objects
    Children []FieldSpec // for object-typed fields (e.g., tools.yaml "detect")

    // Conditional
    Condition *Condition // from "if/then" — field appears when condition met
}

type Condition struct {
    Field string
    Value any
}
```

### Schema loading

`Load(contentType string) (*Schema, error)` reads `content/schemas/<type>.schema.json` and walks the JSON Schema tree:

- Top-level `"type": "array"` → array schema, items spec becomes the form template
- Top-level `"type": "object"` → single-object schema (pack.yaml)
- `"enum"` → populates `FieldSpec.Enum`
- `"format"` → populates `FieldSpec.Format`
- `"pattern"` → populates `FieldSpec.Pattern`
- `"properties"` with nested `"type": "object"` → recurses into `Children`
- `"if/then"` conditionals → populates `FieldSpec.Condition`
- `"additionalProperties"` with string type → string-keyed map (install platform map, links map)

### Validation

`Validate(schema *Schema, data any) []ValidationError` walks data against the schema:

```go
type ValidationError struct {
    Path     string // e.g., "[2].install.windows"
    Field    string // "windows"
    Message  string // "required field missing"
    Severity string // "error" or "warning"
}
```

Used by both the editor (inline field errors) and `content validate` (batch reporting).

### Why a custom parser instead of an existing JSON Schema library

Libraries like `xeipuv/gojsonschema` validate data but don't expose schema structure (enums, formats, patterns) needed for value help. We need to both validate AND introspect, so a purpose-built parser is cleaner than wrapping a validation library and separately parsing for metadata.

## Editor TUI (`internal/editor/`)

Bubbletea `tea.Model` with two states: list view and form view. Uses `charmbracelet/huh` (new dependency) for form components.

### State 1 — List View (array-based content types)

- Scrollable table showing all items from the merged view across layers
- Each row shows key columns (ID + first 2-3 string fields from schema) and a layer badge
- Keybindings: `↑↓` navigate, `Enter` edit, `a` add new (pre-populated from schema defaults), `d` delete (only items in target layer), `/` filter, `q` save & quit, `Esc` quit without saving
- Items from inherited layers are editable — selecting one auto-creates an override copy in the target layer

### State 2 — Form View (editing a single item)

Generated dynamically from `[]FieldSpec` + current item values. Schema types map to huh components:

| Schema type | huh component |
| --- | --- |
| `string` with `enum` | `huh.NewSelect` |
| `string` with `format: uri` | `huh.NewInput` + URI validation |
| `string` with `pattern` | `huh.NewInput` + regex validation |
| `string` (plain) | `huh.NewInput` |
| `integer` | `huh.NewInput` + integer validation |
| `boolean` | `huh.NewConfirm` |
| `array` of `string` with `enum` items | `huh.NewMultiSelect` |
| `array` of `string` (free-form) | Custom tag input (type + Enter to add) |
| `object` (nested) | Grouped fields with visual indent |
| `additionalProperties` map | Key-value pair editor (see below) |

**Key-value pair editor** (for `additionalProperties` maps like `install` platforms and `links`):

The map editor presents existing key-value pairs as a mini-list within the form. Each pair is a row showing `key: value`. Keybindings:

- `Enter` on a pair → edit the value via `huh.NewInput`
- `a` → add new pair: prompts for key (with suggestions from schema context, e.g., platform names `windows`, `macos`, `linux`, `all` for install maps), then value
- `d` → delete selected pair
- `↑↓` → navigate between pairs

For maps with a known set of keys (like `install` where platforms are enumerable), the "add" action uses `huh.NewSelect` for the key instead of free-text input. The schema's `additionalProperties` type determines the value input widget.

Conditional fields (e.g., `additive_position` when `additive=true`) use huh's dynamic `OptionsFunc` — field visibility reacts to the condition trigger value.

For single-object files (pack.yaml), the editor opens directly in form view — no list step.

### Save flow

1. User presses `Ctrl+S` or `q` (save & quit)
2. `Ctrl+S` is handled as a custom Bubbletea key binding wrapping the huh form — not a native huh shortcut
3. Full schema validation runs on the current item
4. If errors: highlights first error field, blocks save
5. If clean: marshals to YAML, writes to the resolved file path
6. For override layers: only writes items belonging to that layer (not inherited items)

## Validate Command

`sap-devs content validate` scans all content files across all detected layers.

### Behavior

- Iterates all layers (official cache or CWD repo, company, user data dir, project `.sap-devs/`)
- For each pack directory, scans for all known YAML filenames
- Maps filename to schema (e.g., `resources.yaml` → `resources.schema.json`)
- Runs `schema.Validate()` on each file
- Exit code 0 if all pass, exit code 1 if any errors (suitable for CI/pre-commit)
- Warnings (severity "warning") are displayed but do not affect exit code — only errors cause exit code 1

### Flags

- `--pack <id>` — validate only a specific pack
- `--layer <name>` — validate only a specific layer
- `--json` — machine-readable output

### Validate output format

```text
$ sap-devs content validate

Validating content across all layers...

  official  content/packs/base/pack.yaml           ✓
  official  content/packs/cap/resources.yaml       ✓
  user      packs/cap/resources.yaml               ✗ 2 errors
            [0].url: not a valid URI
            [1].type: must be one of: official-docs, sample, community, tutorial, blog

Validated 14 files across 3 layers: 13 passed, 2 errors in 1 file
```

## List Command

`sap-devs content list` shows all content files across layers for discovery.

### List output format

```text
$ sap-devs content list

  PACK      FILE                    LAYER     ITEMS
  base      pack.yaml               official  —
  base      influencers.yaml        official  8
  cap       resources.yaml          official  6
  cap       resources.yaml          user      2 (overrides)
  ...
```

### List flags

- `--pack <id>` — filter to one pack
- `--layer <name>` — filter to one layer

## Dependencies

- `charmbracelet/huh` v2 — form components (Select, MultiSelect, Input, Confirm)
- `charmbracelet/bubbletea` — already present, list view model
- `charmbracelet/lipgloss` — already present (indirect), styling
- `gopkg.in/yaml.v3` — already present, YAML marshal/unmarshal

No new external dependencies beyond `huh`.

## Files to create

| File | Purpose |
| --- | --- |
| `cmd/content.go` | Parent `content` command |
| `cmd/content_edit.go` | `content edit` subcommand |
| `cmd/content_validate.go` | `content validate` subcommand |
| `cmd/content_list.go` | `content list` subcommand |
| `internal/schema/schema.go` | Schema types and `Load()` |
| `internal/schema/validate.go` | `Validate()` function |
| `internal/schema/schema_test.go` | Schema parser tests |
| `internal/schema/validate_test.go` | Validation tests |
| `internal/editor/editor.go` | Main TUI model, state machine |
| `internal/editor/list.go` | List view (Bubbletea model) |
| `internal/editor/form.go` | Form view (huh form generator) |
| `internal/editor/resolve.go` | Layer resolution and file path detection |
| `internal/editor/merge.go` | Merged view assembly with layer badges |

## Documentation

A new major section in `docs/content-authoring.md` covering:

- **Using the content editor** — `content edit`, `content validate`, `content list` usage with examples
- **Content contributor workflow** — how to edit official/company content in a git checkout, submit changes
- **Content customization workflow** — how to create user-layer and project-layer overrides
- **Schema reference** — what each schema enforces, field types, enums, format constraints
- **Extending content types** — how adding a new schema automatically enables editing for that type

This extends the existing `docs/content-authoring.md` which currently covers `<!-- sap-devs:fetch -->` markers. The editor documentation becomes a peer section.

## Out of scope for Phase 1

- Undo/redo within the editor
- Diff view showing changes before save
- Git commit/push integration from the editor
- Drag-and-drop reordering of array items
- Bulk editing across multiple files
- Content file creation wizard (creating a new pack from scratch)
