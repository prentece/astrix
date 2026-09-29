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
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return "", fmt.Errorf("falha ao criar diretório para a skill: %w", err)
	}

	content := BuildSkillContent(agentID, projectID, projectName)
	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("falha ao gravar arquivo de skill em %s: %w", destPath, err)
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

// BuildSkillContent gera o texto de instrução Markdown formatado com parâmetros do projeto.
func BuildSkillContent(agentID, projectID, projectName string) string {
	var sb strings.Builder

	if agentID == "antigravity" || agentID == "claude" {
		sb.WriteString("---\n")
		sb.WriteString("name: astrix\n")
		sb.WriteString("description: Navegação e inspeção de código baseada em AST e índices estruturados com o Astrix MCP\n")
		sb.WriteString(fmt.Sprintf("project_id: %q\n", projectID))
		sb.WriteString("---\n\n")
	} else if agentID == "cursor" {
		sb.WriteString("---\n")
		sb.WriteString("description: Diretrizes de navegação no projeto via Astrix MCP\n")
		sb.WriteString("globs: *\n")
		sb.WriteString("alwaysApply: true\n")
		sb.WriteString("---\n\n")
	}

	sb.WriteString(fmt.Sprintf("# Astrix: %s\n\n", projectName))
	sb.WriteString(fmt.Sprintf("Este projeto está indexado no **Astrix** sob o Project ID: `%s`.\n\n", projectID))
	sb.WriteString("Ao inspecionar, navegar, debugar ou refatorar este código, utilize sempre o servidor MCP do Astrix com as ferramentas padronizadas abaixo:\n\n")
	sb.WriteString("## Catálogo de Ferramentas MCP\n\n")

	sb.WriteString("1. **`get_project_structure`**:\n")
	sb.WriteString(fmt.Sprintf("   - Argumento: `{\"project_id\": %q}`\n", projectID))
	sb.WriteString("   - Retorna a árvore hierárquica completa de arquivos suportados. Use para entender a topologia do projeto.\n\n")

	sb.WriteString("2. **`lookup_symbol`**:\n")
	sb.WriteString(fmt.Sprintf("   - Argumentos: `{\"project_id\": %q, \"symbol_name\": \"<Nome>\", \"mode\": \"definition|references\"}`\n", projectID))
	sb.WriteString("   - Localiza declarações e referências de símbolos no código.\n\n")

	sb.WriteString("3. **`get_implementation`**:\n")
	sb.WriteString(fmt.Sprintf("   - Argumentos: `{\"project_id\": %q, \"filepath\": \"<caminho_relativo>\", \"symbol_name\": \"<Nome>\"}`\n", projectID))
	sb.WriteString("   - Recupera a implementação exata de uma função, método, struct ou classe sem ler o arquivo inteiro.\n\n")

	sb.WriteString("4. **`get_implementation_bundle`**:\n")
	sb.WriteString(fmt.Sprintf("   - Argumentos: `{\"project_id\": %q, \"symbols\": \"[{\\\"filepath\\\": \\\"...\\\", \\\"symbol_name\\\": \\\"...\\\"}]\"}`\n", projectID))
	sb.WriteString("   - Recupera múltiplos símbolos simultaneamente em uma única requisição.\n\n")

	sb.WriteString("5. **`read_file_lines`**:\n")
	sb.WriteString(fmt.Sprintf("   - Argumentos: `{\"project_id\": %q, \"filepath\": \"<caminho_relativo>\", \"start_line\": 1, \"end_line\": 50}`\n", projectID))
	sb.WriteString("   - Lê um bloco específico de linhas de um arquivo.\n\n")

	sb.WriteString("6. **`grep_code`**:\n")
	sb.WriteString(fmt.Sprintf("   - Argumentos: `{\"project_id\": %q, \"pattern\": \"<texto_ou_regex>\"}`\n", projectID))
	sb.WriteString("   - Busca de ocorrências textuais ou expressões regulares no projeto.\n\n")

	sb.WriteString("7. **`query_structured_file`**:\n")
	sb.WriteString(fmt.Sprintf("   - Argumentos: `{\"project_id\": %q, \"filepath\": \"<caminho>\", \"query\": \"chave.subchave\"}`\n", projectID))
	sb.WriteString("   - Consulta valores específicos em arquivos de configuração JSON, YAML ou CSV sem carregar o arquivo na íntegra.\n\n")

	sb.WriteString("8. **`list_projects`**:\n")
	sb.WriteString("   - Retorna todos os projetos registrados no catálogo do Astrix.\n\n")

	sb.WriteString("## Boas Práticas\n")
	sb.WriteString("- Prefira `lookup_symbol` e `get_implementation` a leituras indiscriminadas de arquivos inteiros.\n")
	sb.WriteString("- Antes de refatorar ou renomear símbolos, utilize `lookup_symbol` com `mode=\"references\"` para análise de impacto.\n")

	return sb.String()
}
