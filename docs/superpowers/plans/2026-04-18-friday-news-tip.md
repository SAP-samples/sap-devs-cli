# Friday News Tip Override Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On Fridays, `sap-devs tip` overrides the normal daily tip with the latest SAP Developer News episode fetched live from the YouTube RSS feed, with a static fallback when the fetch fails.

**Architecture:** All logic lives in `cmd/tip.go` on the `feat/news` branch (which already has `internal/youtube`). Two pure helper functions (`formatFridayTip`, `staticFridayTip`) are added first so they can be unit-tested, then `fridayNewsOverride` orchestrates them, and finally `tipCmd.RunE` is wired to call the override before the existing `SelectTip` path.

**Tech Stack:** Go, `internal/youtube` (already in feat/news branch), `content.Tip` struct, `time.Weekday`

---

## File Map

| File | Change |
|---|---|
| `cmd/news.go` | Add `newsPlaylistURL` constant |
| `cmd/tip.go` | Add `formatFridayTip`, `staticFridayTip`, `fridayNewsOverride`; wire override into `tipCmd.RunE` |
| `cmd/tip_test.go` | Add 3 tests for `formatFridayTip` |

> All files are in the `.worktrees/feat/news/` worktree on branch `feat/news`.

---

### Task 1: `formatFridayTip` helper + 3 unit tests (TDD)

**Files:**
- Modify: `cmd/tip_test.go`
- Modify: `cmd/tip.go`

The `formatFridayTip` function is pure (no I/O), so we write the tests first.

- [ ] **Step 1: Write the 3 failing tests in `cmd/tip_test.go`**

Add these tests after the existing `TestTipSeed_*` tests:

```go
func TestFormatFridayTip_ShortDescription(t *testing.T) {
	ep := youtube.Episode{
		Title:       "Episode 42",
		URL:         "https://youtu.be/abc123",
		Description: "Short desc",
	}
	tip := formatFridayTip(ep)
	assert.Equal(t, "SAP Developer News — Episode 42", tip.Title)
	assert.Equal(t, "https://youtu.be/abc123\n\nShort desc", tip.Content)
}

func TestFormatFridayTip_LongDescriptionTrimmed(t *testing.T) {
	// Build a description longer than 280 runes using a multi-byte character
	// to confirm rune-count (not byte-count) truncation.
	long := strings.Repeat("é", 300) // each 'é' is 2 bytes but 1 rune
	ep := youtube.Episode{
		Title:       "Ep",
		URL:         "https://youtu.be/x",
		Description: long,
	}
	tip := formatFridayTip(ep)
	runes := []rune(tip.Content)
	// Content = URL + "\n\n" + trimmed description
	// The description part must be ≤280 runes and end with "…"
	parts := strings.SplitN(tip.Content, "\n\n", 2)
	assert.Equal(t, "https://youtu.be/x", parts[0])
	desc := parts[1]
	assert.LessOrEqual(t, len([]rune(desc)), 281) // 280 chars + "…" = 281 runes max
	assert.True(t, strings.HasSuffix(desc, "…"))
	_ = runes
}

func TestFormatFridayTip_EmptyDescription(t *testing.T) {
	ep := youtube.Episode{
		Title:       "Ep",
		URL:         "https://youtu.be/x",
		Description: "",
	}
	tip := formatFridayTip(ep)
	assert.Equal(t, "https://youtu.be/x", tip.Content)
}
```

Add the `"strings"` import if not already present (it already is in `cmd/tip.go` but `tip_test.go` needs it). Also add the `youtube` import to `tip_test.go`:

```go
import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/youtube"
)
```

- [ ] **Step 2: Run tests to verify they fail (compile error expected)**

```bash
cd .worktrees/feat/news && go build ./cmd/...
```

Expected: compile error — `formatFridayTip undefined`

- [ ] **Step 3: Implement `formatFridayTip` and `staticFridayTip` in `cmd/tip.go`**

Add the `youtube` import to `cmd/tip.go`:

```go
import (
	// existing imports...
	"github.tools.sap/developer-relations/sap-devs-cli/internal/youtube"
)
```

Add these two functions before the `tipCmd` var:

```go
func formatFridayTip(ep youtube.Episode) *content.Tip {
	desc := ep.Description
	if desc != "" {
		runes := []rune(desc)
		if len(runes) > 280 {
			desc = string(runes[:280]) + "…"
		}
	}
	var c string
	if desc == "" {
		c = ep.URL
	} else {
		c = ep.URL + "\n\n" + desc
	}
	return &content.Tip{
		Title:   "SAP Developer News — " + ep.Title,
		Content: c,
	}
}

func staticFridayTip() *content.Tip {
	return &content.Tip{
		Title:   "It's Friday — SAP Developer News is out!",
		Content: "Watch the latest episode:\n" + newsPlaylistURL,
	}
}
```

> Note: `staticFridayTip` references `newsPlaylistURL` which is added in Task 2. For now this will cause a compile error — that's fine; we'll fix it in Task 2.

- [ ] **Step 4: Build to verify `formatFridayTip` and `staticFridayTip` compile (ignoring missing constant)**

```bash
cd .worktrees/feat/news && go build ./cmd/...
```

Expected: compile error only for `newsPlaylistURL undefined` (Task 2 will fix this)

- [ ] **Step 5: Run tests for `formatFridayTip`**

Since `go test` is blocked by Windows Defender locally, verify with build + vet only:

```bash
cd .worktrees/feat/news && go build ./... && go vet ./...
```

Expected: compile error for `newsPlaylistURL` — that's expected; the constant comes in Task 2.

> Note: Tests will be verified green in CI after Task 2 completes.

- [ ] **Step 6: Commit the test + helper functions**

```bash
cd .worktrees/feat/news
git add cmd/tip.go cmd/tip_test.go
git commit -m "feat: add formatFridayTip and staticFridayTip helpers with tests"
```

---

### Task 2: Add `newsPlaylistURL`, `fridayNewsOverride`, and wire into `tipCmd.RunE`

**Files:**
- Modify: `cmd/news.go` — add `newsPlaylistURL` constant
- Modify: `cmd/tip.go` — add `fridayNewsOverride`; wire into `tipCmd.RunE`

- [ ] **Step 1: Add `newsPlaylistURL` constant to `cmd/news.go`**

In the `const` block alongside `newsPlaylistRSS`:

```go
const (
	newsPlaylistRSS  = "https://www.youtube.com/feeds/videos.xml?playlist_id=PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg"
	newsPlaylistURL  = "https://www.youtube.com/playlist?list=PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg"
	newsCommunityRSS = "https://community.sap.com/t5/developer-news/bg-p/developer-news/rss"
	newsLinkedIn     = "https://www.linkedin.com/newsletters/sap-developer-news-7155319074263044096/"
	newsYTMusic      = "" // footer line is suppressed when empty
)
```

- [ ] **Step 2: Add `fridayNewsOverride` to `cmd/tip.go`**

Add this function after `staticFridayTip`:

```go
func fridayNewsOverride() *content.Tip {
	if time.Now().Weekday() != time.Friday {
		return nil
	}
	episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
	if err != nil || len(episodes) == 0 {
		return staticFridayTip()
	}
	return formatFridayTip(episodes[0])
}
```

- [ ] **Step 3: Wire override into `tipCmd.RunE`**

In `tipCmd.RunE`, replace the existing `SelectTip` call site. The current code is:

```go
useRandom := tipNew || os.Getenv("SAP_DEVS_DEV") == "1"
seed := tipSeed(rotation, useRandom)

tip, err := content.SelectTip(packs, tipTags, seed)
if err != nil {
    // No tips available — not an error worth surfacing as exit code 1
    fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "tip.no_tips"))
    return nil
}
```

Replace with:

```go
useRandom := tipNew || os.Getenv("SAP_DEVS_DEV") == "1"
seed := tipSeed(rotation, useRandom)

var selectedTip *content.Tip
if !useRandom {
    selectedTip = fridayNewsOverride()
}
if selectedTip == nil {
    selectedTip, err = content.SelectTip(packs, tipTags, seed)
    if err != nil {
        // No tips available — not an error worth surfacing as exit code 1
        fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "tip.no_tips"))
        return nil
    }
}
tip := selectedTip
```

Then update the rendering block below to use `tip` (already a `*content.Tip`):

```go
if tipMarkdown || tipPlain {
    fmt.Fprint(cmd.OutOrStdout(), FormatTip(*tip, tipMarkdown, tipPlain))
    return nil
}
md := fmt.Sprintf("## 💡 %s\n\n%s\n", tip.Title, tip.Content)
```

This is unchanged — the `*tip` dereference already works since `tip` is `*content.Tip`.

- [ ] **Step 4: Build and vet to confirm everything compiles**

```bash
cd .worktrees/feat/news && go build ./... && go vet ./...
```

Expected: zero errors

- [ ] **Step 5: Smoke test manually on a Friday (or trust CI)**

On a Friday: `go run . tip` should show a SAP Developer News episode title and URL.
On any other day: `go run . tip` should show a normal tip.
With `--new` on a Friday: `go run . tip --new` should show a normal tip (override bypassed).

If it's not Friday, verify the bypass path with:

```bash
cd .worktrees/feat/news && go run . tip --new
```

Expected: normal tip (not the Friday override)

- [ ] **Step 6: Commit**

```bash
cd .worktrees/feat/news
git add cmd/news.go cmd/tip.go
git commit -m "feat: override tip with SAP Developer News episode on Fridays"
```
