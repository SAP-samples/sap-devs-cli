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
