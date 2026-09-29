package db_test

import (
	"astrix/pkg/storage"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataModelRepo_CRUD(t *testing.T) {
	tempDB := t.TempDir() + "/test_data_storage.db"
	database, err := storage.NewDatabase(tempDB)
	require.NoError(t, err)
	defer database.Close()

	projRepo := storage.NewProjectRepo(database)
	err = projRepo.Create(&storage.Project{
		ID:       "proj-1",
		Name:     "Test Project",
		Path:     "/tmp/test",
		Language: "typescript",
		Status:   storage.StatusReady,
	})
	require.NoError(t, err)

	repo := storage.NewDataModelRepo(database)

	modelsList := []*storage.DataModel{
		{
			ProjectID: "proj-1",
			File:      "src/user.dto.ts",
			Name:      "CreateUserDto",
			Kind:      "class",
			Line:      10,
			Fields: []*storage.ModelField{
				{Name: "id", Type: "string", Required: true},
				{Name: "email", Type: "string", Required: true},
				{Name: "bio", Type: "string", Required: false},
			},
		},
		{
			ProjectID: "proj-1",
			File:      "src/models/user.py",
			Name:      "UserModel",
			Kind:      "class",
			Line:      5,
			Fields: []*storage.ModelField{
				{Name: "username", Type: "str", Required: true},
			},
		},
	}

	// 1. SaveDataModels
	err = repo.SaveDataModels("proj-1", modelsList)
	require.NoError(t, err)

	// 2. GetByName (found)
	m, err := repo.GetByName("proj-1", "CreateUserDto")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "CreateUserDto", m.Name)
	assert.Equal(t, "src/user.dto.ts", m.File)
	assert.Equal(t, "class", m.Kind)
	assert.Equal(t, 10, m.Line)
	require.Len(t, m.Fields, 3)
	assert.Equal(t, "id", m.Fields[0].Name)
	assert.Equal(t, "string", m.Fields[0].Type)
	assert.True(t, m.Fields[0].Required)
	assert.False(t, m.Fields[2].Required)

	// 3. GetByName (not found)
	notFound, err := repo.GetByName("proj-1", "NonExistentModel")
	require.NoError(t, err)
	assert.Nil(t, notFound)

	// 4. ListByProject
	list, err := repo.ListByProject("proj-1")
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// 5. ClearProjectDataModels
	err = repo.ClearProjectDataModels("proj-1")
	require.NoError(t, err)

	clearedList, err := repo.ListByProject("proj-1")
	require.NoError(t, err)
	assert.Empty(t, clearedList)
}
