//go:build !windows

package agent

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcAttr configures the command to create a new process group
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessTree sends a signal to the process group
func killProcessTree(pid int, process *os.Process, force bool) error {
	sig := syscall.SIGINT
	if force {
		sig = syscall.SIGKILL
	}
	if err := syscall.Kill(-pid, sig); err != nil {
		if force {
			return process.Kill()
		}
		return process.Signal(os.Interrupt)
	}
	return nil
}
