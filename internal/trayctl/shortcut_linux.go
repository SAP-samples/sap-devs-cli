//go:build linux

package trayctl

import (
	"fmt"
	"os"
	"path/filepath"
)

func (m *Manager) CreateShortcuts() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	iconPath := filepath.Join(m.binDir(), "sap-devs-tray.png")
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=SAP Devs Tray
Comment=SAP developer tools system tray companion
Exec=%s
Icon=%s
Terminal=false
Categories=Development;
StartupNotify=false
`, m.BinaryPath(), iconPath)

	appsDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(appsDir, "sap-devs-tray.desktop"), []byte(entry), 0644); err != nil {
		return err
	}

	desktopDir := filepath.Join(home, "Desktop")
	if info, err := os.Stat(desktopDir); err == nil && info.IsDir() {
		desktopFile := filepath.Join(desktopDir, "sap-devs-tray.desktop")
		if err := os.WriteFile(desktopFile, []byte(entry), 0755); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) RemoveShortcuts() error {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}
	_ = os.Remove(filepath.Join(home, ".local", "share", "applications", "sap-devs-tray.desktop"))
	_ = os.Remove(filepath.Join(home, "Desktop", "sap-devs-tray.desktop"))
	_ = os.Remove(filepath.Join(m.binDir(), "sap-devs-tray.png"))
	return nil
}
