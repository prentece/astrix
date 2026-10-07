package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AgentType representa o identificador do assistente/agente de IA.
type AgentType string

const (
	AgentAntigravity AgentType = "antigravity"
	AgentClaude      AgentType = "claude"
	AgentCursor      AgentType = "cursor"
	AgentCopilot     AgentType = "copilot"
	AgentGeneric     AgentType = "generic"
	AgentGemini      AgentType = "antigravity" // alias para Antigravity
)

// AgentTarget descreve um assistente/agente de IA suportado.
type AgentTarget struct {
	ID          string
	Name        string
	Description string
	FilePath    string
}

// GetAvailableAgents retorna a lista de agentes de IA suportados para geração de skills.
func GetAvailableAgents() []AgentTarget {
	return []AgentTarget{
		{
			ID:          "antigravity",
			Name:        "Antigravity / Gemini",
			Description: "Gera .agents/skills/astrix/SKILL.md",
			FilePath:    filepath.Join(".agents", "skills", "astrix", "SKILL.md"),
		},
		{
			ID:          "claude",
			Name:        "Claude Code",
			Description: "Gera .claude/skills/astrix/SKILL.md",
			FilePath:    filepath.Join(".claude", "skills", "astrix", "SKILL.md"),
		},
		{
			ID:          "cursor",
			Name:        "Cursor",
			Description: "Gera .cursor/rules/astrix.mdc",
			FilePath:    filepath.Join(".cursor", "rules", "astrix.mdc"),
		},
		{
			ID:          "copilot",
			Name:        "VS Code Copilot",
			Description: "Gera .github/copilot-instructions.md",
			FilePath:    filepath.Join(".github", "copilot-instructions.md"),
		},
		{
			ID:          "generic",
			Name:        "Genérico",
			Description: "Gera ASTRIX.md na raiz do repositório",
			FilePath:    "ASTRIX.md",
		},
	}
}

// GenerateSkill gera as instruções ou skill para o agente especificado na raiz do projeto.
func GenerateSkill(rootDir, agentID, projectID, projectName string) (string, error) {
	var target *AgentTarget
	for _, a := range GetAvailableAgents() {
		if a.ID == agentID {
			target = &a
			break
		}
	}
	if target == nil {
		return "", fmt.Errorf("agente '%s' não suportado", agentID)
	}

	destPath := filepath.Join(rootDir, target.FilePath)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return "", fmt.Errorf("falha ao criar diretório para a skill: %w", err)
	}

	content := BuildSkillContent(agentID)
	if err := os.WriteFile(destPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("falha ao gravar arquivo de skill em %s: %w", destPath, err)
	}

	if HasReferenceFile(agentID) {
		refPath := filepath.Join(filepath.Dir(destPath), filepath.FromSlash(ReferenceRelPath))
		if err := os.MkdirAll(filepath.Dir(refPath), 0o755); err != nil {
			return "", fmt.Errorf("falha ao criar diretório de references: %w", err)
		}
		if err := os.WriteFile(refPath, []byte(BuildReferenceContent()), 0o644); err != nil {
			return "", fmt.Errorf("falha ao gravar reference em %s: %w", refPath, err)
		}
	}

	return destPath, nil
}

// SetupSkills cria ou atualiza as configurações de Skills para os agentes selecionados.
func SetupSkills(projectDir, projectID, projectName string, agents []AgentType) error {
	for _, agent := range agents {
		if _, err := GenerateSkill(projectDir, string(agent), projectID, projectName); err != nil {
			return err
		}
	}
	return nil
}

// RemoveSkills remove todos os arquivos e diretórios de skill gerados para o Astrix no repositório.
func RemoveSkills(rootDir string) ([]string, error) {
	var removed []string
	for _, target := range GetAvailableAgents() {
		targetPath := filepath.Join(rootDir, target.FilePath)
		if HasReferenceFile(target.ID) {
			refPath := filepath.Join(filepath.Dir(targetPath), filepath.FromSlash(ReferenceRelPath))
			_ = os.Remove(refPath)
			_ = os.Remove(filepath.Dir(refPath))
		}
		if _, err := os.Stat(targetPath); err == nil {
			if err := os.Remove(targetPath); err == nil {
				removed = append(removed, target.FilePath)
			}
			// Se for um diretório de skill (ex: .agents/skills/astrix), tenta remover a pasta
			parentDir := filepath.Dir(targetPath)
			if parentDir != rootDir {
				_ = os.Remove(parentDir)
				skillsDir := filepath.Dir(parentDir)
				if skillsDir != rootDir && strings.HasSuffix(parentDir, "astrix") {
					_ = os.Remove(skillsDir)
				}
			}
		}
	}

	// Limpeza de arquivos legados se existirem
	legacyFiles := []string{
		filepath.Join(rootDir, ".agents", "rules", "astrix-skill.md"),
		filepath.Join(rootDir, ".claude", "astrix.json"),
	}
	for _, lf := range legacyFiles {
		if _, err := os.Stat(lf); err == nil {
			_ = os.Remove(lf)
		}
	}

	// Limpa pastas vazias remanescentes
	removeIfEmpty(filepath.Join(rootDir, ".agents", "skills"))
	removeIfEmpty(filepath.Join(rootDir, ".agents", "rules"))
	removeIfEmpty(filepath.Join(rootDir, ".agents"))
	removeIfEmpty(filepath.Join(rootDir, ".cursor", "rules"))
	removeIfEmpty(filepath.Join(rootDir, ".cursor"))
	removeIfEmpty(filepath.Join(rootDir, ".claude", "skills"))
	removeIfEmpty(filepath.Join(rootDir, ".claude"))
	removeIfEmpty(filepath.Join(rootDir, ".github"))

	return removed, nil
}

func removeIfEmpty(dir string) {
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) == 0 {
		_ = os.Remove(dir)
	}
}
