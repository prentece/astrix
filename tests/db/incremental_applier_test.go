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

func TestIndexStore_ApplyIncrementalDelta_Atomicity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "astrix_inc_applier_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := storage.NewDatabase(dbPath)
	require.NoError(t, err)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())
	indexStore := storage.NewIndexStore(database)

	proj := &storage.Project{
		ID:        "proj-inc-atomic",
		Name:      "Atomic Inc Project",
		Path:      "/tmp/proj",
		Language:  "go",
		Status:    storage.StatusReady,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, projectRepo.Create(proj))

	// Insere estado inicial com 2 arquivos: a.go e b.go
	initialSnap := &storage.IndexSnapshot{
		ProjectID: proj.ID,
		Symbols: []*storage.Symbol{
			{ProjectID: proj.ID, Name: "FuncA", File: "a.go", Kind: storage.KindFunction, StartLine: 1, EndLine: 5, Language: "go"},
			{ProjectID: proj.ID, Name: "FuncB", File: "b.go", Kind: storage.KindFunction, StartLine: 1, EndLine: 5, Language: "go"},
		},
		FileStates: []*storage.ProjectFileState{
			{ProjectID: proj.ID, FilePath: "a.go", ContentHash: "hashA", DigestHash: "dhashA"},
			{ProjectID: proj.ID, FilePath: "b.go", ContentHash: "hashB", DigestHash: "dhashB"},
		},
		ReplaceFileStates: true,
	}
	require.NoError(t, indexStore.ReplaceProjectIndex(initialSnap))

	// 1. Aplica delta incremental: deleta b.go, modifica a.go e adiciona c.go
	incDelta := &storage.IncrementalIndexDelta{
		ProjectID:     proj.ID,
		DeletedFiles:  []string{"b.go"},
		ModifiedFiles: []string{"a.go"},
		Symbols: []*storage.Symbol{
			{ProjectID: proj.ID, Name: "FuncAModified", File: "a.go", Kind: storage.KindFunction, StartLine: 1, EndLine: 8, Language: "go"},
			{ProjectID: proj.ID, Name: "FuncC", File: "c.go", Kind: storage.KindFunction, StartLine: 1, EndLine: 10, Language: "go"},
		},
		FileStates: []*storage.ProjectFileState{
			{ProjectID: proj.ID, FilePath: "a.go", ContentHash: "hashA2", DigestHash: "dhashA2"},
			{ProjectID: proj.ID, FilePath: "c.go", ContentHash: "hashC", DigestHash: "dhashC"},
		},
	}
	require.NoError(t, indexStore.ApplyIncrementalDelta(incDelta))

	// Verifica símbolos atualizados
	symbols, err := symbolRepo.GetAllSymbolsForRanking(proj.ID)
	require.NoError(t, err)
	require.Len(t, symbols, 2)

	symNames := []string{symbols[0].Name, symbols[1].Name}
	assert.Contains(t, symNames, "FuncAModified")
	assert.Contains(t, symNames, "FuncC")
	assert.NotContains(t, symNames, "FuncA")
	assert.NotContains(t, symNames, "FuncB")

	// Verifica estados de arquivos atualizados
	states, err := fileStateRepo.ListByProject(proj.ID)
	require.NoError(t, err)
	assert.Len(t, states, 2)
	assert.NotNil(t, states["a.go"])
	assert.Equal(t, "hashA2", states["a.go"].ContentHash)
	assert.NotNil(t, states["c.go"])
	assert.Nil(t, states["b.go"])
}

func TestIndexStore_ApplyIncrementalDelta_RollbackOnFailure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "astrix_inc_rb_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := storage.NewDatabase(dbPath)
	require.NoError(t, err)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	indexStore := storage.NewIndexStore(database)

	proj := &storage.Project{
		ID:        "proj-inc-rb",
		Name:      "Rollback Project",
		Path:      "/tmp/proj",
		Language:  "go",
		Status:    storage.StatusReady,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, projectRepo.Create(proj))

	initialSnap := &storage.IndexSnapshot{
		ProjectID: proj.ID,
		Symbols: []*storage.Symbol{
			{ProjectID: proj.ID, Name: "OriginalFunc", File: "a.go", Kind: storage.KindFunction, StartLine: 1, EndLine: 5, Language: "go"},
		},
		FileStates: []*storage.ProjectFileState{
			{ProjectID: proj.ID, FilePath: "a.go", ContentHash: "hashA", DigestHash: "dhashA"},
		},
		ReplaceFileStates: true,
	}
	require.NoError(t, indexStore.ReplaceProjectIndex(initialSnap))

	// Delta com violação de FK proposital no FileState ("ghost" project)
	badDelta := &storage.IncrementalIndexDelta{
		ProjectID:     proj.ID,
		ModifiedFiles: []string{"a.go"},
		Symbols: []*storage.Symbol{
			{ProjectID: proj.ID, Name: "ModifiedFunc", File: "a.go", Kind: storage.KindFunction, StartLine: 1, EndLine: 8, Language: "go"},
		},
		FileStates: []*storage.ProjectFileState{
			{ProjectID: "ghost-project", FilePath: "a.go", ContentHash: "hashA_new", DigestHash: "dhashA_new"},
		},
	}

	err = indexStore.ApplyIncrementalDelta(badDelta)
	require.Error(t, err)

	// O estado anterior deve ser preservado intacto (rollback)
	symbols, err := symbolRepo.GetAllSymbolsForRanking(proj.ID)
	require.NoError(t, err)
	require.Len(t, symbols, 1)
	assert.Equal(t, "OriginalFunc", symbols[0].Name)
}
