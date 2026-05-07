//go:build windows

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type windowsScheduler struct {
	cacheDir string
}

func newPlatformScheduler(cacheDir string) Scheduler {
	return &windowsScheduler{cacheDir: cacheDir}
}

func (s *windowsScheduler) taskName() string { return "sap-devs-sync" }

func (s *windowsScheduler) logPath() string {
	return filepath.Join(s.cacheDir, "daemon.log")
}

func (s *windowsScheduler) scriptPath() string {
	return filepath.Join(s.cacheDir, "sap-devs-sync.cmd")
}

func (s *windowsScheduler) Install(interval time.Duration, binaryPath string) error {
	script := fmt.Sprintf("@echo off\r\n\"%s\" sync > \"%s\" 2>&1 && \"%s\" inject --no-sync >> \"%s\" 2>&1\r\n",
		binaryPath, s.logPath(), binaryPath, s.logPath())

	if err := os.MkdirAll(filepath.Dir(s.scriptPath()), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(s.scriptPath(), []byte(script), 0644); err != nil {
		return fmt.Errorf("write scheduler script: %w", err)
	}

	cmd := exec.Command("schtasks", "/create",
		"/tn", s.taskName(),
		"/tr", fmt.Sprintf(`"%s"`, s.scriptPath()),
		"/sc", "minute",
		"/mo", intervalMinutes(interval),
		"/f",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks create failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *windowsScheduler) Uninstall() error {
	cmd := exec.Command("schtasks", "/delete", "/tn", s.taskName(), "/f")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks delete failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	os.Remove(s.scriptPath())
	return nil
}

func (s *windowsScheduler) Status() (*Status, error) {
	cmd := exec.Command("schtasks", "/query", "/tn", s.taskName(), "/fo", "csv", "/nh")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &Status{Installed: false}, nil
	}
	fields := strings.Split(strings.TrimSpace(string(out)), ",")
	st := &Status{Installed: true}
	if len(fields) >= 3 {
		if t, err := time.Parse("1/2/2006 3:04:05 PM", strings.Trim(fields[2], "\"")); err == nil {
			st.NextRun = t
		}
	}
	return st, nil
}

func intervalMinutes(d time.Duration) string {
	mins := int(d.Minutes())
	if mins < 1 {
		mins = 1
	}
	return strconv.Itoa(mins)
}
