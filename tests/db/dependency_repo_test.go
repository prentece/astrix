package db_test

import (
	"astrix/pkg/storage"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDependencyGraphRepo_SaveAndQuery(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_dep.db")

	database, err := storage.NewDatabase(dbPath)
	require.NoError(t, err)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)

	proj := &storage.Project{
		ID:       "proj-dep-test",
		Name:     "Dep Test",
		Path:     "/test/dep",
		Language: "typescript",
		Status:   storage.StatusReady,
	}
	require.NoError(t, projectRepo.Create(proj))

	edges := []*storage.DependencyEdge{
		{
			ProjectID:        proj.ID,
			SourceSymbol:     "AddressController",
			TargetSymbol:     "AddressService",
			SourceFile:       "src/api/address.controller.ts",
			TargetFile:       "",
			RelationshipType: "injects",
			CreatedAt:        time.Now(),
		},
		{
			ProjectID:        proj.ID,
			SourceSymbol:     "AddressService",
			TargetSymbol:     "AddressDao",
			SourceFile:       "src/api/address.service.ts",
			TargetFile:       "",
			RelationshipType: "injects",
			CreatedAt:        time.Now(),
		},
		{
			ProjectID:        proj.ID,
			SourceSymbol:     "AddressDao",
			TargetSymbol:     "MongooseModel",
			SourceFile:       "src/api/address.dao.ts",
			TargetFile:       "",
			RelationshipType: "uses",
			CreatedAt:        time.Now(),
		},
		{
			ProjectID:        proj.ID,
			SourceSymbol:     "OrderController",
			TargetSymbol:     "AddressService",
			SourceFile:       "src/api/order.controller.ts",
			TargetFile:       "",
			RelationshipType: "injects",
			CreatedAt:        time.Now(),
		},
	}

	err = depRepo.SaveDependencies(proj.ID, edges)
	require.NoError(t, err)

	// 1. Downstream from AddressController
	downstream, err := depRepo.GetDownstreamEdges(proj.ID, "AddressController")
	require.NoError(t, err)
	require.Len(t, downstream, 1)
	assert.Equal(t, "AddressService", downstream[0].TargetSymbol)
	assert.Equal(t, "injects", downstream[0].RelationshipType)

	// 2. Upstream to AddressService (called by AddressController and OrderController)
	upstream, err := depRepo.GetUpstreamEdges(proj.ID, "AddressService")
	require.NoError(t, err)
	require.Len(t, upstream, 2)
	assert.Equal(t, "AddressController", upstream[0].SourceSymbol)
	assert.Equal(t, "OrderController", upstream[1].SourceSymbol)

	// 3. Update target files
	symMap := map[string]string{
		"AddressService": "src/api/address.service.ts",
		"AddressDao":     "src/api/address.dao.ts",
	}
	err = depRepo.UpdateTargetFiles(proj.ID, symMap)
	require.NoError(t, err)

	downstreamAfter, err := depRepo.GetDownstreamEdges(proj.ID, "AddressController")
	require.NoError(t, err)
	require.Len(t, downstreamAfter, 1)
	assert.Equal(t, "src/api/address.service.ts", downstreamAfter[0].TargetFile)

	// 4. Clear dependencies
	err = depRepo.ClearProjectDependencies(proj.ID)
	require.NoError(t, err)

	downstreamEmpty, err := depRepo.GetDownstreamEdges(proj.ID, "AddressController")
	require.NoError(t, err)
	assert.Empty(t, downstreamEmpty)
}
