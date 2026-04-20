package trayctl

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultRepoURL   = "https://github.com/SAP-samples/sap-devs-cli"
	maxDownloadBytes = 200 * 1024 * 1024 // 200 MB
)

// Manager handles downloading, verifying, starting, and stopping the tray binary.
type Manager struct {
	CacheDir string
	Token    string // optional GitHub token for authenticated downloads
	Version  string // CLI version — tray release must match
	RepoURL  string // defaults to defaultRepoURL if empty
}

func (m *Manager) repoURL() string {
	if m.RepoURL != "" {
		return m.RepoURL
	}
	return defaultRepoURL
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "sap-devs-tray.exe"
	}
	return "sap-devs-tray"
}

func assetName(version string) string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("sap-devs-tray_%s_%s_%s%s", version, runtime.GOOS, runtime.GOARCH, ext)
}

func (m *Manager) binDir() string {
	return filepath.Join(m.CacheDir, "bin")
}

// BinaryPath returns the full path to the tray binary.
func (m *Manager) BinaryPath() string {
	return filepath.Join(m.binDir(), binaryName())
}

// IsInstalled checks if the tray binary exists on disk.
func (m *Manager) IsInstalled() bool {
	_, err := os.Stat(m.BinaryPath())
	return err == nil
}

// Install downloads and verifies the tray binary from GitHub Releases.
// Stops any running tray instance before overwriting.
func (m *Manager) Install() error {
	if m.Version == "" || m.Version == "dev" {
		return fmt.Errorf("tray install requires a release build of sap-devs (current: %s)", m.Version)
	}

	if m.IsInstalled() {
		_ = m.Stop()
	}

	asset := assetName(m.Version)
	tagName := "v" + m.Version
	downloadURL := m.repoURL() + "/releases/download/" + tagName + "/"

	checksumData, err := httpGet(downloadURL+"tray-checksums.txt", m.Token)
	if err != nil {
		return fmt.Errorf("could not download tray-checksums.txt: %w", err)
	}
	expectedHash, err := findChecksum(checksumData, asset)
	if err != nil {
		return fmt.Errorf("tray binary not available for %s/%s in this release", runtime.GOOS, runtime.GOARCH)
	}

	archive, err := httpGet(downloadURL+asset, m.Token)
	if err != nil {
		return fmt.Errorf("could not download %s: %w", asset, err)
	}

	actual := sha256.Sum256(archive)
	if fmt.Sprintf("%x", actual) != expectedHash {
		return fmt.Errorf("checksum mismatch — download may be corrupt")
	}

	binBytes, err := extractBinary(archive, asset)
	if err != nil {
		return fmt.Errorf("could not extract binary: %w", err)
	}

	if err := os.MkdirAll(m.binDir(), 0755); err != nil {
		return err
	}
	path := m.BinaryPath()
	if err := os.WriteFile(path, binBytes, 0755); err != nil {
		return err
	}
	return nil
}

// Verify runs the tray binary with --version to check it executes successfully.
func (m *Manager) Verify() error {
	cmd := exec.Command(m.BinaryPath(), "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tray binary verification failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Uninstall stops the tray and removes the binary.
func (m *Manager) Uninstall() error {
	_ = m.Stop()
	return os.Remove(m.BinaryPath())
}

// Start launches the tray process in the background.
func (m *Manager) Start() error {
	if !m.IsInstalled() {
		return fmt.Errorf("tray is not installed — run `sap-devs tray install` first")
	}
	cmd := exec.Command(m.BinaryPath())
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// Stop terminates the running tray process.
func (m *Manager) Stop() error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("taskkill", "/im", "sap-devs-tray.exe", "/f")
	default:
		cmd = exec.Command("pkill", "-f", "sap-devs-tray")
	}
	return cmd.Run()
}

// IsRunning checks whether the tray process is currently running.
func (m *Manager) IsRunning() bool {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("tasklist", "/fi", "imagename eq sap-devs-tray.exe", "/fo", "csv", "/nh")
		out, err := cmd.Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), "sap-devs-tray.exe")
	default:
		return exec.Command("pgrep", "-f", "sap-devs-tray").Run() == nil
	}
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
	return "", fmt.Errorf("asset %s not found in checksums", assetName)
}

func extractBinary(data []byte, assetFileName string) ([]byte, error) {
	name := binaryName()
	if strings.HasSuffix(assetFileName, ".zip") {
		return extractFromZip(data, name)
	}
	return extractFromTarGz(data, name)
}
