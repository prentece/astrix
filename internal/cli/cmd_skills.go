package cli

import (
	"astrix/internal/cli/ui"
	"astrix/internal/service"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// RunSkills reconfigura interativamente as skills de IA para o projeto atual utilizando Huh?.
func RunSkills(projService *service.ProjectService) error {
	return PrintSkills(projService, false)
}

// PrintSkills executa o fluxo de configuração de SKILLs com controle de exibição interativa.
func PrintSkills(projService *service.ProjectService, isInteractive ...bool) error {
	currDir, _ := os.Getwd()
	ctx, err := DetectContext(currDir, projService.ProjectRepo())
	if err != nil {
		return err
	}

	if !ctx.IsRegistered || ctx.ProjectID == "" {
		return fmt.Errorf("diretório atual não é um projeto cadastrado no Astrix")
	}

	var options []huh.Option[string]
	for _, target := range GetAvailableAgents() {
		opt := huh.NewOption(fmt.Sprintf("%s (%s)", target.Name, target.FilePath), target.ID)
		if target.ID == "antigravity" {
			opt = opt.Selected(true)
		}
		options = append(options, opt)
	}

	var selectedAgentStrings []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Description("Selecione os editores/agentes para gerar as SKILLs:").
				Options(options...).
				Value(&selectedAgentStrings),
		),
	).WithTheme(ui.HuhTheme()).WithKeyMap(ui.HuhKeyMap()).WithShowHelp(true)

	if len(isInteractive) == 0 || !isInteractive[0] {
		ui.EnterAltScreen()
		defer ui.ExitAltScreen()
	}

	if err := form.Run(); err != nil {
		return err
	}

	var selectedAgents []AgentType
	for _, a := range selectedAgentStrings {
		selectedAgents = append(selectedAgents, AgentType(a))
	}
	if len(selectedAgents) == 0 {
		selectedAgents = []AgentType{AgentAntigravity}
	}

	_, _ = RemoveSkills(ctx.RootDir)
	if err := SetupSkills(ctx.RootDir, ctx.ProjectID, ctx.Name, selectedAgents); err != nil {
		return fmt.Errorf("falha ao gerar skills: %w", err)
	}

	if len(isInteractive) > 0 && isInteractive[0] {
		ui.ClearScreen()
		proj, _ := projService.GetByID(ctx.ProjectID)
		if proj != nil {
			statusDetail := fmt.Sprintf("%s • %d arquivos, %d símbolos", proj.Status, proj.FileCount, proj.SymbolCount)
			fmt.Println(ui.HeaderCard("ASTRIX", fmt.Sprintf("%s (%s)", proj.Name, proj.Language), statusDetail, proj.Path, proj.Status == "ready"))
			fmt.Println()
		}
	}

	status := lipgloss.NewStyle().Foreground(ui.ColorSuccess).Render("configurado")
	fmt.Println(ui.KeyValue("Projeto", ctx.Name))
	fmt.Println(ui.KeyValue("Status", status))
	var agentNames []string
	for _, a := range selectedAgents {
		agentNames = append(agentNames, string(a))
	}
	fmt.Println(ui.KeyValue("Skills de IA", strings.Join(agentNames, ", ")))
	return nil
}
