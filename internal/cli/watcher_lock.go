package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// WatcherLock gerencia a exclusividade do FileWatcher entre múltiplos processos MCP.
type WatcherLock struct {
	path string
	file *os.File
}

// NewWatcherLock cria uma nova instância de WatcherLock para o caminho especificado.
func NewWatcherLock(path string) *WatcherLock {
	return &WatcherLock{path: path}
}

// Path retorna o caminho do arquivo de lock.
func (l *WatcherLock) Path() string {
	return l.path
}

// IsHeld indica se este processo detém o lock no momento.
func (l *WatcherLock) IsHeld() bool {
	return l.file != nil
}

// TryAcquire tenta obter o lock exclusivo de forma não-bloqueante no SO.
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

	acquired, err := lockFileOS(f)
	if err != nil {
		_ = f.Close()
		return false, err
	}
	if !acquired {
		_ = f.Close()
		return false, nil
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
		unlockFileOS(l.file)
		_ = l.file.Close()
		l.file = nil
	}
}
