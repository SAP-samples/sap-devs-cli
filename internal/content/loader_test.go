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
