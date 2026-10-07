package db_test

import (
	"astrix/pkg/storage"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDependencyGraphRepo_ResolveTargetFiles_Disambiguation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "astrix_dep_disambig_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := storage.NewDatabase(dbPath)
	require.NoError(t, err)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)

	proj := &storage.Project{
		ID:        "proj-disambig",
		Name:      "Disambig Project",
		Path:      "/tmp/proj",
		Language:  "go",
		Status:    storage.StatusReady,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, projectRepo.Create(proj))

	// Insere uma aresta que depende do símbolo 'Config'
	// O source_file está em 'pkg/service/user_service.go'
	edge := &storage.DependencyEdge{
		ProjectID:        proj.ID,
		SourceSymbol:     "UserService",
		TargetSymbol:     "Config",
		SourceFile:       "pkg/service/user_service.go",
		TargetFile:       "", // pendente de resolução
		RelationshipType: "uses",
	}
	require.NoError(t, depRepo.SaveDependencies(proj.ID, []*storage.DependencyEdge{edge}))

	// Existem 3 símbolos 'Config' homônimos no projeto:
	// 1. 'pkg/service/config.go' (mesmo diretório do source_file -> deve ser o preferido!)
	// 2. 'pkg/other/config.go' (diretório diferente)
	// 3. 'tests/mocks/config_test.go' (arquivo de teste -> penalizado)
	symbols := []*storage.Symbol{
		{
			ProjectID: proj.ID,
			Name:      "Config",
			File:      "pkg/other/config.go",
		},
		{
			ProjectID: proj.ID,
			Name:      "Config",
			File:      "tests/mocks/config_test.go",
		},
		{
			ProjectID: proj.ID,
			Name:      "Config",
			File:      "pkg/service/config.go",
		},
	}

	// Executa resolução desambiguada
	err = depRepo.ResolveTargetFiles(proj.ID, symbols)
	require.NoError(t, err)

	// Verifica se a aresta foi preenchida apontando para o arquivo no mesmo diretório
	downstream, err := depRepo.GetDownstreamEdges(proj.ID, "UserService")
	require.NoError(t, err)
	require.Len(t, downstream, 1)
	assert.Equal(t, "pkg/service/config.go", downstream[0].TargetFile)
}
