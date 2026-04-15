package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// maxDownloadBytes caps downloads to prevent memory exhaustion on malicious/misconfigured servers.
const maxDownloadBytes = 100 * 1024 * 1024 // 100 MB

// downloadBase overrides the releases download URL in tests.
// When empty, repoURL is used as the base.
var downloadBase string

// executableFn is os.Executable by default; overridden in tests.
var executableFn = os.Executable

// Install downloads the release asset for the current OS/arch, verifies its
// SHA256 checksum against checksums.txt, and replaces the running binary.
// token is an optional Bearer token for GitHub Enterprise authentication; pass "" if not needed.
func Install(repoURL string, release *Release, token string) error {
	currentPath, err := executableFn()
	if err != nil {
		return fmt.Errorf("could not determine binary path: %w", err)
	}

	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	assetName := fmt.Sprintf("sap-devs_%s_%s_%s%s", release.Version, runtime.GOOS, runtime.GOARCH, ext)

	base := downloadBase
	if base == "" {
		base = repoURL
	}
	downloadURL := base + "/releases/download/" + release.TagName + "/"

	// Download checksums first to verify platform support before downloading archive
	checksumData, err := httpGet(downloadURL+"checksums.txt", token)
	if err != nil {
		return fmt.Errorf("could not download checksums.txt: %w", err)
	}

	// Check platform is supported before downloading the (potentially large) archive
	expectedHash, err := findChecksum(checksumData, assetName)
	if err != nil {
		return err // "no release asset found for ..."
	}

	// Download archive
	archive, err := httpGet(downloadURL+assetName, token)
	if err != nil {
		return fmt.Errorf("could not download %s: %w", assetName, err)
	}

	// Verify SHA256
	actual := sha256.Sum256(archive)
	actualHex := fmt.Sprintf("%x", actual)
	if actualHex != expectedHash {
		return fmt.Errorf("checksum mismatch — download may be corrupt")
	}

	// Extract binary from archive
	binBytes, err := extractBinary(archive, ext)
	if err != nil {
		return fmt.Errorf("could not extract binary: %w", err)
	}

	// Write to temp file in same directory as current binary
	dir := filepath.Dir(currentPath)
	tmp, err := os.CreateTemp(dir, "sap-devs-update-*")
	if err != nil {
		return fmt.Errorf("could not create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(binBytes); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("could not write temp file: %w", err)
	}
	tmp.Close()
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	// Replace binary (platform-specific)
	if runtime.GOOS == "windows" {
		// Windows locks running executables; remove then rename
		if err := os.Remove(currentPath); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("could not remove old binary: %w", err)
		}
	}
	if err := os.Rename(tmpPath, currentPath); err != nil {
		// On Windows: original already removed, tmpPath still exists
		return fmt.Errorf("could not replace binary: %w", err)
	}
	return nil
}

func httpGet(url, token string) ([]byte, error) {
	client := &http.Client{Timeout: 300 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes))
}

// findChecksum parses checksums.txt and returns the SHA256 hex for assetName.
// Format: "<hex>  <filename>" per line.
func findChecksum(data []byte, assetName string) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == assetName {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("no release asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// extractBinary extracts the binary named "sap-devs" (or "sap-devs.exe" on Windows)
// from the archive bytes. ext is ".tar.gz" or ".zip".
func extractBinary(data []byte, ext string) ([]byte, error) {
	binName := "sap-devs"
	if runtime.GOOS == "windows" {
		binName = "sap-devs.exe"
	}
	if ext == ".zip" {
		return extractFromZip(data, binName)
	}
	return extractFromTarGz(data, binName)
}

func extractFromTarGz(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(hdr.Name) == name {
			return io.ReadAll(io.LimitReader(tr, maxDownloadBytes))
		}
	}
	return nil, fmt.Errorf("binary %q not found in archive", name)
}

func extractFromZip(data []byte, name string) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range r.File {
		if filepath.Base(f.Name) == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(io.LimitReader(rc, maxDownloadBytes))
			rc.Close()
			return data, err
		}
	}
	return nil, fmt.Errorf("binary %q not found in zip archive", name)
}
