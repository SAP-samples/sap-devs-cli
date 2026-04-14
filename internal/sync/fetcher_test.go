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
	f, _ := w.Create("sap-devs-cli-main/content/packs/cap/pack.yaml")
	f.Write([]byte("id: cap\nname: CAP\n"))
	w.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	dest := t.TempDir()
	err := sapSync.FetchArchive(srv.URL, dest, "")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dest, "content", "packs", "cap", "pack.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "id: cap")
}

func TestFetcher_BlocksZipSlip(t *testing.T) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	// Crafted entry that tries to escape the destination directory
	f, _ := w.Create("repo-abc123/../../evil.txt")
	f.Write([]byte("malicious content"))
	w.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	dest := t.TempDir()
	err := sapSync.FetchArchive(srv.URL, dest, "")
	assert.Error(t, err, "zip slip should be blocked")
	assert.Contains(t, err.Error(), "zip slip blocked")
}

func TestFetcher_AuthRedirectReturnsActionableError(t *testing.T) {
	// Server that redirects to /login on the same host (simulates GHE auth wall)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>Login page</html>"))
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer srv.Close()

	dest := t.TempDir()
	err := sapSync.FetchArchive(srv.URL+"/repo.zip", dest, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication required")
	assert.Contains(t, err.Error(), "GITHUB_TOOLS_SAP_TOKEN")
	assert.Contains(t, err.Error(), "sap-devs config token")
}

func TestFetcher_SendsAuthHeader(t *testing.T) {
	var gotAuth string
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, _ := w.Create("repo-main/content/pack.yaml")
	f.Write([]byte("id: test\n"))
	w.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/zip")
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	dest := t.TempDir()
	err := sapSync.FetchArchive(srv.URL, dest, "mytoken")
	require.NoError(t, err)
	assert.Equal(t, "token mytoken", gotAuth)
}

func TestFetcher_NoAuthHeaderWhenTokenEmpty(t *testing.T) {
	var gotAuth string
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, _ := w.Create("repo-main/content/pack.yaml")
	f.Write([]byte("id: test\n"))
	w.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/zip")
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	dest := t.TempDir()
	require.NoError(t, sapSync.FetchArchive(srv.URL, dest, ""))
	assert.Equal(t, "", gotAuth)
}
