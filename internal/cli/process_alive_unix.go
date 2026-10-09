//go:build !windows

package cli

import (
	"os"
	"syscall"
)

func isProcessAliveOS(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
