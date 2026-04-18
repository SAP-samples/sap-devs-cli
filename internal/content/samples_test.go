package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func fixtureSamplePacks() []*content.Pack {
	return []*content.Pack{
		{
			ID: "cap",
			Samples: []content.Sample{
				{ID: "cap/handler", Label: "CAP service handler", URL: "https://github.com/SAP-samples/cloud-cap-samples/blob/main/bookshop/srv/cat-service.js", Description: "Canonical handler pattern", Tags: []string{"cap", "node", "handler"}, Inject: true, PackID: "cap"},
				{ID: "cap/schema", Label: "CDS data model", URL: "https://github.com/SAP-samples/cloud-cap-samples/blob/main/bookshop/db/schema.cds", Description: "Entity definitions", Tags: []string{"cap", "cds"}, Inject: false, PackID: "cap"},
			},
		},
		{
			ID: "abap",
			Samples: []content.Sample{
				{ID: "abap/rap-bo", Label: "RAP Business Object", URL: "https://github.com/SAP-samples/abap-platform-rap/blob/main/src/zbp_travel.clas.abap", Description: "RAP BO implementation", Tags: []string{"abap", "rap"}, Inject: true, PackID: "abap"},
			},
		},
	}
}

func TestFlattenSamples(t *testing.T) {
	got := content.FlattenSamples(fixtureSamplePacks())
	require.Len(t, got, 3)
	assert.Equal(t, "cap/handler", got[0].ID)
	assert.Equal(t, "cap", got[0].PackID)
	assert.Equal(t, "cap/schema", got[1].ID)
	assert.Equal(t, "abap/rap-bo", got[2].ID)
}

func TestFlattenSamples_NilInput(t *testing.T) {
	got := content.FlattenSamples(nil)
	assert.Empty(t, got)
}

func TestFilterSamplesByTags_ORMatch(t *testing.T) {
	samples := content.FlattenSamples(fixtureSamplePacks())
	got := content.FilterSamplesByTags(samples, []string{"rap"})
	require.Len(t, got, 1)
	assert.Equal(t, "abap/rap-bo", got[0].ID)
}

func TestFilterSamplesByTags_MultipleTagsOR(t *testing.T) {
	samples := content.FlattenSamples(fixtureSamplePacks())
	got := content.FilterSamplesByTags(samples, []string{"cds", "rap"})
	require.Len(t, got, 2)
}

func TestFilterSamplesByTags_CaseInsensitive(t *testing.T) {
	samples := content.FlattenSamples(fixtureSamplePacks())
	got := content.FilterSamplesByTags(samples, []string{"CAP"})
	assert.Len(t, got, 2)
}

func TestFilterSamplesByTags_NoMatch(t *testing.T) {
	samples := content.FlattenSamples(fixtureSamplePacks())
	got := content.FilterSamplesByTags(samples, []string{"nonexistent"})
	assert.Empty(t, got)
}

func TestFilterSamplesByPack(t *testing.T) {
	got := content.FilterSamplesByPack(fixtureSamplePacks(), "cap")
	require.Len(t, got, 2)
	assert.Equal(t, "cap/handler", got[0].ID)
}

func TestFilterSamplesByPack_NotFound(t *testing.T) {
	got := content.FilterSamplesByPack(fixtureSamplePacks(), "nonexistent")
	assert.Nil(t, got)
}

func TestFilterSamples_DescriptionMatch(t *testing.T) {
	samples := content.FlattenSamples(fixtureSamplePacks())
	got := content.FilterSamples(samples, "canonical")
	require.Len(t, got, 1)
	assert.Equal(t, "cap/handler", got[0].ID)
}

func TestFilterSamples_TagMatch(t *testing.T) {
	samples := content.FlattenSamples(fixtureSamplePacks())
	got := content.FilterSamples(samples, "rap")
	require.Len(t, got, 1)
	assert.Equal(t, "abap/rap-bo", got[0].ID)
}

func TestFilterSamples_IDMatch(t *testing.T) {
	samples := content.FlattenSamples(fixtureSamplePacks())
	got := content.FilterSamples(samples, "cap/schema")
	require.Len(t, got, 1)
	assert.Equal(t, "cap/schema", got[0].ID)
}

func TestFilterSamples_LabelMatch(t *testing.T) {
	samples := content.FlattenSamples(fixtureSamplePacks())
	got := content.FilterSamples(samples, "RAP Business")
	require.Len(t, got, 1)
	assert.Equal(t, "abap/rap-bo", got[0].ID)
}

func TestFilterSamples_CaseInsensitive(t *testing.T) {
	samples := content.FlattenSamples(fixtureSamplePacks())
	got := content.FilterSamples(samples, "ENTITY")
	require.Len(t, got, 1)
	assert.Equal(t, "cap/schema", got[0].ID)
}

func TestFilterSamples_NoMatch(t *testing.T) {
	samples := content.FlattenSamples(fixtureSamplePacks())
	got := content.FilterSamples(samples, "zzznomatch")
	assert.Empty(t, got)
}

func TestFindSample_Found(t *testing.T) {
	samples := content.FlattenSamples(fixtureSamplePacks())
	got := content.FindSample(samples, "cap/schema")
	require.NotNil(t, got)
	assert.Equal(t, "CDS data model", got.Label)
}

func TestFindSample_NotFound(t *testing.T) {
	samples := content.FlattenSamples(fixtureSamplePacks())
	got := content.FindSample(samples, "nonexistent/id")
	assert.Nil(t, got)
}
