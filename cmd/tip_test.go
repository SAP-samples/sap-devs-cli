package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/youtube"
)

func TestTipSeed_DailyConsistency(t *testing.T) {
	// Two calls with the same rotation and useRandom=false must return the same value
	s1 := tipSeed("daily", false)
	s2 := tipSeed("daily", false)
	assert.Equal(t, s1, s2)
}

func TestTipSeed_SessionSameAsHourly(t *testing.T) {
	session := tipSeed("session", false)
	hourly := tipSeed("hourly", false)
	assert.Equal(t, session, hourly)
}

func TestTipSeed_EmptyStringIsDailyBehavior(t *testing.T) {
	// "" falls through to the default case — same formula as "daily"
	empty := tipSeed("", false)
	daily := tipSeed("daily", false)
	assert.Equal(t, empty, daily)
}

func TestTipSeed_RandomProducesDistinctValues(t *testing.T) {
	// useRandom=true must eventually produce a different seed.
	// Two back-to-back calls can collide on fast hardware, so we retry.
	s1 := tipSeed("daily", true)
	for i := 0; i < 1000; i++ {
		if tipSeed("daily", true) != s1 {
			return
		}
	}
	t.Fatal("tipSeed with useRandom=true returned the same value 1001 times in a row")
}

func TestTipSeed_HourlyAndDailyArePositive(t *testing.T) {
	daily := tipSeed("daily", false)
	hourly := tipSeed("hourly", false)
	assert.Greater(t, daily, int64(0))
	assert.Greater(t, hourly, int64(0))
}

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
	long := strings.Repeat("é", 300)
	ep := youtube.Episode{
		Title:       "Ep",
		URL:         "https://youtu.be/x",
		Description: long,
	}
	tip := formatFridayTip(ep)
	parts := strings.SplitN(tip.Content, "\n\n", 2)
	assert.Equal(t, "https://youtu.be/x", parts[0])
	desc := parts[1]
	assert.Equal(t, 281, len([]rune(desc)))
	assert.True(t, strings.HasSuffix(desc, "…"))
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
