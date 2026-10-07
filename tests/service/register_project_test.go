package service_test

import (
	"astrix/internal/service"
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectService_RegisterProject_RespectsExplicitName(t *testing.T) {
	database, err := storage.NewDatabase(":memory:")
	require.NoError(t, err)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)
	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)
	svc := service.NewProjectService(projectRepo, symbolRepo, engine)

	// Cria pasta temporária com package.json que tem "name": "Nio"
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "Nio"}`), 0644)

	// 1. Cenário: Usuário explicitamente forneceu o nome da pasta (ex: "fibra-frontend")
	proj, err := svc.RegisterProject("fibra-frontend", dir, "typescript")
	require.NoError(t, err)
	// Deve formatar apenas o nome fornecido ("Fibra Frontend"), sem descartar para "Nio"!
	assert.Equal(t, "Fibra Frontend", proj.Name)

	// 2. Cenário: Se o nome for omitido (""), aí sim deve usar o nome do manifesto ("Nio")
	dir2 := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir2, "package.json"), []byte(`{"name": "Nio"}`), 0644)
	proj2, err := svc.RegisterProject("", dir2, "typescript")
	require.NoError(t, err)
	assert.Equal(t, "Nio", proj2.Name)
}
