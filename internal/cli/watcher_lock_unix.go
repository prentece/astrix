//go:build !windows

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// TryAcquire tenta obter o lock exclusivo de forma não-bloqueante no Unix (Linux, macOS).
// Retorna true se este processo se tornou o líder do watcher, false se outro processo já o detém.
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

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return false, nil
		}
		return false, fmt.Errorf("falha ao tentar adquirir lock do watcher: %w", err)
	}

	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Sync()

	l.file = f
	return true, nil
}

// Release libera o lock e fecha o arquivo.
func (l *WatcherLock) Release() {
	if l.file != nil {
		if f, ok := l.file.(*os.File); ok {
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			_ = f.Close()
		}
		l.file = nil
	}
}
