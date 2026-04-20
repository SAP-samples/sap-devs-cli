//go:build windows

package service

import (
	"fmt"
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

func (s *windowsScheduler) Install(interval time.Duration, binaryPath string) error {
	script := fmt.Sprintf(`"%s" sync > "%s" 2>&1 && "%s" inject --no-sync >> "%s" 2>&1`,
		binaryPath, s.logPath(), binaryPath, s.logPath())

	cmd := exec.Command("schtasks", "/create",
		"/tn", s.taskName(),
		"/tr", fmt.Sprintf(`cmd /c "%s"`, script),
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
