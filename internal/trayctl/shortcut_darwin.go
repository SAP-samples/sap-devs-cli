//go:build darwin

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
	appDir := filepath.Join(home, "Applications", "SAP Devs Tray.app", "Contents")
	macosDir := filepath.Join(appDir, "MacOS")
	resDir := filepath.Join(appDir, "Resources")

	for _, d := range []string{macosDir, resDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	symlinkPath := filepath.Join(macosDir, "sap-devs-tray")
	_ = os.Remove(symlinkPath)
	if err := os.Symlink(m.BinaryPath(), symlinkPath); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}

	icnsSource := filepath.Join(m.binDir(), "icon.icns")
	icnsDest := filepath.Join(resDir, "AppIcon.icns")
	if data, err := os.ReadFile(icnsSource); err == nil {
		_ = os.WriteFile(icnsDest, data, 0644)
	}

	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>SAP Devs Tray</string>
	<key>CFBundleIdentifier</key>
	<string>com.sap-devs.tray</string>
	<key>CFBundleExecutable</key>
	<string>sap-devs-tray</string>
	<key>CFBundleIconFile</key>
	<string>AppIcon</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>LSUIElement</key>
	<true/>
	<key>LSBackgroundOnly</key>
	<false/>
</dict>
</plist>`

	return os.WriteFile(filepath.Join(appDir, "Info.plist"), []byte(plist), 0644)
}

func (m *Manager) RemoveShortcuts() error {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}
	_ = os.RemoveAll(filepath.Join(home, "Applications", "SAP Devs Tray.app"))
	_ = os.Remove(filepath.Join(m.binDir(), "icon.icns"))
	return nil
}
