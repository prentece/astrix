package watcher

import (
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	ignore "github.com/sabhiram/go-gitignore"
)

// FileWatcherService monitora alterações no sistema de arquivos em tempo real e gerencia auto-sincronização.
type FileWatcherService struct {
	projectRepo    storage.ProjectRepository
	fileStateRepo  storage.FileStateRepository
	engine         *indexer.Engine
	deltaEngine    *indexer.DeltaEngine

	watcher          *fsnotify.Watcher
	debounceDuration time.Duration

	mu              sync.RWMutex
	watchedProjects map[string]*storage.Project // projectID -> Project
	pathProjects    map[string]string          // canonical path -> projectID
	autoSync        map[string]bool            // projectID -> bool (default true)
	pendingDeltas   map[string]*storage.DeltaResult
	syncingMap      map[string]bool
	debounceTimers  map[string]*time.Timer
	gitIgnores      map[string]*ignore.GitIgnore

	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewFileWatcherService cria uma nova instância de FileWatcherService.
func NewFileWatcherService(
	projectRepo storage.ProjectRepository,
	fileStateRepo storage.FileStateRepository,
	engine *indexer.Engine,
) (*FileWatcherService, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("falha ao inicializar fsnotify: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &FileWatcherService{
		projectRepo:      projectRepo,
		fileStateRepo:    fileStateRepo,
		engine:           engine,
		deltaEngine:      indexer.NewDeltaEngine(fileStateRepo),
		watcher:          fsWatcher,
		debounceDuration: 1500 * time.Millisecond,
		watchedProjects:  make(map[string]*storage.Project),
		pathProjects:     make(map[string]string),
		autoSync:         make(map[string]bool),
		pendingDeltas:    make(map[string]*storage.DeltaResult),
		syncingMap:       make(map[string]bool),
		debounceTimers:   make(map[string]*time.Timer),
		gitIgnores:       make(map[string]*ignore.GitIgnore),
		ctx:              ctx,
		cancelFunc:       cancel,
	}, nil
}

// Start inicializa os watchers para todos os projetos cadastrados e o loop de eventos.
func (w *FileWatcherService) Start() error {
	projects, err := w.projectRepo.ListAll()
	if err == nil {
		for _, proj := range projects {
			if err := w.WatchProject(proj); err != nil {
				log.Printf("[WATCHER WARN] Falha ao monitorar projeto %s (%s): %v\n", proj.Name, proj.Path, err)
			}
		}
	}

	// Goroutine para consumir eventos do fsnotify
	go w.eventLoop()

	// Goroutine de polling/ticker de fallback (a cada 4s) para garantir 100% de detecção no boundary WSL2/Docker
	go w.fallbackDeltaTicker()

	log.Printf("[WATCHER INIT] FileWatcherService ativo com debounce de %v e fallback delta engine.\n", w.debounceDuration)
	return nil
}

// Stop desliga o watcher e cancela o contexto.
func (w *FileWatcherService) Stop() {
	w.cancelFunc()
	if w.watcher != nil {
		_ = w.watcher.Close()
	}
}

// WatchProject adiciona um projeto e suas subpastas não ignoradas ao monitoramento de arquivos.
func (w *FileWatcherService) WatchProject(proj *storage.Project) error {
	if proj == nil || proj.Path == "" {
		return nil
	}

	cleanPath := filepath.Clean(proj.Path)
	info, err := os.Stat(cleanPath)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("caminho inválido ou inacessível: %s", cleanPath)
	}

	w.mu.Lock()
	w.watchedProjects[proj.ID] = proj
	w.pathProjects[cleanPath] = proj.ID
	w.autoSync[proj.ID] = proj.AutoSync

	// Carrega .gitignore se existir
	gitignorePath := filepath.Join(cleanPath, ".gitignore")
	if data, err := os.ReadFile(gitignorePath); err == nil {
		w.gitIgnores[proj.ID] = ignore.CompileIgnoreLines(strings.Split(string(data), "\n")...)
	}
	w.mu.Unlock()

	// Caminha recursivamente adicionando diretórios ao fsnotify
	var addedDirs int
	_ = filepath.Walk(cleanPath, func(path string, fileInfo os.FileInfo, walkErr error) error {
		if walkErr != nil || !fileInfo.IsDir() {
			return nil
		}

		dirName := fileInfo.Name()
		if indexer.DefaultIgnoredDirs[dirName] || strings.HasPrefix(dirName, ".") && dirName != "." {
			return filepath.SkipDir
		}

		rel, _ := filepath.Rel(cleanPath, path)
		if rel != "." && w.isIgnoredByGit(proj.ID, rel) {
			return filepath.SkipDir
		}

		if err := w.watcher.Add(path); err == nil {
			addedDirs++
		}
		return nil
	})

	log.Printf("[WATCHER] Projeto '%s' monitorado em '%s' (%d diretórios registrados)\n", proj.Name, cleanPath, addedDirs)
	return nil
}

// UnwatchProject remove um projeto do monitoramento.
func (w *FileWatcherService) UnwatchProject(projectID string) {
	w.mu.Lock()
	proj, exists := w.watchedProjects[projectID]
	if !exists {
		w.mu.Unlock()
		return
	}

	delete(w.watchedProjects, projectID)
	delete(w.pathProjects, filepath.Clean(proj.Path))
	delete(w.autoSync, projectID)
	delete(w.pendingDeltas, projectID)
	delete(w.syncingMap, projectID)
	delete(w.gitIgnores, projectID)

	if timer, ok := w.debounceTimers[projectID]; ok {
		timer.Stop()
		delete(w.debounceTimers, projectID)
	}
	w.mu.Unlock()

	// Remove diretórios do fsnotify
	_ = filepath.Walk(proj.Path, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			_ = w.watcher.Remove(path)
		}
		return nil
	})
}

// SetAutoSync altera a preferência de sincronização automática para um projeto e persiste no banco.
func (w *FileWatcherService) SetAutoSync(projectID string, enabled bool) {
	w.mu.Lock()
	w.autoSync[projectID] = enabled
	if proj, ok := w.watchedProjects[projectID]; ok {
		proj.AutoSync = enabled
	}
	w.mu.Unlock()

	if w.projectRepo != nil {
		if err := w.projectRepo.SetAutoSync(projectID, enabled); err != nil {
			log.Printf("[WATCHER WARN] Falha ao persistir auto_sync para projeto %s: %v\n", projectID, err)
		}
	}
}

// GetAutoSync retorna se o auto-sync está habilitado para o projeto.
func (w *FileWatcherService) GetAutoSync(projectID string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	enabled, exists := w.autoSync[projectID]
	if !exists {
		return true // padrão
	}
	return enabled
}

// SetDebounceDuration define a janela de agrupamento de eventos.
func (w *FileWatcherService) SetDebounceDuration(duration time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if duration >= 100*time.Millisecond {
		w.debounceDuration = duration
	}
}

// GetSyncStatus retorna o estado atual de sincronização do projeto.
func (w *FileWatcherService) GetSyncStatus(projectID string) (map[string]any, error) {
	w.mu.RLock()
	proj, exists := w.watchedProjects[projectID]
	autoSyncVal, hasAutoSync := w.autoSync[projectID]
	if !hasAutoSync {
		autoSyncVal = true
	}
	isSyncing := w.syncingMap[projectID]
	pendingDelta := w.pendingDeltas[projectID]
	w.mu.RUnlock()

	if !exists {
		p, err := w.projectRepo.GetByID(projectID)
		if err != nil || p == nil {
			return nil, fmt.Errorf("projeto não encontrado: %s", projectID)
		}
		proj = p
	}

	// Detecta delta em tempo real para garantir precisão dinâmica de arquivos alterados
	d, err := w.deltaEngine.DetectDelta(projectID, proj.Path)
	var currentDelta *storage.DeltaResult
	if err == nil {
		if len(d.AddedFiles) > 0 || len(d.ModifiedFiles) > 0 || len(d.DeletedFiles) > 0 {
			currentDelta = d
			w.mu.Lock()
			w.pendingDeltas[projectID] = d
			w.mu.Unlock()
		} else {
			w.mu.Lock()
			delete(w.pendingDeltas, projectID)
			w.mu.Unlock()
		}
	} else if pendingDelta != nil {
		currentDelta = pendingDelta
	}

	hasChanges := currentDelta != nil && (len(currentDelta.AddedFiles) > 0 || len(currentDelta.ModifiedFiles) > 0 || len(currentDelta.DeletedFiles) > 0)

	result := map[string]any{
		"project_id":   projectID,
		"is_syncing":   isSyncing,
		"auto_sync":    autoSyncVal,
		"has_changes":  hasChanges,
		"delta":        currentDelta,
		"last_checked": time.Now(),
	}

	return result, nil
}

// TriggerSync força a sincronização incremental (AST + LLM Summaries) imediatamente.
func (w *FileWatcherService) TriggerSync(projectID string) (*storage.DeltaReport, error) {
	w.mu.Lock()
	if w.syncingMap[projectID] {
		w.mu.Unlock()
		return nil, fmt.Errorf("sincronização já em andamento para o projeto %s", projectID)
	}
	w.syncingMap[projectID] = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.syncingMap[projectID] = false
		delete(w.pendingDeltas, projectID)
		w.mu.Unlock()
	}()

	// Executa indexação incremental AST
	report, err := w.engine.ProcessIncrementalDelta(projectID)
	if err != nil {
		return nil, fmt.Errorf("erro na sincronização incremental: %w", err)
	}

	return report, nil
}

// eventLoop processa eventos emitidos pelo fsnotify.
func (w *FileWatcherService) eventLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleFsnotifyEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[WATCHER ERROR] Erro do fsnotify: %v\n", err)
		}
	}
}

// handleFsnotifyEvent analisa o evento e agenda o debounce do projeto associado.
func (w *FileWatcherService) handleFsnotifyEvent(event fsnotify.Event) {
	eventPath := filepath.Clean(event.Name)

	// Filtra arquivos temporários e pastas de build/cache
	if w.isIgnoredPath(eventPath) {
		return
	}

	// Se for criação de novo diretório, adiciona ao fsnotify
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(eventPath); err == nil && info.IsDir() {
			dirName := info.Name()
			if !indexer.DefaultIgnoredDirs[dirName] && !strings.HasPrefix(dirName, ".") {
				_ = w.watcher.Add(eventPath)
			}
		}
	}

	// Identifica o projeto ao qual o caminho pertence
	projectID := w.findProjectForPath(eventPath)
	if projectID == "" {
		return
	}

	// Agenda debounce para o projeto
	w.scheduleProjectDebounce(projectID)
}

// scheduleProjectDebounce reinicia o temporizador de debounce para agrupar alterações.
func (w *FileWatcherService) scheduleProjectDebounce(projectID string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if timer, exists := w.debounceTimers[projectID]; exists {
		timer.Stop()
	}

	w.debounceTimers[projectID] = time.AfterFunc(w.debounceDuration, func() {
		w.processDebouncedChanges(projectID)
	})
}

// processDebouncedChanges é executado quando o debounce expira.
func (w *FileWatcherService) processDebouncedChanges(projectID string) {
	w.mu.RLock()
	proj, exists := w.watchedProjects[projectID]
	autoSyncVal, hasAutoSync := w.autoSync[projectID]
	if !hasAutoSync {
		autoSyncVal = true
	}
	isSyncing := w.syncingMap[projectID]
	w.mu.RUnlock()

	if !exists || isSyncing {
		return
	}

	// Detecta alterações reais através do DeltaEngine
	delta, err := w.deltaEngine.DetectDelta(projectID, proj.Path)
	if err != nil {
		log.Printf("[WATCHER WARN] Falha ao detectar delta para projeto %s: %v\n", proj.Name, err)
		return
	}

	hasChanges := len(delta.AddedFiles) > 0 || len(delta.ModifiedFiles) > 0 || len(delta.DeletedFiles) > 0
	if !hasChanges {
		w.mu.Lock()
		delete(w.pendingDeltas, projectID)
		w.mu.Unlock()
		return
	}

	if autoSyncVal {
		log.Printf("[WATCHER AUTO-SYNC] Disparando auto-sincronização para '%s' (%d modificados, %d novos, %d removidos)\n",
			proj.Name, len(delta.ModifiedFiles), len(delta.AddedFiles), len(delta.DeletedFiles))
		_, _ = w.TriggerSync(projectID)
	} else {
		w.mu.Lock()
		w.pendingDeltas[projectID] = delta
		w.mu.Unlock()
	}
}

// fallbackDeltaTicker realiza varreduras periódicas leves em segundo plano para robustez total (WSL2/Docker).
func (w *FileWatcherService) fallbackDeltaTicker() {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.runFallbackScan()
		}
	}
}

func (w *FileWatcherService) runFallbackScan() {
	w.mu.RLock()
	projects := make([]*storage.Project, 0, len(w.watchedProjects))
	for _, p := range w.watchedProjects {
		if !w.syncingMap[p.ID] {
			projects = append(projects, p)
		}
	}
	w.mu.RUnlock()

	for _, proj := range projects {
		delta, err := w.deltaEngine.DetectDelta(proj.ID, proj.Path)
		if err != nil {
			continue
		}

		hasChanges := len(delta.AddedFiles) > 0 || len(delta.ModifiedFiles) > 0 || len(delta.DeletedFiles) > 0
		if hasChanges {
			w.scheduleProjectDebounce(proj.ID)
		}
	}
}

// findProjectForPath descobre a qual projeto pertence um arquivo.
func (w *FileWatcherService) findProjectForPath(filePath string) string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for projPath, projID := range w.pathProjects {
		if strings.HasPrefix(filePath, projPath) {
			return projID
		}
	}
	return ""
}

// isIgnoredPath verifica se o arquivo é temporário ou pertence a pastas padrão ignoradas.
func (w *FileWatcherService) isIgnoredPath(p string) bool {
	base := filepath.Base(p)

	// Arquivos temporários de IDEs / SO
	if strings.HasPrefix(base, ".") && !strings.HasPrefix(base, ".env") {
		return true
	}
	if strings.HasSuffix(base, "~") || strings.HasSuffix(base, ".swp") || strings.HasSuffix(base, ".tmp") {
		return true
	}
	if strings.HasSuffix(base, ".db") || strings.HasSuffix(base, ".sqlite") || strings.HasSuffix(base, ".db-journal") {
		return true
	}

	// Diretórios ignorados
	parts := strings.Split(filepath.ToSlash(p), "/")
	for _, part := range parts {
		if indexer.DefaultIgnoredDirs[part] {
			return true
		}
	}

	return false
}

// isIgnoredByGit verifica se o caminho relativo está contemplado no .gitignore do projeto.
func (w *FileWatcherService) isIgnoredByGit(projectID, relPath string) bool {
	w.mu.RLock()
	gi := w.gitIgnores[projectID]
	w.mu.RUnlock()

	if gi != nil {
		return gi.MatchesPath(relPath)
	}
	return false
}
