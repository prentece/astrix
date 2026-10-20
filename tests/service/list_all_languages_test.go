package service_test

import (
	"astrix/internal/service"
	"astrix/pkg/indexer"
	_ "astrix/pkg/indexer/languages"
	"astrix/pkg/storage"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingFileStateRepo simula falha na leitura dos estados de arquivo.
type failingFileStateRepo struct {
	storage.FileStateRepository
}

func (failingFileStateRepo) ListByProject(string) (map[string]*storage.ProjectFileState, error) {
	return nil, errors.New("boom")
}

type listAllFixture struct {
	database    *storage.DB
	projectRepo *storage.ProjectRepo
	symbolRepo  *storage.SymbolRepo
	engine      *indexer.Engine
}

func newListAllFixture(t *testing.T) *listAllFixture {
	t.Helper()
	database, err := storage.NewDatabase(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	engine := indexer.NewEngine(projectRepo, symbolRepo, storage.NewDependencyGraphRepo(database), storage.NewDataModelRepo(database))

	require.NoError(t, projectRepo.Create(&storage.Project{
		ID: "p1", Name: "P1", Path: "/tmp/p1", Language: "go", Status: storage.StatusReady,
	}))
	return &listAllFixture{database: database, projectRepo: projectRepo, symbolRepo: symbolRepo, engine: engine}
}

func TestProjectService_ListAll_NilEngineHasNoLanguages(t *testing.T) {
	f := newListAllFixture(t)
	svc := service.NewProjectService(f.projectRepo, f.symbolRepo, nil)

	projects, err := svc.ListAll()

	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Empty(t, projects[0].Languages)
	assert.Equal(t, "go", projects[0].Language)
}

func TestProjectService_ListAll_EnrichesLanguageShares(t *testing.T) {
	f := newListAllFixture(t)
	stateRepo := storage.NewSQLFileStateRepository(f.database.Conn())
	require.NoError(t, stateRepo.UpsertBatch([]*storage.ProjectFileState{
		{ProjectID: "p1", FilePath: "a.go", FileSize: 750, ContentHash: "h", DigestHash: "d"},
		{ProjectID: "p1", FilePath: "b.py", FileSize: 250, ContentHash: "h", DigestHash: "d"},
	}))
	f.engine.SetFileStateRepo(stateRepo)
	svc := service.NewProjectService(f.projectRepo, f.symbolRepo, f.engine)

	projects, err := svc.ListAll()

	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, []storage.LanguageShare{
		{Name: "go", Percent: 75},
		{Name: "python", Percent: 25},
	}, projects[0].Languages)
}

func TestProjectService_ListAll_LanguageFailureIsBestEffort(t *testing.T) {
	f := newListAllFixture(t)
	f.engine.SetFileStateRepo(failingFileStateRepo{})
	svc := service.NewProjectService(f.projectRepo, f.symbolRepo, f.engine)

	projects, err := svc.ListAll()

	require.NoError(t, err, "falha no cálculo de linguagens nunca derruba a listagem")
	require.Len(t, projects, 1)
	assert.Empty(t, projects[0].Languages)
}
