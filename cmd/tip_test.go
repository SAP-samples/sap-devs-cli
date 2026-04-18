package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
