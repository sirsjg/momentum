//go:build windows

package agent

import (
	"os"
	"os/exec"
	"strconv"
)

// setProcAttr is a no-op on Windows (no process groups)
func setProcAttr(cmd *exec.Cmd) {
	// Windows doesn't support Setpgid
}

// killProcessTree kills the process and its children using taskkill
func killProcessTree(pid int, process *os.Process, force bool) error {
	// /T kills process tree, /F forces termination (skip for graceful shutdown)
	args := []string{"/T", "/PID", strconv.Itoa(pid)}
	if force {
		args = append([]string{"/F"}, args...)
	}
	kill := exec.Command("taskkill", args...)
	if err := kill.Run(); err != nil {
		if force {
			return process.Kill()
		}
		return process.Signal(os.Interrupt)
	}
	return nil
}
