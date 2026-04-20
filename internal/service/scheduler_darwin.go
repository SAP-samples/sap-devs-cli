//go:build darwin

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type darwinScheduler struct {
	cacheDir string
}

func newPlatformScheduler(cacheDir string) Scheduler {
	return &darwinScheduler{cacheDir: cacheDir}
}

const plistLabel = "com.sap-devs.sync"

func (s *darwinScheduler) plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist")
}

func (s *darwinScheduler) logPath() string {
	return filepath.Join(s.cacheDir, "daemon.log")
}

func (s *darwinScheduler) generatePlist(interval time.Duration, binaryPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>/bin/sh</string>
		<string>-c</string>
		<string>%s sync &amp;&amp; %s inject --no-sync</string>
	</array>
	<key>StartInterval</key>
	<integer>%d</integer>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>`, plistLabel, binaryPath, binaryPath, int(interval.Seconds()), s.logPath(), s.logPath())
}

func (s *darwinScheduler) Install(interval time.Duration, binaryPath string) error {
	plist := s.generatePlist(interval, binaryPath)
	path := s.plistPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(plist), 0644); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "unload", path).Run()
	cmd := exec.Command("launchctl", "load", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl load failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *darwinScheduler) Uninstall() error {
	path := s.plistPath()
	cmd := exec.Command("launchctl", "unload", path)
	_ = cmd.Run() // ignore error if not loaded
	return os.Remove(path)
}

func (s *darwinScheduler) Status() (*Status, error) {
	path := s.plistPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Status{Installed: false}, nil
	}
	cmd := exec.Command("launchctl", "list", plistLabel)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &Status{Installed: true}, nil
	}
	_ = out // launchctl list output is informational
	return &Status{Installed: true}, nil
}
