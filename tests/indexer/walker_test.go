package indexer_test

import (
	"astrix/pkg/indexer"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTreeText(t *testing.T) {
	tempDir := t.TempDir()

	// Cria estrutura de diretórios e arquivos de teste:
	// root/
	// ├── devops/
	// │   └── pr-validation.yml
	// ├── docker/
	// │   ├── postgres-init/
	// │   │   └── init.sql
	// │   └── README.md
	// ├── src/
	// │   ├── api/
	// │   │   └── handler.go
	// │   ├── app.module.ts
	// │   └── main.ts
	// ├── node_modules/ (deve ser ignorado)
	// │   └── pkg/
	// ├── .git/ (deve ser ignorado)
	// │   └── config
	// ├── .env (arquivo oculto)
	// ├── package.json
	// └── tsconfig.json

	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "devops"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "devops", "pr-validation.yml"), []byte("yml"), 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "docker", "postgres-init"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "docker", "postgres-init", "init.sql"), []byte("sql"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "docker", "README.md"), []byte("md"), 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "src", "api"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "src", "api", "handler.go"), []byte("go"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "src", "app.module.ts"), []byte("ts"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "src", "main.ts"), []byte("ts"), 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "node_modules", "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "node_modules", "pkg", "index.js"), []byte("js"), 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".git", "config"), []byte("git"), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".env"), []byte("SECRET=1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte("{}"), 0o644))

	t.Run("depth=2 without hidden", func(t *testing.T) {
		tree, err := indexer.BuildTreeText(tempDir, "", 2, false)
		require.NoError(t, err)

		// Verifica que node_modules e .git não aparecem
		assert.NotContains(t, tree, "node_modules")
		assert.NotContains(t, tree, ".git")
		assert.NotContains(t, tree, ".env")

		// Verifica cabeçalho
		assert.True(t, strings.HasPrefix(tree, "root/\n"))

		// Verifica marcação de (dir) para pastas em depth limite com filhos
		assert.Contains(t, tree, "postgres-init/ (dir)")
		assert.Contains(t, tree, "api/ (dir)")

		// Verifica que arquivos dentro de api/ e postgres-init/ não são expandidos em depth 2
		assert.NotContains(t, tree, "handler.go")
		assert.NotContains(t, tree, "init.sql")

		// Verifica que arquivos de nível 1 e 2 são exibidos
		assert.Contains(t, tree, "pr-validation.yml")
		assert.Contains(t, tree, "README.md")
		assert.Contains(t, tree, "app.module.ts")
		assert.Contains(t, tree, "main.ts")
		assert.Contains(t, tree, "package.json")
		assert.Contains(t, tree, "tsconfig.json")
	})

	t.Run("depth=1", func(t *testing.T) {
		tree, err := indexer.BuildTreeText(tempDir, "", 1, false)
		require.NoError(t, err)

		assert.Contains(t, tree, "devops/ (dir)")
		assert.Contains(t, tree, "docker/ (dir)")
		assert.Contains(t, tree, "src/ (dir)")
		assert.Contains(t, tree, "package.json")
		assert.Contains(t, tree, "tsconfig.json")
		assert.NotContains(t, tree, "README.md")
		assert.NotContains(t, tree, "main.ts")
	})

	t.Run("show_hidden=true", func(t *testing.T) {
		tree, err := indexer.BuildTreeText(tempDir, "", 2, true)
		require.NoError(t, err)

		assert.Contains(t, tree, ".env")
		// .git ainda é ignorado por DefaultIgnoredDirs
		assert.NotContains(t, tree, ".git")
	})

	t.Run("subpath exploration", func(t *testing.T) {
		tree, err := indexer.BuildTreeText(tempDir, "src", 2, false)
		require.NoError(t, err)

		assert.True(t, strings.HasPrefix(tree, "src/\n"))
		assert.Contains(t, tree, "api/")
		assert.Contains(t, tree, "handler.go")
		assert.Contains(t, tree, "app.module.ts")
		assert.Contains(t, tree, "main.ts")
	})
}
