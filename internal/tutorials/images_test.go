package tutorials_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractImageRefs_RelativePaths(t *testing.T) {
	content := "Some text\n![cds commands](cds_commands.png)\nMore text\n![folder](folder_structure.png)\n"
	refs := tutorials.ExtractImageRefs(content, "btp-adai", "main", "cp-apm-nodejs-create-service")
	assert.Len(t, refs, 2)
	assert.Equal(t, "cds commands", refs[0].Alt)
	assert.Equal(t, "cds_commands.png", refs[0].OriginalPath)
	assert.Equal(t, "https://raw.githubusercontent.com/sap-tutorials/btp-adai/main/tutorials/cp-apm-nodejs-create-service/cds_commands.png", refs[0].URL)
}

func TestExtractImageRefs_AbsoluteURLs(t *testing.T) {
	content := "![logo](https://example.com/logo.png)\n"
	refs := tutorials.ExtractImageRefs(content, "repo", "main", "slug")
	assert.Len(t, refs, 1)
	assert.Equal(t, "https://example.com/logo.png", refs[0].URL)
	assert.Equal(t, "https://example.com/logo.png", refs[0].OriginalPath)
}

func TestExtractImageRefs_SkipsTraversals(t *testing.T) {
	content := "![bad](../../secret.png)\n"
	refs := tutorials.ExtractImageRefs(content, "repo", "main", "slug")
	assert.Len(t, refs, 1)
	assert.Equal(t, "../../secret.png", refs[0].URL) // not resolved
}

func TestExtractImageRefs_NoImages(t *testing.T) {
	content := "Just text, no images here.\n"
	refs := tutorials.ExtractImageRefs(content, "repo", "main", "slug")
	assert.Empty(t, refs)
}

// 1x1 red PNG for testing
var testPNG = func() []byte {
	b, _ := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==")
	return b
}()

func TestFetchImage_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(testPNG)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	img, err := tutorials.FetchImage(srv.URL+"/test.png", cacheDir, "my-slug")
	require.NoError(t, err)
	assert.Equal(t, "image/png", img.MIMEType)
	assert.NotEmpty(t, img.Data)

	decoded, err := base64.StdEncoding.DecodeString(img.Data)
	require.NoError(t, err)
	assert.Equal(t, testPNG, decoded)
}

func TestFetchImage_CacheHit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(testPNG)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()

	// First fetch — populates cache
	_, err := tutorials.FetchImage(srv.URL+"/test.png", cacheDir, "my-slug")
	require.NoError(t, err)
	srv.Close() // kill server

	// Second fetch — must come from cache
	img, err := tutorials.FetchImage(srv.URL+"/test.png", cacheDir, "my-slug")
	require.NoError(t, err)
	assert.Equal(t, "image/png", img.MIMEType)
	assert.NotEmpty(t, img.Data)
}

func TestFetchImage_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	_, err := tutorials.FetchImage(srv.URL+"/missing.png", cacheDir, "my-slug")
	assert.Error(t, err)
}

func TestFetchStepImages_Mixed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(testPNG)
	}))
	defer srv.Close()

	refs := []tutorials.ImageRef{
		{Alt: "img1", URL: srv.URL + "/a.png"},
		{Alt: "img2", URL: srv.URL + "/b.png"},
	}
	cacheDir := t.TempDir()
	images := tutorials.FetchStepImages(refs, cacheDir, "my-slug")
	assert.Len(t, images, 2)
	assert.Equal(t, "img1", images[0].Alt)
	assert.NotEmpty(t, images[0].Data)
}
