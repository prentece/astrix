package indexer_test

import (
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "astrix/pkg/indexer/languages"
)

// failingReplacer simula falha na gravação atômica do índice.
type failingReplacer struct{}

func (failingReplacer) ReplaceProjectIndex(*storage.IndexSnapshot) error {
	return assert.AnError
}

func TestEngine_IndexProject_UsesAtomicReplacer(t *testing.T) {
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
	engine.SetIndexReplacer(storage.NewIndexStore(database))

	projDir := filepath.Join(tmpDir, "repo_atomic")
	require.NoError(t, os.MkdirAll(projDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "a.go"), []byte("package main\n\nfunc Alpha() {}\n"), 0o644))

	require.NoError(t, projectRepo.Create(&storage.Project{ID: "proj-atomic", Name: "Atomic", Path: projDir, Language: "go"}))

	require.NoError(t, engine.IndexProject("proj-atomic"))

	syms, _, err := symbolRepo.FindSymbol("proj-atomic", "Alpha", 10, 0)
	require.NoError(t, err)
	require.Len(t, syms, 1)

	states, err := fileStateRepo.ListByProject("proj-atomic")
	require.NoError(t, err)
	assert.Contains(t, states, "a.go")

	proj, err := projectRepo.GetByID("proj-atomic")
	require.NoError(t, err)
	assert.Equal(t, storage.StatusReady, proj.Status)
}

func TestEngine_IndexProject_ReplacerFailure_KeepsOldIndexAndMarksError(t *testing.T) {
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
	engine.SetIndexReplacer(storage.NewIndexStore(database))

	projDir := filepath.Join(tmpDir, "repo_atomic_fail")
	require.NoError(t, os.MkdirAll(projDir, 0o755))
	goFile := filepath.Join(projDir, "a.go")
	require.NoError(t, os.WriteFile(goFile, []byte("package main\n\nfunc Alpha() {}\n"), 0o644))
	require.NoError(t, projectRepo.Create(&storage.Project{ID: "proj-fail", Name: "Fail", Path: projDir, Language: "go"}))

	// Baseline bem-sucedido
	require.NoError(t, engine.IndexProject("proj-fail"))

	// Segunda indexação com gravação falhando
	require.NoError(t, os.WriteFile(goFile, []byte("package main\n\nfunc Beta() {}\n"), 0o644))
	engine.SetIndexReplacer(failingReplacer{})
	require.Error(t, engine.IndexProject("proj-fail"))

	// O índice anterior (Alpha) segue disponível
	syms, _, err := symbolRepo.FindSymbol("proj-fail", "Alpha", 10, 0)
	require.NoError(t, err)
	assert.Len(t, syms, 1)

	proj, err := projectRepo.GetByID("proj-fail")
	require.NoError(t, err)
	assert.Equal(t, storage.StatusError, proj.Status)
}
