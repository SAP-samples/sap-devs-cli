//go:build windows

package trayctl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (m *Manager) CreateShortcuts() error {
	target := m.BinaryPath()
	workDir := m.binDir()
	iconPath := filepath.Join(m.binDir(), "sap-devs-tray.ico")

	startMenuDir := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs")
	if err := createLnk(filepath.Join(startMenuDir, "SAP Devs Tray.lnk"), target, workDir, iconPath); err != nil {
		return fmt.Errorf("start menu shortcut: %w", err)
	}

	desktopPath, err := resolveDesktopPath()
	if err != nil {
		return fmt.Errorf("resolve desktop path: %w", err)
	}
	if err := createLnk(filepath.Join(desktopPath, "SAP Devs Tray.lnk"), target, workDir, iconPath); err != nil {
		return fmt.Errorf("desktop shortcut: %w", err)
	}
	return nil
}

func (m *Manager) RemoveShortcuts() error {
	startMenuLink := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "SAP Devs Tray.lnk")
	_ = os.Remove(startMenuLink)

	desktopPath, _ := resolveDesktopPath()
	if desktopPath != "" {
		_ = os.Remove(filepath.Join(desktopPath, "SAP Devs Tray.lnk"))
	}

	_ = os.Remove(filepath.Join(m.binDir(), "sap-devs-tray.ico"))
	return nil
}

func resolveDesktopPath() (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"[Environment]::GetFolderPath('Desktop')")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", fmt.Errorf("could not resolve Desktop folder")
	}
	return path, nil
}

func createLnk(lnkPath, target, workDir, iconPath string) error {
	script := fmt.Sprintf(`
$ws = New-Object -ComObject WScript.Shell
$s = $ws.CreateShortcut('%s')
$s.TargetPath = '%s'
$s.WorkingDirectory = '%s'
$s.IconLocation = '%s,0'
$s.Save()
`, lnkPath, target, workDir, iconPath)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
