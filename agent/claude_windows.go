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
	// taskkill /T kills the process tree, /F forces termination
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	if err := kill.Run(); err != nil {
		// Fallback to direct process kill
		return process.Kill()
	}
	return nil
}
