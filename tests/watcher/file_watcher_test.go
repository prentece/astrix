package watcher_test

import (
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"astrix/pkg/watcher"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "astrix/pkg/indexer/languages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestWatcherEnv(t *testing.T) (*storage.DB, *indexer.Engine, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "astrix_watcher_test_*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := storage.NewDatabase(dbPath)
	require.NoError(t, err)

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)
	engine.SetFileStateRepo(fileStateRepo)

	return database, engine, tmpDir
}

func TestFileWatcher_LifecycleAndAutoSync(t *testing.T) {
	database, engine, tmpDir := setupTestWatcherEnv(t)
	defer os.RemoveAll(tmpDir)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	watcherService, err := watcher.NewFileWatcherService(projectRepo, fileStateRepo, engine)
	require.NoError(t, err)
	defer watcherService.Stop()

	// Reduz debounce para testes rápidos
	watcherService.SetDebounceDuration(100 * time.Millisecond)

	// Cria diretório de projeto com arquivo inicial
	repoDir := filepath.Join(tmpDir, "sample_repo")
	require.NoError(t, os.MkdirAll(repoDir, 0o755))

	file1 := filepath.Join(repoDir, "main.go")
	require.NoError(t, os.WriteFile(file1, []byte("package main\n\nfunc Main() {}\n"), 0o644))

	proj := &storage.Project{
		ID:       "proj-watcher-1",
		Name:     "Watcher Test",
		Path:     repoDir,
		Language: "go",
		Status:   storage.StatusReady,
	}
	require.NoError(t, projectRepo.Create(proj))

	// Indexa o estado inicial
	_, err = engine.ProcessIncrementalDelta(proj.ID)
	require.NoError(t, err)

	// Inicia o watcher
	require.NoError(t, watcherService.Start())

	// Verifica status inicial
	status, err := watcherService.GetSyncStatus(proj.ID)
	require.NoError(t, err)
	assert.Equal(t, proj.ID, status["project_id"])
	assert.True(t, status["auto_sync"].(bool))
	assert.False(t, status["has_changes"].(bool))

	// 1. Cria um novo arquivo no disco para disparar watcher
	file2 := filepath.Join(repoDir, "helper.go")
	require.NoError(t, os.WriteFile(file2, []byte("package main\n\nfunc Helper() string { return \"ok\" }\n"), 0o644))

	// Aguarda debounce e indexação delta automática
	require.Eventually(t, func() bool {
		syms, _, err := storage.NewSymbolRepo(database).FindSymbol(proj.ID, "Helper", 10, 0)
		return err == nil && len(syms) > 0
	}, 3*time.Second, 50*time.Millisecond)

	// 2. Testa modo Manual (AutoSync = false)
	watcherService.SetAutoSync(proj.ID, false)
	assert.False(t, watcherService.GetAutoSync(proj.ID))

	file3 := filepath.Join(repoDir, "manual.go")
	require.NoError(t, os.WriteFile(file3, []byte("package main\n\nfunc Manual() {}\n"), 0o644))

	// Aguarda detecção de pendência pelo watcher
	require.Eventually(t, func() bool {
		st, err := watcherService.GetSyncStatus(proj.ID)
		if err != nil {
			return false
		}
		return st["has_changes"].(bool)
	}, 3*time.Second, 50*time.Millisecond)

	// 3. Força sincronização manual
	report, err := watcherService.TriggerSync(proj.ID)
	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.GreaterOrEqual(t, report.FilesParsed, 1)
}

func TestFileWatcher_IgnorePatterns(t *testing.T) {
	database, engine, tmpDir := setupTestWatcherEnv(t)
	defer os.RemoveAll(tmpDir)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	watcherService, err := watcher.NewFileWatcherService(projectRepo, fileStateRepo, engine)
	require.NoError(t, err)
	defer watcherService.Stop()

	repoDir := filepath.Join(tmpDir, "ignore_repo")
	require.NoError(t, os.MkdirAll(repoDir, 0o755))

	proj := &storage.Project{
		ID:       "proj-ignore-1",
		Name:     "Ignore Test",
		Path:     repoDir,
		Language: "go",
		Status:   storage.StatusReady,
	}
	require.NoError(t, projectRepo.Create(proj))
	require.NoError(t, watcherService.WatchProject(proj))

	// Desativa autoSync para checar deltas
	watcherService.SetAutoSync(proj.ID, false)

	// Cria pasta ignorada node_modules e .git
	nodeDir := filepath.Join(repoDir, "node_modules", "package")
	require.NoError(t, os.MkdirAll(nodeDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nodeDir, "index.js"), []byte("console.log()"), 0o644))

	// Cria arquivo temporário
	tmpFile := filepath.Join(repoDir, "temp.tmp")
	require.NoError(t, os.WriteFile(tmpFile, []byte("tmp"), 0o644))

	// Verifica se o status de sincronização não possui alterações registradas
	status, err := watcherService.GetSyncStatus(proj.ID)
	require.NoError(t, err)
	assert.False(t, status["has_changes"].(bool))
}
