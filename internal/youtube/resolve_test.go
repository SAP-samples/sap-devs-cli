package youtube_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/youtube"
)

func TestResolve_VideoType_NoNetworkCall(t *testing.T) {
	src := content.YouTubeSource{
		ID:      "cap-walkthrough",
		Type:    "video",
		Name:    "CAP Full Walkthrough",
		VideoID: "dQw4w9WgXcQ",
		Tags:    []string{"tutorial"},
	}
	episodes, err := youtube.Resolve(src, "")
	require.NoError(t, err)
	require.Len(t, episodes, 1)
	assert.Equal(t, "dQw4w9WgXcQ", episodes[0].ID)
	assert.Equal(t, "CAP Full Walkthrough", episodes[0].Title)
	assert.Equal(t, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", episodes[0].URL)
}
