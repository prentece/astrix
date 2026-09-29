package indexer_test

import (
	"astrix/pkg/indexer"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryStructuredFile_JSON(t *testing.T) {
	tempDir := t.TempDir()
	jsonContent := `{
		"name": "my-app",
		"version": "1.0.0",
		"dependencies": {
			"@nestjs/core": "7.6.14",
			"@nestjs/common": "7.6.14"
		},
		"scripts": {
			"build": "nest build",
			"start": "nest start"
		},
		"users": [
			{"id": 1, "name": "Alice", "role": "admin", "email": "alice@example.com"},
			{"id": 2, "name": "Bob", "role": "user", "email": "bob@example.com"}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(jsonContent), 0o644))

	t.Run("Extract primitive string value", func(t *testing.T) {
		val, err := indexer.QueryStructuredFile(tempDir, "package.json", "dependencies.@nestjs/core")
		require.NoError(t, err)
		assert.Equal(t, "7.6.14", val)
	})

	t.Run("Extract scripts.build", func(t *testing.T) {
		val, err := indexer.QueryStructuredFile(tempDir, "package.json", "scripts.build")
		require.NoError(t, err)
		assert.Equal(t, "nest build", val)
	})

	t.Run("Extract nested array query", func(t *testing.T) {
		val, err := indexer.QueryStructuredFile(tempDir, "package.json", `users.#(role=="admin").email`)
		require.NoError(t, err)
		assert.Equal(t, "alice@example.com", val)
	})

	t.Run("Extract object returns minified JSON", func(t *testing.T) {
		val, err := indexer.QueryStructuredFile(tempDir, "package.json", "dependencies")
		require.NoError(t, err)
		assert.Equal(t, `{"@nestjs/core":"7.6.14","@nestjs/common":"7.6.14"}`, val)
	})

	t.Run("Not found path", func(t *testing.T) {
		val, err := indexer.QueryStructuredFile(tempDir, "package.json", "dependencies.missing")
		require.NoError(t, err)
		assert.Contains(t, val, "[Not Found]")
	})
}

func TestQueryStructuredFile_YAML(t *testing.T) {
	tempDir := t.TempDir()
	yamlContent := `version: "3.8"
services:
  postgres:
    image: postgres:15-alpine
    ports:
      - "5432:5432"
    environment:
      POSTGRES_DB: app
      POSTGRES_USER: admin
  redis:
    image: redis:7
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "docker-compose.yml"), []byte(yamlContent), 0o644))

	t.Run("Extract scalar value", func(t *testing.T) {
		val, err := indexer.QueryStructuredFile(tempDir, "docker-compose.yml", "version")
		require.NoError(t, err)
		assert.Equal(t, "3.8", val)
	})

	t.Run("Extract nested mapping scalar", func(t *testing.T) {
		val, err := indexer.QueryStructuredFile(tempDir, "docker-compose.yml", "services.postgres.image")
		require.NoError(t, err)
		assert.Equal(t, "postgres:15-alpine", val)
	})

	t.Run("Extract sequence item", func(t *testing.T) {
		val, err := indexer.QueryStructuredFile(tempDir, "docker-compose.yml", "services.postgres.ports.0")
		require.NoError(t, err)
		assert.Equal(t, "5432:5432", val)
	})

	t.Run("Extract subtree", func(t *testing.T) {
		val, err := indexer.QueryStructuredFile(tempDir, "docker-compose.yml", "services.postgres.environment")
		require.NoError(t, err)
		assert.Contains(t, val, "POSTGRES_DB: app")
		assert.Contains(t, val, "POSTGRES_USER: admin")
	})

	t.Run("Not found path", func(t *testing.T) {
		val, err := indexer.QueryStructuredFile(tempDir, "docker-compose.yml", "services.mysql")
		require.NoError(t, err)
		assert.Contains(t, val, "[Not Found]")
	})
}

func TestQueryStructuredFile_CSV(t *testing.T) {
	tempDir := t.TempDir()
	csvContent := `id,name,role,status
1,Alice,admin,ACTIVE
2,Bob,user,PENDING
3,Charlie,manager,ACTIVE
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "users.csv"), []byte(csvContent), 0o644))

	t.Run("Filter by column value", func(t *testing.T) {
		val, err := indexer.QueryStructuredFile(tempDir, "users.csv", "filter:status=ACTIVE")
		require.NoError(t, err)
		assert.Contains(t, val, "Alice")
		assert.Contains(t, val, "Charlie")
		assert.NotContains(t, val, "Bob")
	})

	t.Run("Select specific columns", func(t *testing.T) {
		val, err := indexer.QueryStructuredFile(tempDir, "users.csv", "columns:id,name")
		require.NoError(t, err)
		assert.Contains(t, val, "id,name")
		assert.Contains(t, val, "1,Alice")
		assert.NotContains(t, val, "ACTIVE")
	})

	t.Run("Select single column", func(t *testing.T) {
		val, err := indexer.QueryStructuredFile(tempDir, "users.csv", "name")
		require.NoError(t, err)
		assert.Contains(t, val, "Alice")
		assert.Contains(t, val, "Bob")
		assert.Contains(t, val, "Charlie")
	})
}
