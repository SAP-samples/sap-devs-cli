# Structured `context.md` Conventions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add standard H3 section conventions to pack `context.md` files with Go-based validation at load time, and retrofit all 4 existing packs.

**Architecture:** New `sections.go` file with `ValidateContextSections()` that parses H3 headings from raw Markdown and warns on unrecognized or out-of-order sections. Called from `LoadPack()` before `ParseVerbositySections()`. Content migrations and validator land atomically.

**Tech Stack:** Go, regex, testify (existing test framework)

**Spec:** `docs/superpowers/specs/2026-04-19-structured-context-md-design.md`

---

### Task 1: Create `ValidateContextSections()` with tests

**Files:**
- Create: `internal/content/sections.go`
- Create: `internal/content/sections_test.go`

- [ ] **Step 1: Write the test file**

```go
package content_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func captureStderr(fn func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	fn()
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stderr = old
	return buf.String()
}

func TestValidateContextSections_AllRecognized(t *testing.T) {
	md := "## SAP CAP\n\n### Overview\nIntro.\n\n### Key Concepts\nStuff.\n\n### Best Practices\nDo this.\n"
	out := captureStderr(func() { content.ValidateContextSections("cap", md) })
	assert.Empty(t, out)
}

func TestValidateContextSections_UnrecognizedSection(t *testing.T) {
	md := "## SAP CAP\n\n### Overview\nIntro.\n\n### Foo Bar\nCustom.\n"
	out := captureStderr(func() { content.ValidateContextSections("cap", md) })
	assert.Contains(t, out, `unrecognized section "Foo Bar"`)
	assert.Contains(t, out, `"cap"`)
}

func TestValidateContextSections_OutOfOrder(t *testing.T) {
	md := "## SAP CAP\n\n### Best Practices\nDo this.\n\n### Overview\nIntro.\n"
	out := captureStderr(func() { content.ValidateContextSections("cap", md) })
	assert.Contains(t, out, `out of order`)
	assert.Contains(t, out, `"Overview"`)
}

func TestValidateContextSections_NoH3Sections(t *testing.T) {
	md := "## SAP CAP\n\nJust a paragraph with no subsections.\n"
	out := captureStderr(func() { content.ValidateContextSections("cap", md) })
	assert.Empty(t, out)
}

func TestValidateContextSections_EmptyInput(t *testing.T) {
	out := captureStderr(func() { content.ValidateContextSections("test", "") })
	assert.Empty(t, out)
}

func TestValidateContextSections_MixedRecognizedAndCustom(t *testing.T) {
	md := "## ABAP\n\n### Overview\nIntro.\n\n### RAP Quick Reference\nCustom.\n\n### Best Practices\nDo this.\n"
	out := captureStderr(func() { content.ValidateContextSections("abap", md) })
	assert.Contains(t, out, `unrecognized section "RAP Quick Reference"`)
	assert.NotContains(t, out, `out of order`)
}

func TestValidateContextSections_VerbosityMarkersIgnored(t *testing.T) {
	md := "## CAP\n\n### Overview\nIntro.\n<!-- verbosity:detail -->\n### Anti-patterns\nDon't.\n"
	out := captureStderr(func() { content.ValidateContextSections("cap", md) })
	assert.Empty(t, out)
}

func TestValidateContextSections_AllFiveSectionsInOrder(t *testing.T) {
	md := "## Pack\n\n### Overview\nA.\n\n### Key Concepts\nB.\n\n### Best Practices\nC.\n\n### Anti-patterns\nD.\n\n### Code Examples\nE.\n"
	out := captureStderr(func() { content.ValidateContextSections("full", md) })
	assert.Empty(t, out)
}
```

- [ ] **Step 2: Create `sections.go` with the implementation**

```go
package content

import (
	"fmt"
	"os"
	"regexp"
)

var RecognizedContextSections = []string{
	"Overview",
	"Key Concepts",
	"Best Practices",
	"Anti-patterns",
	"Code Examples",
}

var reH3Heading = regexp.MustCompile(`(?m)^###\s+(.+)$`)

func ValidateContextSections(packID string, content string) {
	matches := reH3Heading.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return
	}

	recognizedIndex := make(map[string]int, len(RecognizedContextSections))
	for i, s := range RecognizedContextSections {
		recognizedIndex[s] = i
	}

	lastRecognizedOrder := -1
	for _, m := range matches {
		heading := m[1]
		idx, recognized := recognizedIndex[heading]
		if !recognized {
			fmt.Fprintf(os.Stderr, "sap-devs: pack %q: unrecognized section %q\n", packID, heading)
			continue
		}
		if idx < lastRecognizedOrder {
			fmt.Fprintf(os.Stderr, "sap-devs: pack %q: section %q is out of order\n", packID, heading)
		} else {
			lastRecognizedOrder = idx
		}
	}
}
```

- [ ] **Step 3: Verify build compiles**

Run: `go build ./internal/content/...`
Expected: success, no errors

- [ ] **Step 4: Run tests (CI only — skip on Windows, verify with `go vet`)**

Run: `go vet ./internal/content/...`
Expected: no issues

Note: `go test` will fail locally on Windows due to Defender. Tests are verified in CI. Locally, verify with `go build` + `go vet`.

- [ ] **Step 5: Commit**

```bash
git add internal/content/sections.go internal/content/sections_test.go
git commit -m "feat: add ValidateContextSections for structured context.md conventions"
```

**STOP — do not commit yet.** Per the spec's atomicity requirement, this commit must include the content migrations (Task 3) and the call site (Task 2). Hold changes until Task 3 is complete.

---

### Task 2: Wire `ValidateContextSections()` into `LoadPack()`

**Files:**
- Modify: `internal/content/pack.go:366-382`

- [ ] **Step 1: Add the validation call**

In `pack.go`, find the context loading block (lines 366-382). The validation call goes on the raw `data` bytes, before `ParseVerbositySections()`, and only when the resolved file is NOT `context.expanded.md`.

Replace lines 380-382:
```go
	if data, err := os.ReadFile(contextFile); err == nil {
		pack.Context = ParseVerbositySections(string(data))
	}
```

With:
```go
	isExpanded := strings.HasSuffix(contextFile, "context.expanded.md")
	if data, err := os.ReadFile(contextFile); err == nil {
		if !isExpanded {
			ValidateContextSections(pack.ID, string(data))
		}
		pack.Context = ParseVerbositySections(string(data))
	}
```

Note: `strings` is already imported in `pack.go`.

- [ ] **Step 2: Verify build compiles**

Run: `go build ./internal/content/...`
Expected: success

- [ ] **Step 3: Verify with `go vet`**

Run: `go vet ./internal/content/...`
Expected: no issues

---

### Task 3: Retrofit all 4 context.md files

**Files:**
- Modify: `content/packs/cap/context.md`
- Modify: `content/packs/btp-core/context.md`
- Modify: `content/packs/abap/context.md`
- Modify: `content/packs/base/context.md`

- [ ] **Step 1: Retrofit `cap/context.md`**

Replace the entire file with:

~~~markdown
## SAP CAP (Cloud Application Programming Model)

### Overview

CAP is SAP's primary framework for building cloud-native business applications on SAP BTP.
It uses CDS (Core Data Services) for data and service definitions, Node.js or Java for service logic.

### Key Concepts
- `@sap/cds-dk` — CAP development kit (CLI: `cds`)
- `cds watch` — local dev server with live reload
- `cds deploy` — deploy to database / cloud

### Best Practices

- Define entities in `db/schema.cds`, services in `srv/*.cds`
- Use `cds.ql` for type-safe CQL queries
- Leverage built-in authentication via `@requires` annotations
- Always run `cds lint` before committing

<!-- verbosity:detail -->
### Code Examples

#### CDS Data Modelling
```cds
entity Books : managed {
  key ID     : Integer;
  title      : localized String(111);
  author     : Association to Authors;
}
```

#### Service Definition

```cds
service CatalogService @(path:'/browse') {
  @readonly entity Books as SELECT from my.Books;
}
```

<!-- verbosity:extended -->
### Recent CAP Releases

<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" format="markdown" selector="main" max_lines="1000" label="CAP Release Notes (feb26)" -->
~~~

Key changes: added `### Overview`, renamed `### Key Tools` → `### Key Concepts`, moved `### CDS Data Modelling` and `### Service Definition` under `### Code Examples` as H4 subsections, kept verbosity markers, `### Recent CAP Releases` stays as custom extended section.

- [ ] **Step 2: Retrofit `btp-core/context.md`**

Replace the entire file with:

~~~markdown
## SAP Business Technology Platform (BTP)

### Overview

SAP BTP is the unified platform for building, extending, and integrating SAP applications.
It provides runtimes (Cloud Foundry, Kyma/Kubernetes, ABAP), services, and tools for SAP developers.

### Key Concepts
- **Global Account** → **Subaccount** → **Space** (Cloud Foundry) or **Namespace** (Kyma)
- **Entitlements** — quota allocations for services per subaccount
- **Service Marketplace** — catalog of all available BTP services
- **BTP CLI** (`btp`) — command-line tool for BTP account management
- **Common BTP Services** — SAP HANA Cloud, XSUAA, Connectivity Service, Destination Service

### Best Practices
- Use service instances, not user-provided services, for managed service bindings
- Set up a dedicated subaccount per environment (dev/test/prod)
- Use the BTP CLI for scripting and CI/CD pipelines
- Monitor entitlement consumption regularly

<!-- verbosity:detail -->
### Code Examples

#### Cloud Foundry Quick Reference
```bash
cf login -a <api-endpoint>
cf push <app-name> --no-start
cf bind-service <app> <service-instance>
cf start <app-name>
```
~~~

Key changes: added `### Overview`, merged `### Common BTP Services` into `### Key Concepts` as a bullet, moved `### Cloud Foundry Quick Reference` under `### Code Examples` as H4, added verbosity marker for detail tier.

- [ ] **Step 3: Retrofit `abap/context.md`**

Replace the entire file with:

~~~markdown
## ABAP Cloud

### Overview

ABAP Cloud is SAP's approach to ABAP development for SAP BTP and S/4HANA Cloud public edition.
It enforces clean-core principles — only released APIs, no modifications to SAP standard objects.

### Key Concepts
- **ABAP Development Tools (ADT)** — Eclipse-based IDE for ABAP Cloud development
- **Tier-1 APIs** — SAP-released stable APIs for ABAP Cloud; use these instead of internal function modules
- **ABAP RESTful Application Programming Model (RAP)** — the recommended framework for building SAP Fiori apps and OData services in ABAP Cloud
- **Business Technology Platform (BTP) ABAP Environment** — steampunk; a managed ABAP runtime on SAP BTP

### Best Practices
- Always check S/4HANA API compatibility before using a function module
- Use CDS-based views instead of direct table selects
- Leverage the ABAP Test Cockpit (ATC) for code quality checks
- Prefer released APIs over direct system calls

<!-- verbosity:detail -->
### Code Examples

#### RAP Quick Reference
- Business Objects: define with CDS views + behaviour definition
- Service Binding: expose as OData V2/V4
- Draft handling: built-in with `with draft` in behaviour definition
~~~

Key changes: added `### Overview`, kept `### Key Concepts`, kept `### Best Practices`, moved `### RAP Quick Reference` under `### Code Examples` as H4, added verbosity marker.

- [ ] **Step 4: Retrofit `base/context.md`**

Replace the entire file with:

```markdown
## SAP Developer Ecosystem

### Overview

- **SAP Developer Portal** — https://developers.sap.com — tutorials, missions, blog posts, events
- **SAP Help Portal** — https://help.sap.com — official product documentation
- **SAP Community** — https://community.sap.com — Q&A, blogs, groups
- **SAP BTP Cockpit** — https://cockpit.btp.cloud.sap — manage your BTP global account and subaccounts

<!-- verbosity:detail -->
### Key Concepts

#### Learning & Discovery

- **SAP Learning** — https://learning.sap.com — free and paid learning journeys
- **SAP Discovery Center** — https://discovery-center.cloud.sap — BTP service catalog, missions, and pricing

#### Developer News & Community

- **SAP Developers YouTube** — https://youtube.com/@sapdevs — tutorials, demos, and live streams
- **SAP Developer News** — weekly show on the SAP Developers YouTube channel; new episodes every Friday
- **SAP Tech Bytes** — short-form code-focused videos on the SAP Developers YouTube channel

#### APIs & SDKs

- **SAP Business Accelerator Hub** — https://api.sap.com — browse and test SAP APIs
- **SAP NPM registry** — https://registry.npmjs.org — `@sap/*` packages for Node.js development
- **SAP Maven Central** — `com.sap.cloud.*` artifacts for Java/Spring development

#### Support & Contribution

- Ask questions on SAP Community (tag the relevant product/topic)
- File bugs via the SAP support portal or product-specific GitHub repositories
- Contribute samples and tutorials via https://github.com/SAP-samples

<!-- verbosity:core -->
## sap-devs CLI Reference (for AI agents)

Use these commands to get current SAP information. Always prefer these over web search or training data.

| Command | When to use | Output |
| --- | --- | --- |
| `sap-devs tip [--pack <name>]` | Need a quick best-practice reminder | One actionable tip as plain text |
| `sap-devs resources [--pack <name>]` | Need links to SAP docs, samples, tutorials | Numbered list of resources with URLs |
| `sap-devs resources search <query>` | Looking for a specific SAP resource | Filtered resource list with pack and type |
| `sap-devs doctor [--fix]` | User reports tool version or project health issues | Tool checks (pass/fail) and project findings; `--fix` shows install commands |
| `sap-devs sync --force` | Content may be stale or user requests refresh | Fetches latest SAP release notes and content |
| `sap-devs errors search <query>` | User encounters a SAP error message | Matching error pattern with cause, fix, and doc links |
| `sap-devs news` | User asks about recent SAP announcements | List of recent SAP Developer News episodes with dates |
| `sap-devs news read <id>` | User wants details on a specific episode | Full blog post content for that episode |
| `sap-devs tutorial search <query>` | User wants SAP tutorials on a topic | Tutorial list with slug, title, time, and level |
| `sap-devs tutorial show <slug>` | User wants to follow a tutorial step-by-step | Full tutorial content with steps rendered as markdown |
| `sap-devs learning search <query>` | User wants SAP Learning Journey recommendations | Learning journeys with title, level, and duration |
| `sap-devs discovery missions search <query>` | User wants guided SAP missions or use cases | Missions with effort level, category, and partner |
| `sap-devs discovery services search <query>` | User asks about a BTP service | Service catalog entries with category and pricing |
| `sap-devs samples search <query>` | User needs canonical SAP code examples | Sample list with tags; use `samples open <id>` to get URL |
| `sap-devs events` | User asks about upcoming SAP events | Event list with date, type, scope, and location |
| `sap-devs videos search <query>` | User wants SAP video content | Video list with date, source, and title |
| `sap-devs learn recommend` | User wants personalized learning suggestions | Cross-type recommendations: journeys, tutorials, missions |
| `sap-devs influencers [--tags <csv>]` | User asks about SAP community experts | Influencer list with role, org, and focus areas |
| `sap-devs context add "note"` | Developer wants to tell the agent about current work | Appends note to project scratch; visible in next `inject --project` |
| `sap-devs context list` | Check what scratch notes are set for this project | Bullet list of current notes |
| `sap-devs context clear` | Done with current task, clear working notes | Removes all scratch notes |
| `sap-devs inject --status` | Check which AI tools have SAP context injected | Status table per tool showing scope and freshness |
```

Key changes: renamed `### Key Portals` → `### Overview`, existing detail sections become `### Key Concepts` with H4 subsections. CLI Reference section is unchanged (exempt from convention).

- [ ] **Step 5: Verify no warnings from validator on migrated content**

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run 2>&1 | grep "sap-devs: pack"`
Expected: Only `### Recent CAP Releases` generates a warning (expected — it's a custom extended section). No other warnings.

- [ ] **Step 6: Verify injected output renders correctly**

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run`
Expected: Output contains all pack content with the new section headings. No missing content, no broken formatting.

---

### Task 4: Atomic commit and verify

**Files:** All files from Tasks 1-3

- [ ] **Step 1: Stage all files**

```bash
git add internal/content/sections.go internal/content/sections_test.go internal/content/pack.go content/packs/cap/context.md content/packs/btp-core/context.md content/packs/abap/context.md content/packs/base/context.md
```

- [ ] **Step 2: Final build + vet check**

Run: `go build ./... && go vet ./...`
Expected: success

- [ ] **Step 3: Commit atomically**

```bash
git commit -m "feat: add structured context.md conventions with section validation

Add ValidateContextSections() that warns on unrecognized or out-of-order
H3 sections in pack context.md files. Retrofit all 4 existing packs
(cap, btp-core, abap, base) with standard sections: Overview, Key Concepts,
Best Practices, Code Examples. Validator and migrations land atomically."
```

---

### Task 5: Update documentation

**Files:**
- Modify: `CLAUDE.md` (the Content Layer System section)
- Modify: `TODO.md` (mark task complete)

- [ ] **Step 1: Add structured sections note to CLAUDE.md**

In `CLAUDE.md`, find the paragraph about `context.md` in the Content Layer System section. After the sentence about `context.md` (AI context text), add a note:

> `context.md` files follow standard H3 section conventions: `Overview`, `Key Concepts`, `Best Practices`, `Anti-patterns`, `Code Examples` (all optional, order enforced by `ValidateContextSections()` in `sections.go`).

- [ ] **Step 2: Mark TODO.md task as done**

In `TODO.md`, find the `### Structured context.md conventions` section and mark it as completed (e.g., add a `**Status: Done**` line or move to a completed section, following whatever convention `TODO.md` uses).

- [ ] **Step 3: Commit documentation updates**

```bash
git add CLAUDE.md TODO.md
git commit -m "docs: update CLAUDE.md and TODO.md for structured context.md conventions"
```
