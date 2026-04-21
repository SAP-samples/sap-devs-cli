package news

import (
	"strings"
	"time"

	"github.com/SAP-samples/sap-devs-cli/internal/community"
	"github.com/SAP-samples/sap-devs-cli/internal/youtube"
)

// NewsItem pairs a YouTube episode with its matched Community blog post (if any).
type NewsItem struct {
	Episode   youtube.Episode
	Community *community.BlogPost // nil if no match within ±7 days
}

const correlationWindow = 7 * 24 * time.Hour

// Correlate pairs each episode with the closest Community post within ±7 days.
// When multiple posts are within the window, the one with the highest title
// similarity (longest common substring length) wins.
// Each post is used at most once — episodes are matched in order, and once a
// post is claimed, later episodes cannot reuse it.
// Episodes with no match have Community set to nil.
// Input episode order is preserved.
func Correlate(episodes []youtube.Episode, posts []community.BlogPost) []NewsItem {
	used := make(map[int]bool)
	items := make([]NewsItem, len(episodes))
	for i, ep := range episodes {
		items[i] = NewsItem{Episode: ep, Community: bestMatch(ep, posts, used)}
	}
	return items
}

func bestMatch(ep youtube.Episode, posts []community.BlogPost, used map[int]bool) *community.BlogPost {
	var best *community.BlogPost
	bestIdx := -1
	bestScore := -1
	for j := range posts {
		if used[j] {
			continue
		}
		diff := ep.Published.Sub(posts[j].Published)
		if diff < -correlationWindow || diff > correlationWindow {
			continue
		}
		score := lcs(strings.ToLower(ep.Title), strings.ToLower(posts[j].Title))
		if best == nil || score > bestScore {
			best = &posts[j]
			bestIdx = j
			bestScore = score
		}
	}
	if bestIdx >= 0 {
		used[bestIdx] = true
	}
	return best
}

// lcs returns the byte length of the longest common substring of a and b.
func lcs(a, b string) int {
	best := 0
	for i := range a {
		for j := range b {
			l := 0
			for i+l < len(a) && j+l < len(b) && a[i+l] == b[j+l] {
				l++
			}
			if l > best {
				best = l
			}
		}
	}
	return best
}
