package sync_test

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
)

func TestFetcher_DownloadsAndExtractsZip(t *testing.T) {
	// Create an in-memory zip with one file
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, _ := w.Create("packs/cap/pack.yaml")
	f.Write([]byte("id: cap\nname: CAP\n"))
	w.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	dest := t.TempDir()
	err := sapSync.FetchArchive(srv.URL, dest)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dest, "packs", "cap", "pack.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "id: cap")
}
