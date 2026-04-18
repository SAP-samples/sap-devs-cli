package videos

import (
	"strings"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// ResolveAll loads all cached videos for the given sources from cacheDir.
// Sources that have no cache file are silently skipped.
func ResolveAll(sources []content.YouTubeSource, cacheDir string) ([]content.Video, error) {
	var out []content.Video
	for _, src := range sources {
		vids, err := LoadCache(cacheDir, src.PackID, src.ID)
		if err != nil {
			continue
		}
		out = append(out, vids...)
	}
	return out, nil
}

// FilterVideos returns all videos whose title, description, or tags contain query (case-insensitive).
func FilterVideos(vids []content.Video, query string) []content.Video {
	q := strings.ToLower(query)
	var out []content.Video
	for _, v := range vids {
		if strings.Contains(strings.ToLower(v.Title), q) ||
			strings.Contains(strings.ToLower(v.Description), q) {
			out = append(out, v)
			continue
		}
		for _, tag := range v.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				out = append(out, v)
				break
			}
		}
	}
	return out
}

// FindVideo returns a pointer to the first video with the given composite ID, or nil if not found.
func FindVideo(vids []content.Video, id string) *content.Video {
	for i := range vids {
		if vids[i].ID == id {
			return &vids[i]
		}
	}
	return nil
}
