package news_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/community"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/news"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/youtube"
)

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
}

func TestCorrelate_ExactDateMatch(t *testing.T) {
	episodes := []youtube.Episode{
		{ID: "ep1", Title: "News Apr 11", Published: date(2026, time.April, 11)},
	}
	posts := []community.BlogPost{
		{Title: "SAP Dev News Apr 11", URL: "https://community.sap.com/1", Published: date(2026, time.April, 11)},
	}
	items := news.Correlate(episodes, posts)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].Community)
	assert.Equal(t, "https://community.sap.com/1", items[0].Community.URL)
}

func TestCorrelate_WithinSevenDayWindow(t *testing.T) {
	episodes := []youtube.Episode{
		{ID: "ep1", Title: "News Apr 11", Published: date(2026, time.April, 11)},
	}
	posts := []community.BlogPost{
		{Title: "SAP Dev News Apr 11", URL: "https://community.sap.com/1", Published: date(2026, time.April, 14)},
	}
	items := news.Correlate(episodes, posts)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].Community)
}

func TestCorrelate_OutsideWindowIsNil(t *testing.T) {
	episodes := []youtube.Episode{
		{ID: "ep1", Title: "News Apr 11", Published: date(2026, time.April, 11)},
	}
	posts := []community.BlogPost{
		{Title: "Old post", URL: "https://community.sap.com/old", Published: date(2026, time.March, 1)},
	}
	items := news.Correlate(episodes, posts)
	require.Len(t, items, 1)
	assert.Nil(t, items[0].Community)
}

func TestCorrelate_NilPostsAllNil(t *testing.T) {
	episodes := []youtube.Episode{
		{ID: "ep1", Title: "News Apr 11", Published: date(2026, time.April, 11)},
		{ID: "ep2", Title: "News Apr 4", Published: date(2026, time.April, 4)},
	}
	items := news.Correlate(episodes, nil)
	require.Len(t, items, 2)
	assert.Nil(t, items[0].Community)
	assert.Nil(t, items[1].Community)
}

func TestCorrelate_TitleTiebreaker(t *testing.T) {
	// Two posts within ±7 days — the one with higher title similarity wins.
	episodes := []youtube.Episode{
		{ID: "ep1", Title: "SAP Developer News Apr 11", Published: date(2026, time.April, 11)},
	}
	posts := []community.BlogPost{
		{Title: "SAP Developer News Apr 11", URL: "https://community.sap.com/match", Published: date(2026, time.April, 13)},
		{Title: "Unrelated Post", URL: "https://community.sap.com/nomatch", Published: date(2026, time.April, 12)},
	}
	items := news.Correlate(episodes, posts)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].Community)
	assert.Equal(t, "https://community.sap.com/match", items[0].Community.URL)
}

func TestCorrelate_EpisodeOrderPreserved(t *testing.T) {
	episodes := []youtube.Episode{
		{ID: "ep1", Published: date(2026, time.April, 11)},
		{ID: "ep2", Published: date(2026, time.April, 4)},
	}
	items := news.Correlate(episodes, nil)
	assert.Equal(t, "ep1", items[0].Episode.ID)
	assert.Equal(t, "ep2", items[1].Episode.ID)
}
