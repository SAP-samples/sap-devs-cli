package trayctl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func autostartEntryName() string {
	switch runtime.GOOS {
	case "windows":
		return "sap-devs-tray"
	case "darwin":
		return "com.sap-devs.tray"
	default:
		return "sap-devs-tray.desktop"
	}
}

func (m *Manager) RegisterAutostart() error {
	binaryPath := m.BinaryPath()
	switch runtime.GOOS {
	case "windows":
		return registerWindowsAutostart(binaryPath)
	case "darwin":
		return registerDarwinAutostart(binaryPath)
	case "linux":
		return registerLinuxAutostart(binaryPath)
	default:
		return fmt.Errorf("autostart not supported on %s", runtime.GOOS)
	}
}

func (m *Manager) UnregisterAutostart() error {
	switch runtime.GOOS {
	case "windows":
		return unregisterWindowsAutostart()
	case "darwin":
		return unregisterDarwinAutostart()
	case "linux":
		return unregisterLinuxAutostart()
	default:
		return nil
	}
}

func registerWindowsAutostart(binaryPath string) error {
	cmd := exec.Command("reg", "add",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", "sap-devs-tray",
		"/t", "REG_SZ",
		"/d", binaryPath,
		"/f",
	)
	return cmd.Run()
}

func unregisterWindowsAutostart() error {
	cmd := exec.Command("reg", "delete",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", "sap-devs-tray",
		"/f",
	)
	return cmd.Run()
}

func registerDarwinAutostart(binaryPath string) error {
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.sap-devs.tray</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>`, binaryPath)

	home, _ := os.UserHomeDir()
	path := filepath.Join(home, "Library", "LaunchAgents", "com.sap-devs.tray.plist")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(plist), 0644)
}

func unregisterDarwinAutostart() error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, "Library", "LaunchAgents", "com.sap-devs.tray.plist")
	_ = exec.Command("launchctl", "unload", path).Run()
	return os.Remove(path)
}

func registerLinuxAutostart(binaryPath string) error {
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=sap-devs Tray
Exec=%s
Terminal=false
StartupNotify=false
X-GNOME-Autostart-enabled=true
`, binaryPath)

	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "autostart")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "sap-devs-tray.desktop"), []byte(entry), 0644)
}

func unregisterLinuxAutostart() error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".config", "autostart", "sap-devs-tray.desktop")
	return os.Remove(path)
}
