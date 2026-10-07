package cli

import (
	"astrix/internal/cli/ui"
	"astrix/internal/service"
	"astrix/pkg/storage"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
)

// RunWizard executa o fluxo interativo de onboarding e cadastro do projeto atual utilizando Huh?.
func RunWizard(projService *service.ProjectService, ctx *ProjectContext) error {
	ui.EnterAltScreen()
	defer ui.ExitAltScreen()

	projectName := ctx.Name

	var options []huh.Option[string]
	for _, target := range GetAvailableAgents() {
		opt := huh.NewOption(fmt.Sprintf("%s (%s)", target.Name, target.FilePath), target.ID)
		if target.ID == "antigravity" {
			opt = opt.Selected(true)
		}
		options = append(options, opt)
	}

	var selectedAgentStrings []string
	confirmSetup := true

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Nome do Projeto").
				Value(&projectName).
				Validate(func(str string) error {
					trimmed := strings.TrimSpace(str)
					if trimmed == "" {
						return errors.New("o nome do projeto não pode ser vazio")
					}
					if strings.ContainsAny(trimmed, "/\\:*?\"<>|") {
						return errors.New("o nome contém caracteres inválidos")
					}
					return nil
				}),

			huh.NewMultiSelect[string]().
				Title("Agentes de IA").
				Description("Selecione onde gerar as SKILLs:").
				Options(options...).
				Value(&selectedAgentStrings),

			huh.NewConfirm().
				Title("Confirmar cadastro e iniciar indexação?").
				Affirmative("Confirmar").
				Negative("Cancelar").
				Value(&confirmSetup),
		),
	).WithTheme(ui.HuhTheme()).WithKeyMap(ui.HuhKeyMap())

	if err := form.Run(); err != nil || !confirmSetup {
		return huh.ErrUserAborted
	}

	ctx.Name = strings.TrimSpace(projectName)

	var selectedAgents []AgentType
	for _, a := range selectedAgentStrings {
		selectedAgents = append(selectedAgents, AgentType(a))
	}
	if len(selectedAgents) == 0 {
		selectedAgents = []AgentType{AgentAntigravity}
	}

	var indexedProj *storage.Project
	err := ui.RunWithSpinner(fmt.Sprintf("Cadastrando e indexando '%s'...", ctx.Name), func() error {
		proj, err := projService.RegisterProject(ctx.Name, ctx.RootDir, ctx.DetectedLang)
		if err != nil {
			return fmt.Errorf("falha ao criar projeto: %w", err)
		}

		_ = SaveProjectConfig(ctx.RootDir, proj.ID, proj.Name, proj.Language)
		_ = SetupSkills(ctx.RootDir, proj.ID, proj.Name, selectedAgents)

		indexed, err := projService.ReindexProject(proj.ID)
		if err != nil {
			return fmt.Errorf("falha ao indexar projeto: %w", err)
		}
		indexedProj = indexed
		return nil
	})
	if err != nil {
		fmt.Println(ui.ErrorBox("Falha no Cadastro", err.Error()))
		return err
	}

	fmt.Println(ui.SuccessBox(fmt.Sprintf("Projeto '%s' cadastrado e indexado (%d arquivos, %d símbolos).",
		indexedProj.Name, indexedProj.FileCount, indexedProj.SymbolCount), ""))
	return nil
}
