package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// makeTarGz creates an in-memory tar.gz containing one file named `name` with `content`.
func makeTarGz(name string, content []byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))})
	tw.Write(content)
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

// makeZip creates an in-memory zip archive containing one file named `name` with `content`.
func makeZip(name string, content []byte) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create(name)
	f.Write(content)
	zw.Close()
	return buf.Bytes()
}

// platformExt returns the archive extension for the current OS, matching installer.go logic.
func platformExt() string {
	if runtime.GOOS == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}

// setupInstallServer creates an httptest server serving a platform-appropriate archive and checksums.txt.
// Returns the server, the asset name, and the hex SHA256 of the archive.
func setupInstallServer(t *testing.T, version string) (*httptest.Server, string) {
	t.Helper()
	binaryContent := []byte("fake-binary-content")
	ext := platformExt()
	assetName := fmt.Sprintf("sap-devs_%s_%s_%s%s", version, runtime.GOOS, runtime.GOARCH, ext)
	var archive []byte
	if ext == ".zip" {
		archive = makeZip("sap-devs.exe", binaryContent)
	} else {
		archive = makeTarGz("sap-devs", binaryContent)
	}
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%x  %s\n", sum, assetName)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			w.Write([]byte(checksums))
		case strings.HasSuffix(r.URL.Path, assetName):
			w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv, assetName
}

func TestInstall_Success(t *testing.T) {
	version := "9.9.9"
	srv, _ := setupInstallServer(t, version)
	defer srv.Close()
	downloadBase = srv.URL

	// Create a temp file to act as the "current binary"
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "sap-devs")
	os.WriteFile(fakeBin, []byte("old"), 0o755)
	executableFn = func() (string, error) { return fakeBin, nil }
	t.Cleanup(func() { executableFn = os.Executable; downloadBase = "" })

	rel := &Release{Version: version, TagName: "v" + version}
	if err := Install("https://example.com/owner/repo", rel); err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	got, _ := os.ReadFile(fakeBin)
	if string(got) != "fake-binary-content" {
		t.Fatalf("binary not replaced: got %q", got)
	}
}

func TestInstall_ChecksumMismatch(t *testing.T) {
	version := "9.9.9"
	ext := platformExt()
	assetName := fmt.Sprintf("sap-devs_%s_%s_%s%s", version, runtime.GOOS, runtime.GOARCH, ext)
	var archive []byte
	if ext == ".zip" {
		archive = makeZip("sap-devs.exe", []byte("fake-binary"))
	} else {
		archive = makeTarGz("sap-devs", []byte("fake-binary"))
	}
	// Wrong hash in checksums.txt
	checksums := fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte("wrong")), assetName)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			w.Write([]byte(checksums))
		default:
			w.Write(archive)
		}
	}))
	defer srv.Close()
	downloadBase = srv.URL

	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "sap-devs")
	os.WriteFile(fakeBin, []byte("old"), 0o755)
	executableFn = func() (string, error) { return fakeBin, nil }
	t.Cleanup(func() { executableFn = os.Executable; downloadBase = "" })

	rel := &Release{Version: version, TagName: "v" + version}
	err := Install("https://example.com/owner/repo", rel)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch error, got: %v", err)
	}
}

func TestInstall_UnsupportedPlatform(t *testing.T) {
	version := "9.9.9"
	// checksums.txt mentions a different platform, not current GOOS/GOARCH
	checksums := "abcd1234  sap-devs_9.9.9_plan9_mips.tar.gz\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "checksums.txt") {
			w.Write([]byte(checksums))
		}
	}))
	defer srv.Close()
	downloadBase = srv.URL

	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "sap-devs")
	os.WriteFile(fakeBin, []byte("old"), 0o755)
	executableFn = func() (string, error) { return fakeBin, nil }
	t.Cleanup(func() { executableFn = os.Executable; downloadBase = "" })

	rel := &Release{Version: version, TagName: "v" + version}
	err := Install("https://example.com/owner/repo", rel)
	if err == nil || !strings.Contains(err.Error(), "no release asset found") {
		t.Fatalf("expected 'no release asset found' error, got: %v", err)
	}
}
