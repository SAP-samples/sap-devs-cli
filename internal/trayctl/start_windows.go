//go:build windows

package trayctl

import (
	"os/exec"
	"syscall"
)

func startProcess(binaryPath string) error {
	cmd := exec.Command(binaryPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000010, // CREATE_NEW_PROCESS_GROUP (DETACHED_PROCESS is invalid for GUI-subsystem binaries)
	}
	return cmd.Start()
}
