//go:build windows

package cli

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func lockFileOS(f *os.File) (bool, error) {
	var ol windows.Overlapped
	err := windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&ol,
	)
	if err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return false, nil
		}
		return false, fmt.Errorf("falha ao tentar adquirir lock do watcher: %w", err)
	}
	return true, nil
}

func unlockFileOS(f *os.File) {
	var ol windows.Overlapped
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
