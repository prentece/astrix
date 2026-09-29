package indexer_test

import (
	"astrix/pkg/storage"
	"astrix/pkg/indexer"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_GetIndexingProgress(t *testing.T) {
	tempDir := t.TempDir()
	tempDB := filepath.Join(tempDir, "test_progress.db")
	database, err := storage.NewDatabase(tempDB)
	require.NoError(t, err)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)

	// Inicialmente nenhum projeto deve ter progresso ativo
	assert.Nil(t, engine.GetIndexingProgress("non-existent-id"))

	// Cria um repositório temporário de teste com alguns arquivos Go
	repoDir := filepath.Join(tempDir, "sample-repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	file1 := filepath.Join(repoDir, "main.go")
	require.NoError(t, os.WriteFile(file1, []byte("package main\n\nfunc main() {}\n"), 0644))

	file2 := filepath.Join(repoDir, "util.go")
	require.NoError(t, os.WriteFile(file2, []byte("package main\n\nfunc Helper() string { return \"ok\" }\n"), 0644))

	proj := &storage.Project{
		ID:       "proj-progress-test",
		Name:     "Progress Test",
		Path:     repoDir,
		Language: "go",
		Status:   storage.StatusPending,
	}
	require.NoError(t, projectRepo.Create(proj))

	// Executa a indexação
	err = engine.IndexProject(proj.ID)
	require.NoError(t, err)

	// Após a conclusão, o progresso ativo deve ser limpo e o status deve ser Ready
	assert.Nil(t, engine.GetIndexingProgress(proj.ID))

	updatedProj, err := projectRepo.GetByID(proj.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedProj)
	assert.Equal(t, storage.StatusReady, updatedProj.Status)
	assert.Equal(t, 2, updatedProj.FileCount)
}
