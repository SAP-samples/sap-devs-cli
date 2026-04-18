# Friday Developer News Hook Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs news hook` — a command that prints a Friday reminder message on Fridays and exits silently on other days — and wire it into Claude Code's session-start hook.

**Architecture:** A new `newsHookCmd` cobra subcommand and a pure `fridayHookMessage(day time.Weekday) string` helper are added to `cmd/news.go`. The helper holds all logic and is tested in a new `cmd/news_test.go`. A new entry in `content/packs/base/hook.yaml` registers `sap-devs news hook` as a `sessionStart` hook for Claude Code.

**Tech Stack:** Go, cobra, `time.Weekday`

---

## File Map

| File | Change |
|---|---|
| `cmd/news.go` | Add `fridayHookMessage` helper and `newsHookCmd`; register with `newsCmd.AddCommand` in `init()` |
| `cmd/news_test.go` | New file — 2 tests for `fridayHookMessage` |
| `content/packs/base/hook.yaml` | Add `community/friday-developer-news` entry |

---

### Task 1: `fridayHookMessage` helper + tests (TDD)

**Files:**
- Create: `cmd/news_test.go`
- Modify: `cmd/news.go`

- [ ] **Step 1: Create `cmd/news_test.go` with 2 failing tests**

```go
package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFridayHookMessage_Friday(t *testing.T) {
	msg := fridayHookMessage(time.Friday)
	assert.NotEmpty(t, msg)
	assert.Contains(t, msg, "Friday")
	assert.Contains(t, msg, "sap-devs news latest")
}

func TestFridayHookMessage_NotFriday(t *testing.T) {
	nonFridays := []time.Weekday{
		time.Sunday,
		time.Monday,
		time.Tuesday,
		time.Wednesday,
		time.Thursday,
		time.Saturday,
	}
	for _, day := range nonFridays {
		assert.Empty(t, fridayHookMessage(day), "expected empty string for %s", day)
	}
}
```

- [ ] **Step 2: Verify compile fails**

```bash
cd .worktrees/feat/news && go build ./cmd/...
```

Expected: compile error — `fridayHookMessage undefined`

- [ ] **Step 3: Add `fridayHookMessage` and `newsHookCmd` to `cmd/news.go`**

Add the helper and command before the `init()` function:

```go
func fridayHookMessage(day time.Weekday) string {
	if day != time.Friday {
		return ""
	}
	return "📺 It's Friday — a new SAP Developer News episode is likely out!\n\nWould you like me to open the latest episode? Run `sap-devs news latest` or just say yes."
}

var newsHookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Print a Friday Developer News reminder (used by session-start hook)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if msg := fridayHookMessage(time.Now().Weekday()); msg != "" {
			fmt.Fprintln(cmd.OutOrStdout(), msg)
		}
		return nil
	},
}
```

Add `"time"` to the import block in `cmd/news.go`. It is not currently present.

- [ ] **Step 4: Register `newsHookCmd` in `init()`**

Find the existing line in `cmd/news.go`:

```go
newsCmd.AddCommand(newsListCmd, newsLatestCmd, newsOpenCmd, newsSearchCmd, newsReadCmd)
```

Replace with:

```go
newsCmd.AddCommand(newsListCmd, newsLatestCmd, newsOpenCmd, newsSearchCmd, newsReadCmd, newsHookCmd)
```

- [ ] **Step 5: Build and vet**

```bash
cd .worktrees/feat/news && go build ./... && go vet ./...
```

Expected: zero errors

- [ ] **Step 6: Commit**

```bash
cd .worktrees/feat/news
git add cmd/news.go cmd/news_test.go
git commit -m "feat: add news hook command with Friday reminder message"
```

---

### Task 2: Wire hook entry into `content/packs/base/hook.yaml`

**Files:**
- Modify: `content/packs/base/hook.yaml`

- [ ] **Step 1: Add hook entry to `content/packs/base/hook.yaml`**

Current file contents:

```yaml
- id: tip-on-session-start
  event: sessionStart
  command: "sap-devs tip --markdown"
  tools:
    - claude-code
```

New contents (append the second entry):

```yaml
- id: tip-on-session-start
  event: sessionStart
  command: "sap-devs tip --markdown"
  tools:
    - claude-code

- id: community/friday-developer-news
  event: sessionStart
  command: "sap-devs news hook"
  tools:
    - claude-code
```

- [ ] **Step 2: Verify the hook loads correctly**

```bash
cd .worktrees/feat/news && go run . hook list
```

Expected: table output showing both `tip-on-session-start` and `community/friday-developer-news` rows.

- [ ] **Step 3: Build and vet**

```bash
cd .worktrees/feat/news && go build ./... && go vet ./...
```

Expected: zero errors

- [ ] **Step 4: Commit**

```bash
cd .worktrees/feat/news
git add content/packs/base/hook.yaml
git commit -m "feat: register friday-developer-news session-start hook"
```

---

### Task 3: Documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/developer/developer-guide.md`

- [ ] **Step 1: Update `CLAUDE.md` CLI Commands table**

Find the `news` row:

```
| `news list/latest/open/search/read` | Browse SAP Developer News episodes fetched live from YouTube RSS and SAP Community |
```

Replace with:

```
| `news list/latest/open/search/read/hook` | Browse SAP Developer News episodes fetched live from YouTube RSS and SAP Community; `news hook` prints a Friday reminder for use as a session-start hook |
```

- [ ] **Step 2: Update `docs/developer/developer-guide.md`**

Find the line:

```
**Subcommands:** `list [-n]`, `latest`, `open <id>`, `search <query>`, `read <id> [--plain]`.
```

Replace with:

```
**Subcommands:** `list [-n]`, `latest`, `open <id>`, `search <query>`, `read <id> [--plain]`, `hook`.

**`news hook`:** Prints a Friday reminder message on Fridays, silent otherwise. Designed as a `sessionStart` hook for Claude Code — install with `sap-devs hook install community/friday-developer-news`. The pure helper `fridayHookMessage(day time.Weekday) string` holds all logic and is unit-tested in `cmd/news_test.go`. Note: this is distinct from the Friday tip override in `cmd/tip.go`, which fetches the latest episode live via YouTube RSS; `news hook` prints a static prompt and delegates fetching to the AI.
```

- [ ] **Step 3: Build and vet**

```bash
cd .worktrees/feat/news && go build ./... && go vet ./...
```

Expected: zero errors

- [ ] **Step 4: Commit**

```bash
cd .worktrees/feat/news
git add CLAUDE.md docs/developer/developer-guide.md
git commit -m "docs: document news hook subcommand and Friday reminder hook"
```
