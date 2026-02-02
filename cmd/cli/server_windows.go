//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr sets platform-specific process attributes for detaching
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
