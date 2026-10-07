package cli

import (
	"astrix/internal/cli/ui"
	"astrix/internal/service"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// RunClean remove com segurança os dados do projeto no contexto atual com confirmação limpa.
func RunClean(projService *service.ProjectService) error {
	return PrintClean(projService, false)
}

// PrintClean remove o projeto com controle de exibição interativa.
func PrintClean(projService *service.ProjectService, isInteractive ...bool) error {
	currDir, _ := os.Getwd()
	ctx, err := DetectContext(currDir, projService.ProjectRepo())
	if err != nil {
		return err
	}

	if !ctx.IsRegistered || ctx.ProjectID == "" {
		return fmt.Errorf("diretório atual não é um projeto cadastrado no Astrix")
	}

	var confirmDelete bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Description(fmt.Sprintf("Confirmar exclusão definitiva do projeto '%s' (%s)?", ctx.Name, ctx.ProjectID)).
				Affirmative("Excluir projeto").
				Negative("Cancelar").
				Value(&confirmDelete),
		),
	).WithTheme(ui.HuhTheme()).WithKeyMap(ui.HuhKeyMap())

	if len(isInteractive) == 0 || !isInteractive[0] {
		ui.EnterAltScreen()
		defer ui.ExitAltScreen()
	}

	if err := form.Run(); err != nil {
		return err
	}
	if !confirmDelete {
		return huh.ErrUserAborted
	}

	if err := projService.DeleteProject(ctx.ProjectID); err != nil {
		return fmt.Errorf("falha ao remover projeto: %w", err)
	}

	_, _ = RemoveSkills(ctx.RootDir)
	_ = RemoveProjectConfig(ctx.RootDir)

	if len(isInteractive) > 0 && isInteractive[0] {
		ui.ClearScreen()
		fmt.Println(ui.HeaderCard("ASTRIX", "Remoção de Projeto", "Concluído", ctx.RootDir, false))
		fmt.Println()
	}

	status := lipgloss.NewStyle().Foreground(ui.ColorError).Render("removido")
	fmt.Println(ui.KeyValue("Projeto", ctx.Name))
	fmt.Println(ui.KeyValue("Status", status))
	return nil
}
