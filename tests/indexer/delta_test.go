package indexer_test

import (
	"astrix/pkg/storage"
	"astrix/pkg/indexer"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "astrix/pkg/indexer/languages"
)

func setupTestDB(t *testing.T) (*storage.DB, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "astrix_delta_test_*")
	if err != nil {
		t.Fatalf("falha ao criar tmpDir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := storage.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("falha ao inicializar SQLite: %v", err)
	}

	return database, tmpDir
}

func TestDeltaEngine_StatCacheDetection(t *testing.T) {
	database, tmpDir := setupTestDB(t)
	defer os.RemoveAll(tmpDir)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())
	deltaEngine := indexer.NewDeltaEngine(fileStateRepo)

	projDir := filepath.Join(tmpDir, "test_repo")
	_ = os.MkdirAll(projDir, 0o755)

	proj := &storage.Project{
		ID:       "proj-1",
		Name:     "Test Project",
		Path:     projDir,
		Language: "go",
		Status:   storage.StatusReady,
	}
	_ = projectRepo.Create(proj)

	file1 := filepath.Join(projDir, "service.go")
	_ = os.WriteFile(file1, []byte("package test\n\nfunc Run() string {\n\treturn \"ok\"\n}\n"), 0o644)

	// Simula baseline de estado em cache
	info, _ := os.Stat(file1)
	err := fileStateRepo.Upsert(&storage.ProjectFileState{
		ProjectID:   "proj-1",
		FilePath:    "service.go",
		MTime:       info.ModTime().Unix(),
		FileSize:    info.Size(),
		ContentHash: "hash1",
		DigestHash:  "dhash1",
	})
	if err != nil {
		t.Fatalf("falha ao salvar estado: %v", err)
	}

	// 1. Nenhuma alteração
	delta, err := deltaEngine.DetectDelta("proj-1", projDir)
	if err != nil {
		t.Fatalf("DetectDelta falhou: %v", err)
	}
	if len(delta.ModifiedFiles) != 0 || len(delta.AddedFiles) != 0 || len(delta.DeletedFiles) != 0 {
		t.Fatalf("Esperava zero alterações, obteve: %+v", delta)
	}
	if delta.UnchangedCount != 1 {
		t.Fatalf("Esperava 1 arquivo inalterado, obteve %d", delta.UnchangedCount)
	}

	// 2. Modifica arquivo
	time.Sleep(1100 * time.Millisecond) // Garante que mtime mude no sistema de arquivos
	_ = os.WriteFile(file1, []byte("package test\n\nfunc Run() string {\n\t// Comentário novo\n\treturn \"ok v2\"\n}\n"), 0o644)

	delta, err = deltaEngine.DetectDelta("proj-1", projDir)
	if err != nil {
		t.Fatalf("DetectDelta falhou: %v", err)
	}
	if len(delta.ModifiedFiles) != 1 || delta.ModifiedFiles[0] != "service.go" {
		t.Fatalf("Esperava service.go modificado, obteve: %+v", delta.ModifiedFiles)
	}

	// 3. Adiciona novo arquivo
	file2 := filepath.Join(projDir, "util.go")
	_ = os.WriteFile(file2, []byte("package test\n\nfunc Format() {}\n"), 0o644)

	delta, err = deltaEngine.DetectDelta("proj-1", projDir)
	if err != nil {
		t.Fatalf("DetectDelta falhou: %v", err)
	}
	if len(delta.AddedFiles) != 1 || delta.AddedFiles[0] != "util.go" {
		t.Fatalf("Esperava util.go em AddedFiles, obteve: %+v", delta.AddedFiles)
	}

	// 4. Remove arquivo
	_ = os.Remove(file1)
	delta, err = deltaEngine.DetectDelta("proj-1", projDir)
	if err != nil {
		t.Fatalf("DetectDelta falhou: %v", err)
	}
	if len(delta.DeletedFiles) != 1 || delta.DeletedFiles[0] != "service.go" {
		t.Fatalf("Esperava service.go em DeletedFiles, obteve: %+v", delta.DeletedFiles)
	}
}

func TestTwoTierHash_InternalChangeVsSignatureChange(t *testing.T) {
	database, tmpDir := setupTestDB(t)
	defer os.RemoveAll(tmpDir)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)
	engine.SetFileStateRepo(fileStateRepo)

	projDir := filepath.Join(tmpDir, "repo_two_tier")
	_ = os.MkdirAll(projDir, 0o755)

	filePath := filepath.Join(projDir, "user_service.go")
	initialCode := `package main

// UserService gerencia usuarios
type UserService struct{}

func (s *UserService) GetUser(id string) string {
	return "User: " + id
}
`
	_ = os.WriteFile(filePath, []byte(initialCode), 0o644)

	proj := &storage.Project{
		ID:       "proj-tier",
		Name:     "Tier Test",
		Path:     projDir,
		Language: "go",
		Status:   storage.StatusPending,
	}
	_ = projectRepo.Create(proj)

	// Baseline index
	err := engine.IndexProject(proj.ID)
	if err != nil {
		t.Fatalf("IndexProject falhou: %v", err)
	}

	// 1. Modifica o arquivo no disco
	time.Sleep(10 * time.Millisecond)
	modifiedCode := `package main

// UserService gerencia usuarios
type UserService struct{}

func (s *UserService) GetUser(id string) string {
	return "User modified: " + id
}

func (s *UserService) DeleteUser(id string) bool {
	return true
}
`
	_ = os.WriteFile(filePath, []byte(modifiedCode), 0o644)

	report, err := engine.ProcessIncrementalDelta(proj.ID)
	if err != nil {
		t.Fatalf("ProcessIncrementalDelta falhou: %v", err)
	}

	if report.FilesParsed != 1 {
		t.Errorf("Esperava 1 arquivo parseado pela AST, obteve %d", report.FilesParsed)
	}

	// Verifica se novo símbolo foi adicionado
	syms, _, _ := symbolRepo.FindSymbol(proj.ID, "DeleteUser", 10, 0)
	if len(syms) == 0 {
		t.Errorf("Esperava símbolo DeleteUser adicionado pela reindexação incremental")
	}
}

func TestCascadeDeletion_RemovesSymbolsAndSummaries(t *testing.T) {
	database, tmpDir := setupTestDB(t)
	defer os.RemoveAll(tmpDir)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)
	engine.SetFileStateRepo(fileStateRepo)

	projDir := filepath.Join(tmpDir, "repo_cascade")
	_ = os.MkdirAll(projDir, 0o755)

	filePath := filepath.Join(projDir, "controller.go")
	_ = os.WriteFile(filePath, []byte("package main\n\nfunc HandleRequest() {}\n"), 0o644)

	proj := &storage.Project{
		ID:       "proj-del",
		Name:     "Del Test",
		Path:     projDir,
		Language: "go",
		Status:   storage.StatusPending,
	}
	_ = projectRepo.Create(proj)
	_ = engine.IndexProject(proj.ID)

	// Deleta o arquivo no disco
	_ = os.Remove(filePath)

	report, err := engine.ProcessIncrementalDelta(proj.ID)
	if err != nil {
		t.Fatalf("ProcessIncrementalDelta falhou: %v", err)
	}

	if report.FilesDeleted != 1 {
		t.Errorf("Esperava 1 arquivo deletado, obteve %d", report.FilesDeleted)
	}

	// Verifica se símbolos foram removidos
	syms, _, _ := symbolRepo.FindSymbol(proj.ID, "HandleRequest", 10, 0)
	if len(syms) != 0 {
		t.Errorf("Símbolos do arquivo deletado ainda existem no banco: %d", len(syms))
	}
}
