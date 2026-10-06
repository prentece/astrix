package cli_test

import (
	"astrix/internal/cli"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLI_ContextDetection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "astrix_cli_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 1. Diretório vazio não é projeto
	ctx, err := cli.DetectContext(tempDir)
	require.NoError(t, err)
	assert.False(t, ctx.IsProject)
	assert.False(t, ctx.IsRegistered)

	// 2. Adiciona go.mod -> vira projeto válido não cadastrado
	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/test\n"), 0644)
	ctx, err = cli.DetectContext(tempDir)
	require.NoError(t, err)
	assert.True(t, ctx.IsProject)
	assert.False(t, ctx.IsRegistered)
	assert.Equal(t, "go", ctx.DetectedLang)

	// 3. Grava .astrix -> vira projeto cadastrado
	err = cli.SaveProjectConfig(tempDir, "proj-12345", "TestProject", "go")
	require.NoError(t, err)

	ctx, err = cli.DetectContext(tempDir)
	require.NoError(t, err)
	assert.True(t, ctx.IsProject)
	assert.True(t, ctx.IsRegistered)
	assert.Equal(t, "proj-12345", ctx.ProjectID)
	assert.Equal(t, "TestProject", ctx.Name)

	// 4. Setup de Skills
	err = cli.SetupSkills(tempDir, ctx.ProjectID, ctx.Name, []cli.AgentType{cli.AgentAntigravity, cli.AgentCursor})
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempDir, ".agents", "skills", "astrix", "SKILL.md"))
	assert.FileExists(t, filepath.Join(tempDir, ".cursor", "rules", "astrix.mdc"))

	// 5. Limpeza de Skills e Config
	_, _ = cli.RemoveSkills(tempDir)
	assert.NoFileExists(t, filepath.Join(tempDir, ".cursor", "rules", "astrix.mdc"))
	assert.NoFileExists(t, filepath.Join(tempDir, ".agents", "skills", "astrix", "SKILL.md"))

	err = cli.RemoveProjectConfig(tempDir)
	require.NoError(t, err)

	ctx, err = cli.DetectContext(tempDir)
	require.NoError(t, err)
	assert.False(t, ctx.IsRegistered)
}

func TestCLI_ManifestNameDetection(t *testing.T) {
	// 1. Projeto com package.json contendo "name": "nio" em uma pasta qualquer "fibra-frontend"
	t.Run("uses manifest name when present", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "fibra-frontend-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		_ = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name": "Nio"}`), 0644)

		ctx, err := cli.DetectContext(tempDir)
		require.NoError(t, err)
		assert.True(t, ctx.IsProject)
		assert.Equal(t, "Nio", ctx.Name)
		assert.Equal(t, "javascript", ctx.DetectedLang)
	})

	// 2. Projeto sem manifesto formata o nome da pasta
	t.Run("formats folder name when no manifest name", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "minha-ferramenta-cli-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		_ = os.WriteFile(filepath.Join(tempDir, ".git"), []byte(""), 0644)

		ctx, err := cli.DetectContext(tempDir)
		require.NoError(t, err)
		assert.True(t, ctx.IsProject)
		assert.Contains(t, ctx.Name, "Minha Ferramenta Cli")
	})
}

