//go:build windows

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// TryAcquire tenta obter o lock exclusivo de forma não-bloqueante no Windows.
func (l *WatcherLock) TryAcquire() (bool, error) {
	if l.file != nil {
		return true, nil
	}

	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("falha ao criar diretório para watcher.lock: %w", err)
	}

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return false, fmt.Errorf("falha ao abrir watcher.lock: %w", err)
	}

	var overlapped windows.Overlapped
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	err = windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, &overlapped)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return false, nil
		}
		return false, fmt.Errorf("falha ao tentar adquirir lock do watcher no windows: %w", err)
	}

	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Sync()

	l.file = f
	return true, nil
}

// Release libera o lock e fecha o arquivo no Windows.
func (l *WatcherLock) Release() {
	if l.file != nil {
		if f, ok := l.file.(*os.File); ok {
			var overlapped windows.Overlapped
			_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &overlapped)
			_ = f.Close()
		}
		l.file = nil
	}
}
