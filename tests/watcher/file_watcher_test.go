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
	indexStore := storage.NewIndexStore(database)
	engine.SetIndexReplacer(indexStore)
	engine.SetIncrementalApplier(indexStore)

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

func TestFileWatcher_FullLifecycle_CreateModifyDelete(t *testing.T) {
	database, engine, tmpDir := setupTestWatcherEnv(t)
	defer os.RemoveAll(tmpDir)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	watcherService, err := watcher.NewFileWatcherService(projectRepo, fileStateRepo, engine)
	require.NoError(t, err)
	defer watcherService.Stop()

	// Debounce rápido para testes
	watcherService.SetDebounceDuration(80 * time.Millisecond)

	repoDir := filepath.Join(tmpDir, "lifecycle_repo")
	require.NoError(t, os.MkdirAll(repoDir, 0o755))

	// Arquivo inicial
	mainFile := filepath.Join(repoDir, "main.go")
	require.NoError(t, os.WriteFile(mainFile, []byte("package main\n\nfunc Main() {}\n"), 0o644))

	proj := &storage.Project{
		ID:       "proj-lifecycle-test",
		Name:     "Lifecycle Test",
		Path:     repoDir,
		Language: "go",
		Status:   storage.StatusReady,
	}
	require.NoError(t, projectRepo.Create(proj))
	require.NoError(t, watcherService.WatchProject(proj))

	// Indexa o estado inicial (baseline)
	_, err = engine.ProcessIncrementalDelta(proj.ID)
	require.NoError(t, err)

	p, err := projectRepo.GetByID(proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, p.FileCount)
	assert.Equal(t, 1, p.SymbolCount)

	// Inicia o watcher
	require.NoError(t, watcherService.Start())

	// ==========================================
	// 1. CRIAÇÃO: Criar novo arquivo calc.go
	// ==========================================
	calcFile := filepath.Join(repoDir, "calc.go")
	require.NoError(t, os.WriteFile(calcFile, []byte("package main\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"), 0o644))

	// Aguarda o watcher auto-sincronizar a criação
	require.Eventually(t, func() bool {
		syms, _, err := symbolRepo.FindSymbol(proj.ID, "Add", 10, 0)
		return err == nil && len(syms) == 1
	}, 3*time.Second, 50*time.Millisecond, "Símbolo Add deve ser indexado após criação do arquivo")

	// Verifica se file state foi criado
	states, err := fileStateRepo.ListByProject(proj.ID)
	require.NoError(t, err)
	assert.Contains(t, states, "calc.go")

	// Verifica se metadados do projeto foram atualizados
	p, err = projectRepo.GetByID(proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, p.FileCount, "FileCount deve ser 2 após criação")
	assert.Equal(t, 2, p.SymbolCount, "SymbolCount deve ser 2 após criação")

	// ==========================================
	// 2. ALTERAÇÃO: Modificar calc.go adicionando Sub
	// ==========================================
	modifiedContent := "package main\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Sub(a, b int) int {\n\treturn a - b\n}\n"
	require.NoError(t, os.WriteFile(calcFile, []byte(modifiedContent), 0o644))

	// Aguarda o watcher auto-sincronizar a modificação
	require.Eventually(t, func() bool {
		symsSub, _, errSub := symbolRepo.FindSymbol(proj.ID, "Sub", 10, 0)
		symsAdd, _, errAdd := symbolRepo.FindSymbol(proj.ID, "Add", 10, 0)
		return errSub == nil && len(symsSub) == 1 && errAdd == nil && len(symsAdd) == 1
	}, 3*time.Second, 50*time.Millisecond, "Símbolos Add e Sub devem existir após alteração do arquivo")

	p, err = projectRepo.GetByID(proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, p.FileCount, "FileCount deve permanecer 2")
	assert.Equal(t, 3, p.SymbolCount, "SymbolCount deve ser 3 (Main, Add, Sub)")

	// ==========================================
	// 3. DELEÇÃO: Remover calc.go do disco
	// ==========================================
	require.NoError(t, os.Remove(calcFile))

	// Aguarda o watcher auto-sincronizar a deleção
	require.Eventually(t, func() bool {
		symsAdd, _, _ := symbolRepo.FindSymbol(proj.ID, "Add", 10, 0)
		symsSub, _, _ := symbolRepo.FindSymbol(proj.ID, "Sub", 10, 0)
		return len(symsAdd) == 0 && len(symsSub) == 0
	}, 3*time.Second, 50*time.Millisecond, "Símbolos de calc.go devem ser removidos após deleção do arquivo")

	// Verifica se file state foi removido
	states, err = fileStateRepo.ListByProject(proj.ID)
	require.NoError(t, err)
	assert.NotContains(t, states, "calc.go", "calc.go não deve mais constar em project_file_states")

	// Verifica se metadados do projeto retornaram ao estado baseline
	p, err = projectRepo.GetByID(proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, p.FileCount, "FileCount deve retornar para 1 após deleção")
	assert.Equal(t, 1, p.SymbolCount, "SymbolCount deve retornar para 1 após deleção")
}

func TestFileWatcher_JavaScriptLifecycle(t *testing.T) {
	database, engine, tmpDir := setupTestWatcherEnv(t)
	defer os.RemoveAll(tmpDir)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	watcherService, err := watcher.NewFileWatcherService(projectRepo, fileStateRepo, engine)
	require.NoError(t, err)
	defer watcherService.Stop()

	watcherService.SetDebounceDuration(80 * time.Millisecond)

	repoDir := filepath.Join(tmpDir, "js_repo")
	require.NoError(t, os.MkdirAll(repoDir, 0o755))

	// Arquivo inicial
	indexFile := filepath.Join(repoDir, "index.js")
	require.NoError(t, os.WriteFile(indexFile, []byte("function init() {}\n"), 0o644))

	proj := &storage.Project{
		ID:       "proj-js-test",
		Name:     "JS Test",
		Path:     repoDir,
		Language: "javascript",
		Status:   storage.StatusReady,
	}
	require.NoError(t, projectRepo.Create(proj))
	require.NoError(t, watcherService.WatchProject(proj))

	// Baseline
	_, err = engine.ProcessIncrementalDelta(proj.ID)
	require.NoError(t, err)

	require.NoError(t, watcherService.Start())

	// 1. CRIAÇÃO: criar teste.js
	testeFile := filepath.Join(repoDir, "teste.js")
	require.NoError(t, os.WriteFile(testeFile, []byte("function Hello() {\n    console.log(\"Hello World\");\n}\n"), 0o644))

	require.Eventually(t, func() bool {
		syms, _, err := symbolRepo.FindSymbol(proj.ID, "Hello", 10, 0)
		return err == nil && len(syms) == 1
	}, 3*time.Second, 50*time.Millisecond, "Função Hello de teste.js deve ser indexada")

	p, err := projectRepo.GetByID(proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, p.FileCount)
	assert.Equal(t, 2, p.SymbolCount)

	// 2. ALTERAÇÃO: adicionar Goodbye em teste.js
	require.NoError(t, os.WriteFile(testeFile, []byte("function Hello() {}\nfunction Goodbye() {}\n"), 0o644))

	require.Eventually(t, func() bool {
		syms, _, err := symbolRepo.FindSymbol(proj.ID, "Goodbye", 10, 0)
		return err == nil && len(syms) == 1
	}, 3*time.Second, 50*time.Millisecond, "Função Goodbye deve ser indexada após alteração")

	p, err = projectRepo.GetByID(proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, p.SymbolCount)

	// 3. DELEÇÃO: apagar teste.js
	require.NoError(t, os.Remove(testeFile))

	require.Eventually(t, func() bool {
		symsHello, _, _ := symbolRepo.FindSymbol(proj.ID, "Hello", 10, 0)
		symsGoodbye, _, _ := symbolRepo.FindSymbol(proj.ID, "Goodbye", 10, 0)
		return len(symsHello) == 0 && len(symsGoodbye) == 0
	}, 3*time.Second, 50*time.Millisecond, "Símbolos de teste.js devem ser removidos após deleção")

	p, err = projectRepo.GetByID(proj.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, p.FileCount)
	assert.Equal(t, 1, p.SymbolCount)
}

func TestFileWatcher_Restartability(t *testing.T) {
	database, engine, tmpDir := setupTestWatcherEnv(t)
	defer os.RemoveAll(tmpDir)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	watcherService, err := watcher.NewFileWatcherService(projectRepo, fileStateRepo, engine)
	require.NoError(t, err)

	repoDir := filepath.Join(tmpDir, "restart_repo")
	require.NoError(t, os.MkdirAll(repoDir, 0o755))
	file1 := filepath.Join(repoDir, "app.go")
	require.NoError(t, os.WriteFile(file1, []byte("package main\nfunc App() {}\n"), 0o644))

	proj := &storage.Project{
		ID:       "proj-restart",
		Name:     "Restart Project",
		Path:     repoDir,
		Language: "go",
		Status:   storage.StatusReady,
		AutoSync: true,
	}
	require.NoError(t, projectRepo.Create(proj))

	// Ciclo 1: Start -> Stop
	err = watcherService.Start()
	require.NoError(t, err)
	watcherService.Stop()

	// Ciclo 2: Start -> Stop (deve reiniciar fsnotify e contexto sem erros)
	err = watcherService.Start()
	require.NoError(t, err)
	watcherService.Stop()

	// Ciclo 3: Start -> Stop
	err = watcherService.Start()
	require.NoError(t, err)
	watcherService.Stop()
}
