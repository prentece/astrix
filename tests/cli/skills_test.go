package cli_test

import (
	"astrix/internal/cli"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkills_AllStandardAgentsGenerateSkillFolderAndReferences(t *testing.T) {
	expectedPaths := map[cli.AgentType]string{
		cli.AgentAntigravity: filepath.Join(".agents", "skills", "astrix", "SKILL.md"),
		cli.AgentClaude:      filepath.Join(".claude", "skills", "astrix", "SKILL.md"),
		cli.AgentCursor:      filepath.Join(".cursor", "skills", "astrix", "SKILL.md"),
		cli.AgentCopilot:     filepath.Join(".github", "skills", "astrix", "SKILL.md"),
		cli.AgentWindsurf:    filepath.Join(".windsurf", "skills", "astrix", "SKILL.md"),
		cli.AgentCodex:       filepath.Join(".codex", "skills", "astrix", "SKILL.md"),
		cli.AgentOpenCode:    filepath.Join(".opencode", "skills", "astrix", "SKILL.md"),
		cli.AgentCline:       filepath.Join(".cline", "skills", "astrix", "SKILL.md"),
		cli.AgentRoo:         filepath.Join(".roo", "skills", "astrix", "SKILL.md"),
	}

	tempDir := t.TempDir()

	var allAgents []cli.AgentType
	for agent := range expectedPaths {
		allAgents = append(allAgents, agent)
	}
	allAgents = append(allAgents, cli.AgentGeneric)

	err := cli.SetupSkills(tempDir, "proj-1", "Test", allAgents)
	require.NoError(t, err)

	for agent, relSkillPath := range expectedPaths {
		t.Run(string(agent), func(t *testing.T) {
			require.True(t, cli.HasReferenceFile(string(agent)))

			fullSkillPath := filepath.Join(tempDir, relSkillPath)
			require.FileExists(t, fullSkillPath)

			contentBytes, err := os.ReadFile(fullSkillPath)
			require.NoError(t, err)
			content := string(contentBytes)
			assert.True(t, strings.HasPrefix(content, "---\nname: astrix\ndescription: "), "SKILL.md de %s deve conter frontmatter YAML padrão Agent Skills", agent)
			assert.NotContains(t, content, "## Tool combination workflows", "SKILL.md de %s deve usar references/workflows.md separado", agent)

			refPath := filepath.Join(filepath.Dir(fullSkillPath), "references", "workflows.md")
			require.FileExists(t, refPath)
		})
	}

	// Agente genérico gera arquivo único ASTRIX.md com workflows inline
	require.False(t, cli.HasReferenceFile(string(cli.AgentGeneric)))
	genericPath := filepath.Join(tempDir, "ASTRIX.md")
	require.FileExists(t, genericPath)
	genericBytes, err := os.ReadFile(genericPath)
	require.NoError(t, err)
	assert.Contains(t, string(genericBytes), "## Tool combination workflows")

	// Simula arquivo legado .cursor/rules/astrix.mdc para garantir limpeza
	legacyCursorRule := filepath.Join(tempDir, ".cursor", "rules", "astrix.mdc")
	require.NoError(t, os.MkdirAll(filepath.Dir(legacyCursorRule), 0o755))
	require.NoError(t, os.WriteFile(legacyCursorRule, []byte("legacy"), 0o644))

	removed, err := cli.RemoveSkills(tempDir)
	require.NoError(t, err)
	assert.Len(t, removed, len(expectedPaths)+1)

	for _, relSkillPath := range expectedPaths {
		assert.NoFileExists(t, filepath.Join(tempDir, relSkillPath))
	}
	assert.NoFileExists(t, genericPath)
	assert.NoFileExists(t, legacyCursorRule)
}
