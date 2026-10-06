package db_test

import (
	"astrix/pkg/storage"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSnapshot(projectID, fileSuffix string) *storage.IndexSnapshot {
	file := "main" + fileSuffix + ".go"
	return &storage.IndexSnapshot{
		ProjectID: projectID,
		Symbols: []*storage.Symbol{
			{File: file, Name: "Run" + fileSuffix, Kind: storage.KindFunction, Language: "go", StartLine: 1, EndLine: 3, StartByte: 0, EndByte: 10},
		},
		References: []*storage.CallerInfo{
			{File: file, Line: 2, Text: "Run()", SymbolName: "Run" + fileSuffix},
		},
		Dependencies: []*storage.DependencyEdge{
			{SourceSymbol: "A" + fileSuffix, TargetSymbol: "B", SourceFile: file, RelationshipType: "calls"},
		},
		DataModels: []*storage.DataModel{
			{Name: "Model" + fileSuffix, File: file, Kind: "struct", Line: 5},
		},
		FileStates: []*storage.ProjectFileState{
			{ProjectID: projectID, FilePath: file, MTime: 1, FileSize: 10, ContentHash: "c" + fileSuffix, DigestHash: "d" + fileSuffix},
		},
		ReplaceFileStates: true,
	}
}

func TestIndexStore_ReplaceProjectIndex_ReplacesAll(t *testing.T) {
	database, err := storage.NewDatabase(t.TempDir() + "/idx.db")
	require.NoError(t, err)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	require.NoError(t, projectRepo.Create(&storage.Project{ID: "p1", Name: "P1", Path: "/p1", Language: "go"}))

	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	modelRepo := storage.NewDataModelRepo(database)
	stateRepo := storage.NewSQLFileStateRepository(database.Conn())
	store := storage.NewIndexStore(database)

	require.NoError(t, store.ReplaceProjectIndex(newSnapshot("p1", "_old")))
	require.NoError(t, store.ReplaceProjectIndex(newSnapshot("p1", "_new")))

	syms, _, err := symbolRepo.FindSymbol("p1", "Run", 10, 0)
	require.NoError(t, err)
	require.Len(t, syms, 1, "o índice antigo deve ter sido substituído")
	assert.Equal(t, "Run_new", syms[0].Name)

	edges, err := depRepo.GetAllEdges("p1")
	require.NoError(t, err)
	require.Len(t, edges, 1)
	assert.Equal(t, "A_new", edges[0].SourceSymbol)

	models, err := modelRepo.ListByProject("p1")
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "Model_new", models[0].Name)

	states, err := stateRepo.ListByProject("p1")
	require.NoError(t, err)
	require.Len(t, states, 1)
	assert.Contains(t, states, "main_new.go")
}

func TestIndexStore_ReplaceProjectIndex_RollbackKeepsOldIndex(t *testing.T) {
	database, err := storage.NewDatabase(t.TempDir() + "/idx_rb.db")
	require.NoError(t, err)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	require.NoError(t, projectRepo.Create(&storage.Project{ID: "p1", Name: "P1", Path: "/p1", Language: "go"}))

	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	stateRepo := storage.NewSQLFileStateRepository(database.Conn())
	store := storage.NewIndexStore(database)

	require.NoError(t, store.ReplaceProjectIndex(newSnapshot("p1", "_old")))

	// Snapshot que falha na última etapa (violação de FK em project_file_states: projeto inexistente).
	bad := newSnapshot("p1", "_new")
	bad.FileStates[0].ProjectID = "ghost"
	err = store.ReplaceProjectIndex(bad)
	require.Error(t, err)

	// Todo o índice anterior deve permanecer intacto (rollback completo).
	syms, _, err := symbolRepo.FindSymbol("p1", "Run", 10, 0)
	require.NoError(t, err)
	require.Len(t, syms, 1)
	assert.Equal(t, "Run_old", syms[0].Name)

	edges, err := depRepo.GetAllEdges("p1")
	require.NoError(t, err)
	require.Len(t, edges, 1)
	assert.Equal(t, "A_old", edges[0].SourceSymbol)

	states, err := stateRepo.ListByProject("p1")
	require.NoError(t, err)
	assert.Contains(t, states, "main_old.go")
}

func TestIndexStore_ReplaceProjectIndex_InvalidSnapshot(t *testing.T) {
	database, err := storage.NewDatabase(t.TempDir() + "/idx_inv.db")
	require.NoError(t, err)
	defer database.Close()

	store := storage.NewIndexStore(database)
	assert.Error(t, store.ReplaceProjectIndex(nil))
	assert.Error(t, store.ReplaceProjectIndex(&storage.IndexSnapshot{}))
}
