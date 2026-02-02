//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr sets platform-specific process attributes for detaching
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
		Pgid:    0,    // Use the new process's PID as PGID
	}
}
