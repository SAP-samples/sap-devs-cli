package youtube

import (
	"fmt"
	"os"
	"time"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// Resolve fetches videos for a YouTubeSource.
// For type:"video", returns a synthetic Episode without a network call.
// For type:"playlist" with apiKey, tries API v3 first, falls back to RSS.
// For type:"playlist" without apiKey, uses RSS directly.
func Resolve(src content.YouTubeSource, apiKey string) ([]Episode, error) {
	switch src.Type {
	case "video":
		return []Episode{{
			ID:        src.VideoID,
			Title:     src.Name,
			URL:       "https://www.youtube.com/watch?v=" + src.VideoID,
			Published: time.Now(),
		}}, nil

	case "playlist":
		rssURL := fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?playlist_id=%s", src.PlaylistID)
		if apiKey != "" {
			eps, err := FetchPlaylistAPI(src.PlaylistID, apiKey)
			if err == nil {
				return eps, nil
			}
			fmt.Fprintf(os.Stderr, "sap-devs: YouTube API error for %s, falling back to RSS: %v\n", src.ID, err)
		}
		return FetchPlaylist(rssURL)

	default:
		return nil, fmt.Errorf("youtube: unknown source type %q", src.Type)
	}
}
