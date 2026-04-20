//go:build linux

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type linuxScheduler struct {
	cacheDir string
}

func newPlatformScheduler(cacheDir string) Scheduler {
	return &linuxScheduler{cacheDir: cacheDir}
}

const unitName = "sap-devs-sync"

func (s *linuxScheduler) unitDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user")
}

func (s *linuxScheduler) logPath() string {
	return filepath.Join(s.cacheDir, "daemon.log")
}

func (s *linuxScheduler) generateServiceUnit(binaryPath string) string {
	return fmt.Sprintf(`[Unit]
Description=sap-devs background sync

[Service]
Type=oneshot
ExecStart=/bin/sh -c '%s sync && %s inject --no-sync'
StandardOutput=file:%s
StandardError=file:%s
`, binaryPath, binaryPath, s.logPath(), s.logPath())
}

func (s *linuxScheduler) generateTimerUnit(interval time.Duration) string {
	return fmt.Sprintf(`[Unit]
Description=sap-devs background sync timer

[Timer]
OnBootSec=5min
OnUnitActiveSec=%s
Persistent=true

[Install]
WantedBy=timers.target
`, interval)
}

func (s *linuxScheduler) Install(interval time.Duration, binaryPath string) error {
	dir := s.unitDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	svc := s.generateServiceUnit(binaryPath)
	if err := os.WriteFile(filepath.Join(dir, unitName+".service"), []byte(svc), 0644); err != nil {
		return err
	}
	tmr := s.generateTimerUnit(interval)
	if err := os.WriteFile(filepath.Join(dir, unitName+".timer"), []byte(tmr), 0644); err != nil {
		return err
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	cmd := exec.Command("systemctl", "--user", "enable", "--now", unitName+".timer")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl enable failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *linuxScheduler) Uninstall() error {
	_ = exec.Command("systemctl", "--user", "disable", "--now", unitName+".timer").Run()
	dir := s.unitDir()
	_ = os.Remove(filepath.Join(dir, unitName+".service"))
	_ = os.Remove(filepath.Join(dir, unitName+".timer"))
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

func (s *linuxScheduler) Status() (*Status, error) {
	dir := s.unitDir()
	timerPath := filepath.Join(dir, unitName+".timer")
	if _, err := os.Stat(timerPath); os.IsNotExist(err) {
		return &Status{Installed: false}, nil
	}
	st := &Status{Installed: true}
	cmd := exec.Command("systemctl", "--user", "show", unitName+".timer",
		"--property=LastTriggerUSec,NextElapseUSecRealtime")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return st, nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", parts[1]); err == nil {
			switch parts[0] {
			case "LastTriggerUSec":
				st.LastRun = t
			case "NextElapseUSecRealtime":
				st.NextRun = t
			}
		}
	}
	return st, nil
}
