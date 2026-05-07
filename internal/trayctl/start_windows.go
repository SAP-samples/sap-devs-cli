//go:build windows

package trayctl

import (
	"os/exec"
	"syscall"
)

func startProcess(binaryPath string) error {
	cmd := exec.Command(binaryPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008 | 0x00000010, // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
	}
	return cmd.Start()
}
