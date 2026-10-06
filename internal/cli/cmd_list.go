package cli

import (
	"astrix/internal/cli/ui"
	"astrix/internal/service"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// RunList executa o comando 'ls' exibindo os projetos cadastrados.
func RunList(projService *service.ProjectService, args ...string) error {
	isJSON := false
	for _, a := range args {
		if a == "--json" {
			isJSON = true
			break
		}
	}
	return PrintList(projService, isJSON)
}

// PrintList renderiza a lista de projetos cadastrados.
func PrintList(projService *service.ProjectService, isJSON ...bool) error {
	projects, err := projService.ListAll()
	if err != nil {
		return fmt.Errorf("falha ao listar projetos: %w", err)
	}

	if len(isJSON) > 0 && isJSON[0] {
		data, err := json.MarshalIndent(projects, "", "  ")
		if err != nil {
			return fmt.Errorf("falha ao formatar JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if len(projects) == 0 {
		fmt.Println(ui.WarningBox("Nenhum Projeto Encontrado", "Execute 'astrix' na raiz de um repositório para cadastrá-lo."))
		return nil
	}

	for i, p := range projects {
		title := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary).Render(p.Name)
		id := lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(fmt.Sprintf("(%s)", p.ID))
		fmt.Printf("%s %s\n", title, id)

		status := lipgloss.NewStyle().Foreground(ui.ColorSuccess).Render(string(p.Status))
		if p.Status != "ready" {
			status = lipgloss.NewStyle().Foreground(ui.ColorWarning).Render(string(p.Status))
		}

		fmt.Println(ui.KeyValue("Linguagem", p.Language))
		fmt.Println(ui.KeyValue("Status", fmt.Sprintf("%s (%d arquivos, %d símbolos)", status, p.FileCount, p.SymbolCount)))
		fmt.Println(ui.KeyValue("Caminho", p.Path))

		if i < len(projects)-1 {
			fmt.Println()
		}
	}
	fmt.Println()

	return nil
}
