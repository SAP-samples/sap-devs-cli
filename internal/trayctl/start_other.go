//go:build !windows

package trayctl

import "os/exec"

func startProcess(binaryPath string) error {
	cmd := exec.Command(binaryPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
