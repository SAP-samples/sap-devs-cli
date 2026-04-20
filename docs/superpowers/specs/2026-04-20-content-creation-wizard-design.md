# Content Creation Wizard — Phase 2c Design Spec

## Goal

Add `sap-devs content create` — a guided TUI wizard that scaffolds a new content pack from scratch. The wizard collects pack metadata via huh v2 forms, creates a `context.md` template, and optionally scaffolds content YAML files with initial entries using the existing schema-driven form infrastructure.

## Audience

Both company/team content authors (creating company-layer packs for internal SAP knowledge) and individual developers (creating user-layer or project-layer packs for personal customization). The wizard assumes the user knows their SAP domain but not the YAML file structure.

## CLI Entry Point

```
sap-devs content create
```

Subcommand under the existing `content` parent, consistent with `content edit`, `content validate`, `content list`. No arguments — the wizard is fully interactive.

## Layer Resolution

The wizard auto-detects the target layer using the existing `detectLayer()` from `internal/editor/resolve.go`:

| Working directory | Detected layer | Pack directory |
|---|---|---|
| Official repo checkout | official | `content/packs/<id>/` |
| Company repo checkout | company | `content/packs/<id>/` |
| Directory with `.sap-devs/` | project | `.sap-devs/packs/<id>/` |
| Anywhere else | user | `~/.local/share/sap-devs/packs/<id>/` (Linux) / `%LOCALAPPDATA%/sap-devs/data/packs/<id>/` (Windows) |

The first wizard step shows the detected layer and lets the user override it via a huh select dropdown. Available layer options depend on context: "official" and "company" are only offered when the CWD is detected as the corresponding repo checkout (via `isOfficialRepo()` / `isCompanyRepo()`). "User" and "project" are always available. If the user selects "company" but the CWD is not a company checkout, the wizard shows an error explaining the constraint. The pack directory path is derived from the selected layer + the pack ID entered in the metadata step.

**Conflict detection:** Before writing any files, the wizard checks if a pack directory with the same ID already exists in the target layer. If it does, the wizard prints an error and aborts. The user should use `content edit` to modify existing packs.

## Wizard Flow

### Step 1: Layer confirmation

A huh select pre-populated with the auto-detected layer. The user can accept the default or change it.

### Step 2: Pack metadata form

A single huh form collecting the `pack.yaml` fields:

| Field | Input type | Required | Validation | Default |
|---|---|---|---|---|
| `id` | text input | yes | pattern `^[a-z][a-z0-9-]*$` | — |
| `name` | text input | yes | non-empty | — |
| `description` | text input | yes | non-empty | — |
| `tags` | text input | yes | comma-separated, parsed to `[]string`, at least one required | — |
| `weight` | text input | no | integer | "50" (intentionally higher than the schema default of 0, so new packs appear before base) |
| `additive` | confirm (bool) | no | — | false |
| `additive_position` | select (before/after) | conditional | only shown when `additive` is true | "after" |

**Omitted fields:**
- `base` — only the official `base` pack should use this; not exposed in the wizard
- `profiles` — informational field; can be added later via `content edit pack.yaml`
- `locales` — can be added later via `content edit pack.yaml`
- `versions` — populated by sync/staleness checks, not user-authored
- `changelog` — populated post-release, not at creation time
- `overlaps` — advanced deduplication field, not needed at creation time

The `additive` field requires a two-phase form approach. The first form is hand-built (not using `BuildForm`, since the wizard needs custom control over which pack.yaml fields appear). It collects id, name, description, tags, weight, and additive. If `additive` is true after the first form completes, a second minimal huh form with a single `huh.NewSelect[string]` collects `additive_position` (before/after, default "after").

### Step 3: Context template creation

The wizard creates `context.md` with the standard section scaffold matching the conventions enforced by `ValidateContextSections()`:

```markdown
### Overview

<!-- TODO: Describe what this pack covers -->

### Key Concepts

<!-- TODO: List the essential concepts -->

### Best Practices

<!-- TODO: Add best practices -->
```

No form needed — this is a template file written directly. Additional optional sections recognised by `ValidateContextSections()` (`Anti-patterns`, `Code Examples`) can be added by the author later.

### Step 4: Content file selection

A huh multi-select form offering optional content files to scaffold:

| File | Description |
|---|---|
| `resources.yaml` | Curated links and documentation |
| `tools.yaml` | Required/recommended developer tools |
| `mcp.yaml` | MCP server definitions |
| `samples.yaml` | Canonical code sample references |
| `known_errors.yaml` | Common error patterns with fixes |
| `tips.md` | Developer tips (H2-delimited) |
| `constraints.md` | Behavioral rules for AI agents |

**Omitted from default list:** `event-types.yaml`, `event-instances.yaml`, `influencers.yaml`, `hook.yaml`, `tutorials.yaml`, `learning.yaml`, `discovery.yaml`, `youtube.yaml`, `paths.yaml`. These are base-pack or specialized files that most custom packs won't need. They remain editable via `content edit` if needed.

If the user selects no files, the wizard skips to the summary step.

### Step 5: Initial entries

For each selected YAML file (schema-backed), the wizard opens the existing `BuildForm()` from `internal/editor/form.go` to create one initial entry. This reuses the Phase 1 schema-driven form infrastructure with no new form code.

Flow per YAML file:
1. Load the file's schema via `schema.Load()`
2. Call `BuildForm(spec, emptyMap)` to create a form for one entry
3. User fills in the entry; result stored in memory
4. User can abort (Esc) to skip this file — no entry created, file still scaffolded as empty array

For markdown files (`tips.md`, `constraints.md`), the wizard creates the file with a placeholder template:

**tips.md:**
```markdown
## Tip title here

Tip content here.
```

**constraints.md:**
```markdown
1. First constraint here.
```

### Step 6: Summary and confirmation

Before writing, the wizard prints a summary of what will be created:

```
Creating pack "my-pack" in user layer:

  ~/.local/share/sap-devs/packs/my-pack/
    pack.yaml
    context.md
    resources.yaml (1 entry)
    tools.yaml (1 entry)

Proceed? [Y/n]
```

On confirm (`Y` or Enter), all files are written. On cancel (`n`), abort without writing anything.

## Architecture

### New files

| File | Responsibility |
|---|---|
| `cmd/content_create.go` | Cobra command definition, calls `findSchemasDir(cwd)` then passes it to `editor.RunCreateWizard(cwd, schemasDir)` |
| `internal/editor/wizard.go` | Wizard orchestration: layer form, metadata form, file selection, initial entry collection, summary, batch write |

### Reused infrastructure

| Component | Used for |
|---|---|
| `internal/editor/resolve.go` | `detectLayer()` for auto-detection; layer path constants; `isOfficialRepo()` / `isCompanyRepo()` for layer availability |
| `internal/editor/merge.go` | `SaveObject()` for pack.yaml |
| `internal/editor/form.go` | `BuildForm()` for schema-driven initial entry forms |
| `internal/schema/` | `Load()` for each content file's schema |
| `internal/theme/` | `ThemeFiori` for huh form styling |
| `content/schemas/` | All existing schema files |

### Data flow

```
User input (huh forms)
  → WizardState (in-memory struct collecting all answers)
    → Batch write (all files at once after confirmation)
```

The `WizardState` struct holds:
- Selected layer + resolved pack directory path
- Pack metadata (`map[string]any` for pack.yaml)
- List of selected content files
- Initial entry data per file (`map[string]map[string]any`)
- Markdown template content for tips.md / constraints.md

### Write strategy

All-or-nothing: the wizard collects all data in memory, then writes all files in one batch after the user confirms. The write sequence:
1. `os.MkdirAll()` for the pack directory
2. `SaveObject()` for pack.yaml
3. `os.WriteFile()` for context.md
4. For each selected YAML file: `yaml.Marshal()` + `os.WriteFile()` directly with a `[]map[string]any` slice (single entry or empty). This avoids `SaveItems()` which requires `[]MergedItem` wrapping and layer filtering — unnecessary for brand-new files with no multi-layer merge concern.
5. For each selected markdown file: `os.WriteFile()` with template content

If a write fails mid-batch, already-written files remain. This is acceptable — the pack directory was just created and can be deleted manually. No rollback mechanism needed.

## Error Handling

| Scenario | Behavior |
|---|---|
| Pack ID already exists in target layer | Error message, abort before any forms |
| Invalid pack ID (fails pattern) | huh inline validation, blocks form submission |
| User aborts any form (Ctrl+C / Esc) | Clean exit via `huh.ErrUserAborted`, no files written. Esc during Step 5 (initial entry) skips that file's entry but keeps the file in the scaffold list (empty array). Esc/Ctrl+C during Steps 1-4 or 6 aborts the entire wizard. |
| Target directory not writable | Error on write attempt |
| Schema file not found for a content type | Skip that file with a warning |

## Testing

| Test area | Approach |
|---|---|
| Pack ID validation | Unit test: pattern matching for valid/invalid IDs |
| Layer path resolution | Covered by existing resolve.go tests |
| File generation | Unit test: given WizardState, verify correct YAML/markdown output |
| Conflict detection | Unit test: existing pack directory → error |
| Integration | Manual: run `sap-devs content create`, verify files on disk |

Interactive huh forms cannot be unit-tested without stdin mocking. The wizard is structured so that form collection and file writing are separate — the file-writing logic is testable independently of the interactive forms.

## Out of Scope

- Profile wiring (auto-adding the pack to a profile)
- Running `inject` after creation
- Editing existing packs (use `content edit`)
- Localization fields in the wizard (can be added via `content edit pack.yaml`)
- Back-navigation between wizard steps (user can re-run the command)
