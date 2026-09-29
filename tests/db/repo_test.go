package db_test

import (
	"astrix/pkg/storage"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectRepo_CRUD(t *testing.T) {
	tempDB := t.TempDir() + "/test_repo.db"
	database, err := storage.NewDatabase(tempDB)
	require.NoError(t, err)
	defer database.Close()

	repo := storage.NewProjectRepo(database)

	// 1. Create
	proj := &storage.Project{
		ID:       "proj-1",
		Name:     "Test Project",
		Path:     "/test/path",
		Language: "go",
	}
	err = repo.Create(proj)
	require.NoError(t, err)

	// 2. GetByID
	saved, err := repo.GetByID("proj-1")
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, "Test Project", saved.Name)
	assert.Equal(t, storage.StatusPending, saved.Status)

	// 3. UpdateStatus
	err = repo.UpdateStatus("proj-1", storage.StatusReady, "", 10, 50)
	require.NoError(t, err)

	updated, err := repo.GetByID("proj-1")
	require.NoError(t, err)
	assert.Equal(t, storage.StatusReady, updated.Status)
	assert.Equal(t, 10, updated.FileCount)
	assert.Equal(t, 50, updated.SymbolCount)
	assert.NotNil(t, updated.IndexedAt)

	// 4. SetAutoSync
	err = repo.SetAutoSync("proj-1", true)
	require.NoError(t, err)

	updated, err = repo.GetByID("proj-1")
	require.NoError(t, err)
	assert.True(t, updated.AutoSync)

	err = repo.SetAutoSync("proj-1", false)
	require.NoError(t, err)

	updated, err = repo.GetByID("proj-1")
	require.NoError(t, err)
	assert.False(t, updated.AutoSync)

	// 5. ListAll
	list, err := repo.ListAll()
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.False(t, list[0].AutoSync)

	// 6. Delete
	err = repo.Delete("proj-1")
	require.NoError(t, err)

	deleted, err := repo.GetByID("proj-1")
	require.NoError(t, err)
	assert.Nil(t, deleted)
}

