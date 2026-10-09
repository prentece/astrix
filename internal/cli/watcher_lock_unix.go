//go:build !windows

package cli

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func lockFileOS(f *os.File) (bool, error) {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return false, nil
		}
		return false, fmt.Errorf("falha ao tentar adquirir lock do watcher: %w", err)
	}
	return true, nil
}

func unlockFileOS(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
