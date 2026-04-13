package sync

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// FetchArchive downloads a zip archive from url and extracts it to destDir.
// Existing files are overwritten; directories are created as needed.
// GitHub/GitLab archives include a top-level directory prefix which is stripped.
func FetchArchive(url, destDir string) error {
	resp, err := http.Get(url) //nolint:gosec // URL comes from user config, not untrusted input
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	// Strip one leading path component (GitHub archives include repo-name-sha/ prefix)
	strip := zipStripPrefix(zr)

	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, strip)
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		dest := filepath.Join(destDir, filepath.FromSlash(name))
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
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}
