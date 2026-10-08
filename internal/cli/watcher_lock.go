package cli

// WatcherLock gerencia a exclusividade do FileWatcher entre múltiplos processos MCP.
type WatcherLock struct {
	path string
	file interface{}
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
