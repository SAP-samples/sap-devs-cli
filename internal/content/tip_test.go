package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestSelectTip_ReturnsATip(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Tips: []content.Tip{
			{Title: "CAP tip 1", Content: "Use cds watch", Tags: []string{"cap"}},
			{Title: "CAP tip 2", Content: "Use CQL", Tags: []string{"cap", "nodejs"}},
		}},
	}
	tip, err := content.SelectTip(packs, []string{"cap"}, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, tip.Title)
}

func TestSelectTip_FiltersByProfileTags(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Tips: []content.Tip{
			{Title: "CAP tip", Content: "For CAP", Tags: []string{"cap"}},
		}},
		{ID: "abap", Tips: []content.Tip{
			{Title: "ABAP tip", Content: "For ABAP", Tags: []string{"abap"}},
		}},
	}
	// Only request cap-tagged tips
	for i := 0; i < 20; i++ {
		tip, err := content.SelectTip(packs, []string{"cap"}, int64(i))
		require.NoError(t, err)
		assert.Contains(t, tip.Tags, "cap")
		assert.Equal(t, "CAP tip", tip.Title)
	}
}

func TestSelectTip_EmptyPoolReturnsError(t *testing.T) {
	_, err := content.SelectTip(nil, []string{"cap"}, 0)
	assert.Error(t, err)
}

func TestSelectTip_SameSeedReturnsSameTip(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Tips: []content.Tip{
			{Title: "tip A", Tags: []string{"cap"}},
			{Title: "tip B", Tags: []string{"cap"}},
		}},
	}
	tip1, _ := content.SelectTip(packs, []string{"cap"}, 42)
	tip2, _ := content.SelectTip(packs, []string{"cap"}, 42)
	assert.Equal(t, tip1.Title, tip2.Title)
}
