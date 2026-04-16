package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestContentLoader_LoadPacks_MergesLayers(t *testing.T) {
	official := makeTempPacksDir(t, map[string]string{
		"cap":  "id: cap\nname: CAP Official\nweight: 100\n",
		"abap": "id: abap\nname: ABAP\nweight: 90\n",
	})
	company := makeTempPacksDir(t, map[string]string{
		"cap": "id: cap\nname: CAP Company Override\nweight: 100\n",
	})

	loader := &content.ContentLoader{
		OfficialDir: official,
		CompanyDir:  company,
	}
	packs, err := loader.LoadPacks(nil, "")
	require.NoError(t, err)
	assert.Len(t, packs, 2)
	capPack := findPack(packs, "cap")
	require.NotNil(t, capPack)
	assert.Equal(t, "CAP Company Override", capPack.Name)
}

func findPack(packs []*content.Pack, id string) *content.Pack {
	for _, p := range packs {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func makeTempPacksDir(t *testing.T, packs map[string]string) string {
	t.Helper()
	root := t.TempDir()
	packsDir := filepath.Join(root, "packs")
	require.NoError(t, os.MkdirAll(packsDir, 0755))
	for id, yaml := range packs {
		packDir := filepath.Join(packsDir, id)
		require.NoError(t, os.MkdirAll(packDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(packDir, "pack.yaml"), []byte(yaml), 0644))
	}
	return root
}

func TestLoadPack_BaseField_TrueWhenSet(t *testing.T) {
	dir := makeTempPacksDir(t, map[string]string{
		"base": "id: base\nname: Base\nweight: 0\nbase: true\n",
	})
	pack, err := content.LoadPack(filepath.Join(dir, "packs", "base"), "")
	require.NoError(t, err)
	assert.True(t, pack.Base, "pack.Base should be true when base: true in pack.yaml")
}

func TestLoadPack_BaseField_FalseByDefault(t *testing.T) {
	dir := makeTempPacksDir(t, map[string]string{
		"cap": "id: cap\nname: CAP\nweight: 100\n",
	})
	pack, err := content.LoadPack(filepath.Join(dir, "packs", "cap"), "")
	require.NoError(t, err)
	assert.False(t, pack.Base, "pack.Base should be false when base field is absent")
}

func TestContentLoader_LoadPacks_BasePackFirst_RegardlessOfWeight(t *testing.T) {
	// base pack has weight 0 (lowest), but must always appear first
	dir := makeTempPacksDir(t, map[string]string{
		"base": "id: base\nname: Base\nweight: 0\nbase: true\n",
		"cap":  "id: cap\nname: CAP\nweight: 100\n",
		"abap": "id: abap\nname: ABAP\nweight: 90\n",
	})
	loader := &content.ContentLoader{OfficialDir: dir}
	packs, err := loader.LoadPacks(nil, "")
	require.NoError(t, err)
	require.Len(t, packs, 3)
	assert.Equal(t, "base", packs[0].ID, "base pack must be first regardless of weight")
}

func TestContentLoader_LoadPacks_MultipleBasePacks_AllFirst(t *testing.T) {
	dir := makeTempPacksDir(t, map[string]string{
		"base1": "id: base1\nname: Base 1\nweight: 50\nbase: true\n",
		"base2": "id: base2\nname: Base 2\nweight: 10\nbase: true\n",
		"cap":   "id: cap\nname: CAP\nweight: 100\n",
	})
	loader := &content.ContentLoader{OfficialDir: dir}
	packs, err := loader.LoadPacks(nil, "")
	require.NoError(t, err)
	require.Len(t, packs, 3)
	assert.True(t, packs[0].Base, "first pack must be base")
	assert.True(t, packs[1].Base, "second pack must be base")
	assert.False(t, packs[2].Base, "third pack must be non-base")
	// base packs are ordered by weight among themselves
	assert.Equal(t, "base1", packs[0].ID, "higher-weight base pack first")
}

func TestContentLoader_LoadPacks_NoBasePacks_OrderUnchanged(t *testing.T) {
	dir := makeTempPacksDir(t, map[string]string{
		"cap":  "id: cap\nname: CAP\nweight: 100\n",
		"abap": "id: abap\nname: ABAP\nweight: 90\n",
	})
	loader := &content.ContentLoader{OfficialDir: dir}
	packs, err := loader.LoadPacks(nil, "")
	require.NoError(t, err)
	require.Len(t, packs, 2)
	// weight ordering unchanged when no base packs
	assert.Equal(t, "cap", packs[0].ID)
	assert.Equal(t, "abap", packs[1].ID)
}

func TestContentLoader_LoadPacks_MinimalProfile_BasePacksOnly(t *testing.T) {
	dir := makeTempPacksDir(t, map[string]string{
		"base": "id: base\nname: Base\nweight: 0\nbase: true\n",
		"cap":  "id: cap\nname: CAP\nweight: 100\n",
	})
	loader := &content.ContentLoader{OfficialDir: dir}
	minimal := &content.Profile{ID: "minimal", Name: "Minimal"}
	packs, err := loader.LoadPacks(minimal, "")
	require.NoError(t, err)
	require.Len(t, packs, 1, "minimal profile must return only base packs")
	assert.Equal(t, "base", packs[0].ID)
}

func TestContentLoader_LoadPacks_AllProfile_AllPacksReturned(t *testing.T) {
	dir := makeTempPacksDir(t, map[string]string{
		"base": "id: base\nname: Base\nweight: 0\nbase: true\n",
		"cap":  "id: cap\nname: CAP\nweight: 100\n",
	})
	loader := &content.ContentLoader{OfficialDir: dir}
	all := &content.Profile{ID: "all", Name: "All Packs"}
	packs, err := loader.LoadPacks(all, "")
	require.NoError(t, err)
	require.Len(t, packs, 2, "all profile must return all packs")
	// base pack must still be first
	assert.Equal(t, "base", packs[0].ID)
	assert.Equal(t, "cap", packs[1].ID)
}

func TestContentLoader_LoadPacks_AdditiveLayer(t *testing.T) {
	// Official layer: cap pack with one tip and one resource
	official := makeLayerDir(t, map[string]packFixture{
		"cap": {
			yaml:      "id: cap\nname: CAP Official\ndescription: Official\ntags: [official]\nweight: 100\n",
			context:   "Official context",
			tips:      "## Official Tip\nOfficial tip content",
			resources: "- id: cap/docs\n  title: Official Docs\n  url: https://official.example\n  type: official-docs\n  tags: []\n",
		},
	})
	// Company layer: additive pack for cap — adds a tip and a resource
	company := makeLayerDir(t, map[string]packFixture{
		"cap": {
			yaml:      "id: cap\nname: \ndescription: \ntags: [company]\nweight: 0\nadditive: true\nadditive_position: after\n",
			context:   "Company context",
			tips:      "## Company Tip\nCompany tip content",
			resources: "- id: cap/company-guide\n  title: Company Guide\n  url: https://company.example\n  type: official-docs\n  tags: []\n",
		},
	})
	// Project layer: another additive pack — overrides name, adds one more tip
	project := makeLayerDir(t, map[string]packFixture{
		"cap": {
			yaml: "id: cap\nname: CAP Project\ndescription: \ntags: []\nweight: 0\nadditive: true\nadditive_position: after\n",
			tips: "## Project Tip\nProject tip content",
		},
	})

	loader := &content.ContentLoader{
		OfficialDir: official,
		CompanyDir:  company,
		ProjectDir:  project,
	}
	packs, err := loader.LoadPacks(nil, "")
	require.NoError(t, err)

	cap := findPack(packs, "cap")
	require.NotNil(t, cap)

	// Name: company layer had empty name — base name preserved; project overrides with non-empty
	assert.Equal(t, "CAP Project", cap.Name)

	// Context: official + company appended (after); project has no context file — no change
	assert.Equal(t, "Official context\n\nCompany context", cap.ContextMD)

	// Tips: all three in order (official, company, project)
	require.Len(t, cap.Tips, 3)
	assert.Equal(t, "Official Tip", cap.Tips[0].Title)
	assert.Equal(t, "Company Tip", cap.Tips[1].Title)
	assert.Equal(t, "Project Tip", cap.Tips[2].Title)

	// Resources: official + company appended; all re-stamped to base pack ID
	require.Len(t, cap.Resources, 2)
	assert.Equal(t, "cap", cap.Resources[0].PackID)
	assert.Equal(t, "cap", cap.Resources[1].PackID)

	// Tags: union of all layers
	assert.Contains(t, cap.Tags, "official")
	assert.Contains(t, cap.Tags, "company")
}

func TestContentLoader_LoadPacks_AdditiveNoBase(t *testing.T) {
	// Additive pack with no matching base — becomes the base as-is, Additive cleared
	company := makeLayerDir(t, map[string]packFixture{
		"new-pack": {
			yaml: "id: new-pack\nname: New Pack\ndescription: desc\ntags: []\nweight: 50\nadditive: true\nadditive_position: after\n",
		},
	})
	loader := &content.ContentLoader{OfficialDir: t.TempDir(), CompanyDir: company}
	packs, err := loader.LoadPacks(nil, "")
	require.NoError(t, err)
	p := findPack(packs, "new-pack")
	require.NotNil(t, p)
	assert.False(t, p.Additive, "Additive flag must be cleared in no-base path")
}

func TestContentLoader_LoadPacks_NonAdditiveOverridesAdditiveResult(t *testing.T) {
	// A non-additive pack in a later layer fully replaces an earlier additive-merged result.
	// This tests the design intent: additive accumulation can be "reset" by a plain replace.
	official := makeLayerDir(t, map[string]packFixture{
		"cap": {
			yaml:    "id: cap\nname: CAP Official\ntags: [official]\nweight: 100\n",
			context: "Official context",
		},
	})
	company := makeLayerDir(t, map[string]packFixture{
		"cap": {
			yaml:    "id: cap\nname: \ntags: [company]\nweight: 0\nadditive: true\nadditive_position: after\n",
			context: "Company context",
		},
	})
	// Project pack is non-additive — fully replaces the official+company merged result
	project := makeLayerDir(t, map[string]packFixture{
		"cap": {
			yaml:    "id: cap\nname: CAP Project Override\ntags: [project]\nweight: 0\n",
			context: "Project only context",
		},
	})

	loader := &content.ContentLoader{
		OfficialDir: official,
		CompanyDir:  company,
		ProjectDir:  project,
	}
	packs, err := loader.LoadPacks(nil, "")
	require.NoError(t, err)

	cap := findPack(packs, "cap")
	require.NotNil(t, cap)

	// Non-additive project layer wins — none of official or company content
	assert.Equal(t, "CAP Project Override", cap.Name)
	assert.Equal(t, "Project only context", cap.ContextMD)
	assert.NotContains(t, cap.Tags, "official")
	assert.NotContains(t, cap.Tags, "company")
	assert.Contains(t, cap.Tags, "project")
}

func TestContentLoader_LoadPacks_AdditivePositionBefore(t *testing.T) {
	// Verify additive_position: before prepends content rather than appending.
	official := makeLayerDir(t, map[string]packFixture{
		"cap": {
			yaml:    "id: cap\nname: CAP\ntags: []\nweight: 100\n",
			context: "Official context",
			tips:    "## Official Tip\nOfficial tip content",
		},
	})
	company := makeLayerDir(t, map[string]packFixture{
		"cap": {
			yaml:    "id: cap\nname: \ntags: []\nweight: 0\nadditive: true\nadditive_position: before\n",
			context: "Company context",
			tips:    "## Company Tip\nCompany tip content",
		},
	})

	loader := &content.ContentLoader{OfficialDir: official, CompanyDir: company}
	packs, err := loader.LoadPacks(nil, "")
	require.NoError(t, err)

	cap := findPack(packs, "cap")
	require.NotNil(t, cap)

	// Context: company prepended before official
	assert.Equal(t, "Company context\n\nOfficial context", cap.ContextMD)

	// Tips: company tip first, then official
	require.Len(t, cap.Tips, 2)
	assert.Equal(t, "Company Tip", cap.Tips[0].Title)
	assert.Equal(t, "Official Tip", cap.Tips[1].Title)
}

// packFixture holds optional file content for makeLayerDir.
type packFixture struct {
	yaml      string
	context   string
	tips      string
	resources string
}

// makeLayerDir creates a temporary content directory with packs as subdirectories
// under <root>/packs/. Each pack can have pack.yaml, context.md, tips.md, and resources.yaml.
func makeLayerDir(t *testing.T, packs map[string]packFixture) string {
	t.Helper()
	root := t.TempDir()
	packsDir := filepath.Join(root, "packs")
	require.NoError(t, os.MkdirAll(packsDir, 0755))
	for id, f := range packs {
		dir := filepath.Join(packsDir, id)
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(f.yaml), 0644))
		if f.context != "" {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "context.md"), []byte(f.context), 0644))
		}
		if f.tips != "" {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "tips.md"), []byte(f.tips), 0644))
		}
		if f.resources != "" {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "resources.yaml"), []byte(f.resources), 0644))
		}
	}
	return root
}
