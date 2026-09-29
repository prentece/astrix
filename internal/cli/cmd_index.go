package cli

import (
	"astrix/internal/cli/ui"
	"astrix/internal/service"
	"astrix/pkg/storage"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

// RunIndex força a reindexação do projeto no contexto atual.
func RunIndex(projService *service.ProjectService) error {
	return PrintIndex(projService, true)
}

// PrintIndex executa a reindexação com controle de exibição de banner.
func PrintIndex(projService *service.ProjectService, showBanner bool) error {
	currDir, _ := os.Getwd()
	ctx, err := DetectContext(currDir, projService.ProjectRepo())
	if err != nil {
		return err
	}

	if !ctx.IsRegistered || ctx.ProjectID == "" {
		return fmt.Errorf("diretório atual não é um projeto cadastrado no Astrix.\nExecute 'astrix' para cadastrar primeiro")
	}

	if showBanner {
		fmt.Println(ui.Banner("Indexação Sintática"))
	}

	var proj *storage.Project
	err = ui.RunWithSpinner(fmt.Sprintf("Reindexando '%s'...", ctx.Name), func() error {
		p, err := projService.ReindexProject(ctx.ProjectID)
		if err != nil {
			return err
		}
		proj = p
		return nil
	})

	if err != nil {
		fmt.Println(ui.ErrorBox("Falha na Indexação", err.Error()))
		return err
	}

	status := lipgloss.NewStyle().Foreground(ui.ColorSuccess).Render(string(proj.Status))
	if proj.Status != "ready" {
		status = lipgloss.NewStyle().Foreground(ui.ColorWarning).Render(string(proj.Status))
	}

	fmt.Println(ui.KeyValue("Projeto", proj.Name))
	fmt.Println(ui.KeyValue("Linguagem", proj.Language))
	fmt.Println(ui.KeyValue("Status", status))
	fmt.Println(ui.KeyValue("Arquivos", fmt.Sprintf("%d", proj.FileCount)))
	fmt.Println(ui.KeyValue("Símbolos", fmt.Sprintf("%d", proj.SymbolCount)))
	return nil
}
