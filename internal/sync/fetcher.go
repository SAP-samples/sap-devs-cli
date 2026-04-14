package sync

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// FetchArchive downloads a zip archive from rawURL and extracts it to destDir.
// If token is non-empty it is sent as an Authorization header.
// Existing files are overwritten; directories are created as needed.
// GitHub/GitLab archives include a top-level directory prefix which is stripped.
func FetchArchive(rawURL, destDir, token string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil) //nolint:gosec // URL comes from user config, not untrusted input
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: HTTP %d", rawURL, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// Auth redirect detection: if we ended up on the login page, surface a clear error.
	// Check resp.Request.URL (the final URL after redirects) for a /login path on the same host.
	if resp.Request != nil && resp.Request.URL != nil && resp.Request.URL.Host == parsedURL.Host && strings.Contains(resp.Request.URL.Path, "/login") {
		return fmt.Errorf("authentication required for %s — set GITHUB_TOOLS_SAP_TOKEN or run 'sap-devs config token'", parsedURL.Host)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	// Strip one leading path component (GitHub archives include repo-name-sha/ prefix)
	strip := zipStripPrefix(zr)

	absBase, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("resolve destDir: %w", err)
	}

	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, strip)
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		dest := filepath.Join(absBase, filepath.FromSlash(name))
		// Zip slip guard: ensure destination is within destDir
		if !strings.HasPrefix(dest, absBase+string(os.PathSeparator)) {
			return fmt.Errorf("zip slip blocked: %q escapes destination", f.Name)
		}
		if err := extractFile(f, dest); err != nil {
			return err
		}
	}
	return nil
}

func zipStripPrefix(zr *zip.Reader) string {
	for _, f := range zr.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) == 2 {
			return parts[0] + "/"
		}
	}
	return ""
}

func extractFile(f *zip.File, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("create dir for %s: %w", dest, err)
	}
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open zip entry %s: %w", f.Name, err)
	}
	defer rc.Close()
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer out.Close()
	if _, err = io.Copy(out, rc); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	return nil
}
