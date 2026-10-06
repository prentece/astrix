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

func TestFileWatcher_PathResolutionAndPrefixCollision(t *testing.T) {
	database, engine, tmpDir := setupTestWatcherEnv(t)
	defer os.RemoveAll(tmpDir)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	watcherService, err := watcher.NewFileWatcherService(projectRepo, fileStateRepo, engine)
	require.NoError(t, err)
	defer watcherService.Stop()

	watcherService.SetDebounceDuration(100 * time.Millisecond)

	// Cria estrutura com nomes que compartilham prefixos
	baseDir := filepath.Join(tmpDir, "work")
	projMainDir := filepath.Join(baseDir, "myproject")
	projSiblingDir := filepath.Join(baseDir, "myproject-other")
	projNestedDir := filepath.Join(projMainDir, "submodule")

	require.NoError(t, os.MkdirAll(projMainDir, 0o755))
	require.NoError(t, os.MkdirAll(projSiblingDir, 0o755))
	require.NoError(t, os.MkdirAll(projNestedDir, 0o755))

	p1 := &storage.Project{ID: "p1", Name: "Main", Path: projMainDir, Language: "go", Status: storage.StatusReady}
	p2 := &storage.Project{ID: "p2", Name: "Sibling", Path: projSiblingDir, Language: "go", Status: storage.StatusReady}
	p3 := &storage.Project{ID: "p3", Name: "Nested", Path: projNestedDir, Language: "go", Status: storage.StatusReady}

	require.NoError(t, projectRepo.Create(p1))
	require.NoError(t, projectRepo.Create(p2))
	require.NoError(t, projectRepo.Create(p3))

	require.NoError(t, watcherService.WatchProject(p1))
	require.NoError(t, watcherService.WatchProject(p2))
	require.NoError(t, watcherService.WatchProject(p3))

	watcherService.SetAutoSync("p1", false)
	watcherService.SetAutoSync("p2", false)
	watcherService.SetAutoSync("p3", false)

	// Cria arquivo no sibling 'myproject-other/service.go'
	siblingFile := filepath.Join(projSiblingDir, "service.go")
	require.NoError(t, os.WriteFile(siblingFile, []byte("package sibling\nfunc Sib() {}\n"), 0o644))

	// Aguarda e verifica que apenas p2 (Sibling) detectou alterações, não p1!
	require.Eventually(t, func() bool {
		st2, _ := watcherService.GetSyncStatus("p2")
		return st2 != nil && st2["has_changes"].(bool)
	}, 2*time.Second, 50*time.Millisecond)

	st1, err := watcherService.GetSyncStatus("p1")
	require.NoError(t, err)
	assert.False(t, st1["has_changes"].(bool), "p1 não deve ser afetado por alterações em p2")

	// Cria arquivo no nested 'myproject/submodule/inner.go'
	nestedFile := filepath.Join(projNestedDir, "inner.go")
	require.NoError(t, os.WriteFile(nestedFile, []byte("package nested\nfunc Inner() {}\n"), 0o644))

	// O nested (p3) deve detectar a alteração por ter a correspondência mais específica
	require.Eventually(t, func() bool {
		st3, _ := watcherService.GetSyncStatus("p3")
		return st3 != nil && st3["has_changes"].(bool)
	}, 2*time.Second, 50*time.Millisecond)
}

func TestFileWatcher_DynamicSubdirectoryCreation(t *testing.T) {
	database, engine, tmpDir := setupTestWatcherEnv(t)
	defer os.RemoveAll(tmpDir)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	watcherService, err := watcher.NewFileWatcherService(projectRepo, fileStateRepo, engine)
	require.NoError(t, err)
	defer watcherService.Stop()

	watcherService.SetDebounceDuration(100 * time.Millisecond)

	repoDir := filepath.Join(tmpDir, "dyn_repo")
	require.NoError(t, os.MkdirAll(repoDir, 0o755))

	proj := &storage.Project{ID: "dyn-proj", Name: "Dyn", Path: repoDir, Language: "go", Status: storage.StatusReady}
	require.NoError(t, projectRepo.Create(proj))
	require.NoError(t, watcherService.WatchProject(proj))
	require.NoError(t, watcherService.Start())

	// 1. Cria subpasta aninhada após o watch já estar ativo
	nestedSubDir := filepath.Join(repoDir, "cmd", "api")
	require.NoError(t, os.MkdirAll(nestedSubDir, 0o755))

	// Dá tempo para o watcher registrar a nova pasta
	time.Sleep(150 * time.Millisecond)

	// 2. Cria arquivo dentro da subpasta recém-criada
	apiFile := filepath.Join(nestedSubDir, "main.go")
	require.NoError(t, os.WriteFile(apiFile, []byte("package main\nfunc RunApi() {}\n"), 0o644))

	// Aguarda auto-sync indexar o símbolo criado dentro da subpasta dinâmica
	symbolRepo := storage.NewSymbolRepo(database)
	require.Eventually(t, func() bool {
		syms, _, err := symbolRepo.FindSymbol(proj.ID, "RunApi", 10, 0)
		return err == nil && len(syms) > 0
	}, 3*time.Second, 50*time.Millisecond)
}

func TestFileWatcher_UnwatchDeletedDirectory(t *testing.T) {
	database, engine, tmpDir := setupTestWatcherEnv(t)
	defer os.RemoveAll(tmpDir)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	watcherService, err := watcher.NewFileWatcherService(projectRepo, fileStateRepo, engine)
	require.NoError(t, err)
	defer watcherService.Stop()

	repoDir := filepath.Join(tmpDir, "del_repo")
	require.NoError(t, os.MkdirAll(repoDir, 0o755))

	proj := &storage.Project{ID: "del-proj", Name: "Del", Path: repoDir, Language: "go", Status: storage.StatusReady}
	require.NoError(t, projectRepo.Create(proj))
	require.NoError(t, watcherService.WatchProject(proj))

	// Remove o diretório do disco antes de chamar UnwatchProject
	require.NoError(t, os.RemoveAll(repoDir))

	// UnwatchProject não deve entrar em pânico nem retornar erro
	assert.NotPanics(t, func() {
		watcherService.UnwatchProject(proj.ID)
	})
}

func TestFileWatcher_DirtyRescheduledAfterSync(t *testing.T) {
	database, engine, tmpDir := setupTestWatcherEnv(t)
	defer os.RemoveAll(tmpDir)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	watcherService, err := watcher.NewFileWatcherService(projectRepo, fileStateRepo, engine)
	require.NoError(t, err)
	defer watcherService.Stop()

	watcherService.SetDebounceDuration(80 * time.Millisecond)

	repoDir := filepath.Join(tmpDir, "dirty_repo")
	require.NoError(t, os.MkdirAll(repoDir, 0o755))

	f1 := filepath.Join(repoDir, "a.go")
	require.NoError(t, os.WriteFile(f1, []byte("package main\nfunc A() {}\n"), 0o644))

	proj := &storage.Project{ID: "dirty-proj", Name: "Dirty", Path: repoDir, Language: "go", Status: storage.StatusReady}
	require.NoError(t, projectRepo.Create(proj))
	require.NoError(t, watcherService.WatchProject(proj))
	require.NoError(t, watcherService.Start())

	// Sincroniza primeiro arquivo
	_, err = watcherService.TriggerSync(proj.ID)
	require.NoError(t, err)

	// Cria segundo arquivo e terceiro em sequência rápida
	f2 := filepath.Join(repoDir, "b.go")
	require.NoError(t, os.WriteFile(f2, []byte("package main\nfunc B() {}\n"), 0o644))

	symbolRepo := storage.NewSymbolRepo(database)
	require.Eventually(t, func() bool {
		syms, _, err := symbolRepo.FindSymbol(proj.ID, "B", 10, 0)
		return err == nil && len(syms) > 0
	}, 3*time.Second, 50*time.Millisecond)
}


