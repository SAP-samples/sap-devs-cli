package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
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

func TestBuiltinProfiles_ContainsAllAndMinimal(t *testing.T) {
	profiles := content.BuiltinProfiles()
	require.Len(t, profiles, 2)
	ids := map[string]bool{}
	for _, p := range profiles {
		ids[p.ID] = true
		assert.NotEmpty(t, p.Name, "built-in profile %s must have a Name", p.ID)
		assert.NotEmpty(t, p.Description, "built-in profile %s must have a Description", p.ID)
	}
	assert.True(t, ids["all"], "built-in profiles must include 'all'")
	assert.True(t, ids["minimal"], "built-in profiles must include 'minimal'")
}

func TestIsBuiltinProfile_ReturnsTrueForReservedIDs(t *testing.T) {
	assert.True(t, content.IsBuiltinProfile("all"))
	assert.True(t, content.IsBuiltinProfile("minimal"))
	assert.False(t, content.IsBuiltinProfile("cap-developer"))
	assert.False(t, content.IsBuiltinProfile(""))
}

func TestContentLoaderLoadProfiles_IncludesBuiltins(t *testing.T) {
	// A loader with one official dir that has one file-backed profile.
	dir := t.TempDir()
	profilesDir := filepath.Join(dir, "profiles")
	require.NoError(t, os.MkdirAll(profilesDir, 0755))
	writeFile(t, filepath.Join(profilesDir, "cap-developer.yaml"),
		"id: cap-developer\nname: CAP Developer\npacks:\n  - id: cap\n    weight: 100\n")

	loader := &content.ContentLoader{OfficialDir: dir}
	profiles, err := loader.LoadProfiles()
	require.NoError(t, err)
	// 1 file-backed + 2 built-ins = 3 total
	assert.Len(t, profiles, 3)
	ids := map[string]bool{}
	for _, p := range profiles {
		ids[p.ID] = true
	}
	assert.True(t, ids["all"])
	assert.True(t, ids["minimal"])
	assert.True(t, ids["cap-developer"])
}

func TestContentLoaderFindProfile_ReturnsBuiltinForAll(t *testing.T) {
	// No files anywhere — built-in must be found regardless.
	loader := &content.ContentLoader{OfficialDir: t.TempDir()}
	p, err := loader.FindProfile("all")
	require.NoError(t, err)
	require.NotNil(t, p, "FindProfile('all') must return non-nil")
	assert.Equal(t, "all", p.ID)
}

func TestContentLoaderFindProfile_ReturnsBuiltinForMinimal(t *testing.T) {
	loader := &content.ContentLoader{OfficialDir: t.TempDir()}
	p, err := loader.FindProfile("minimal")
	require.NoError(t, err)
	require.NotNil(t, p, "FindProfile('minimal') must return non-nil")
	assert.Equal(t, "minimal", p.ID)
}

func TestContentLoaderLoadProfiles_BuiltinWinsOverFile(t *testing.T) {
	// A file named all.yaml must be dropped; the built-in wins.
	dir := t.TempDir()
	profilesDir := filepath.Join(dir, "profiles")
	require.NoError(t, os.MkdirAll(profilesDir, 0755))
	writeFile(t, filepath.Join(profilesDir, "all.yaml"),
		"id: all\nname: CUSTOM ALL\ndescription: custom\n")

	loader := &content.ContentLoader{OfficialDir: dir}
	profiles, err := loader.LoadProfiles()
	require.NoError(t, err)

	var allProfile *content.Profile
	for _, p := range profiles {
		if p.ID == "all" {
			allProfile = p
		}
	}
	require.NotNil(t, allProfile)
	// Built-in name wins — file-backed "CUSTOM ALL" is dropped.
	assert.Equal(t, "All Packs", allProfile.Name)
}
