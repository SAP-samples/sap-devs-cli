package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestApplyWeights_OrdersPacksByProfileWeight(t *testing.T) {
	packs := []*content.Pack{
		{ID: "abap", Weight: 90},
		{ID: "cap", Weight: 100},
		{ID: "fiori", Weight: 70},
	}
	profile := &content.Profile{
		Packs: []content.PackWeight{
			{ID: "fiori", Weight: 200},
			{ID: "cap", Weight: 50},
		},
	}
	ordered := content.ApplyWeights(packs, profile)
	assert.Equal(t, "fiori", ordered[0].ID)
	assert.Equal(t, "abap", ordered[1].ID)
	assert.Equal(t, "cap", ordered[2].ID)
}

func TestApplyWeights_NilProfileReturnsUnchanged(t *testing.T) {
	packs := []*content.Pack{{ID: "cap"}, {ID: "abap"}}
	result := content.ApplyWeights(packs, nil)
	assert.Equal(t, "cap", result[0].ID)
}

func TestLoadProfiles_ReadsAllYAML(t *testing.T) {
	dir := t.TempDir()
	yaml1 := "id: cap-developer\nname: CAP Developer\npacks:\n  - id: cap\n    weight: 100\n"
	yaml2 := "id: abap-developer\nname: ABAP Developer\npacks:\n  - id: abap\n    weight: 100\n"
	writeFile(t, filepath.Join(dir, "cap-developer.yaml"), yaml1)
	writeFile(t, filepath.Join(dir, "abap-developer.yaml"), yaml2)

	profiles, err := content.LoadProfiles(dir)
	require.NoError(t, err)
	assert.Len(t, profiles, 2)
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(data), 0644))
}
