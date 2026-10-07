package cli

import (
	"astrix/internal/cli/ui"
	"astrix/internal/service"
	"astrix/pkg/watcher"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// RunDashboard exibe as informações do projeto e apresenta um menu interativo de ações.
func RunDashboard(
	projService *service.ProjectService,
	codeService *service.CodeService,
	fileWatcher *watcher.FileWatcherService,
	ctx *ProjectContext,
) error {
	ui.EnterAltScreen()
	defer ui.ExitAltScreen()

	for {
		ui.ClearScreen()

		proj, err := projService.GetByID(ctx.ProjectID)
		if err != nil {
			return fmt.Errorf("falha ao obter dados do projeto: %w", err)
		}

		statusDetail := fmt.Sprintf("%s • %d arquivos, %d símbolos", proj.Status, proj.FileCount, proj.SymbolCount)
		fmt.Println(ui.HeaderCard("ASTRIX", fmt.Sprintf("%s (%s)", proj.Name, proj.Language), statusDetail, proj.Path, proj.Status == "ready"))
		fmt.Println()

		// Menu interativo de ações
		var selectedAction string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Ações do Projeto").
					Options(
						huh.NewOption("Monitorar alterações em tempo real (watch)", "watch"),
						huh.NewOption("Reindexar projeto (index)", "index"),
						huh.NewOption("Configurar skills de IA (skills)", "skills"),
						huh.NewOption("Ver configuração MCP (config)", "config"),
						huh.NewOption("Listar projetos cadastrados (ls)", "ls"),
						huh.NewOption("Status geral do sistema (status)", "status"),
						huh.NewOption("Remover projeto (clean)", "clean"),
						huh.NewOption("Sair (exit)", "exit"),
					).
					Value(&selectedAction),
			),
		).WithTheme(ui.HuhTheme()).WithKeyMap(ui.HuhKeyMap()).WithShowHelp(true)

		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				ui.ClearScreen()
				return nil
			}
			return err
		}

		if selectedAction == "exit" {
			ui.ClearScreen()
			return nil
		}

		// Limpa a tela para executar a ação selecionada com o cabeçalho no topo
		ui.ClearScreen()
		fmt.Println(ui.HeaderCard("ASTRIX", fmt.Sprintf("%s (%s)", proj.Name, proj.Language), statusDetail, proj.Path, proj.Status == "ready"))
		fmt.Println()

		var actionErr error
		switch selectedAction {
		case "watch":
			actionErr = RunWatch(projService, codeService, fileWatcher)
		case "index":
			actionErr = PrintIndex(projService, false)
		case "skills":
			actionErr = PrintSkills(projService, true)
			if errors.Is(actionErr, huh.ErrUserAborted) {
				continue
			}
		case "config":
			actionErr = PrintMCPConfig(false)
		case "ls":
			actionErr = PrintList(projService, false)
		case "status":
			actionErr = PrintStatus(projService, false)
		case "clean":
			actionErr = PrintClean(projService, true)
			if errors.Is(actionErr, huh.ErrUserAborted) {
				continue
			}
			if actionErr == nil {
				// Se o projeto foi removido com sucesso, exibe o resultado e encerra o painel
				waitForReturn()
				ui.ClearScreen()
				return nil
			}
		}

		if actionErr != nil && !errors.Is(actionErr, huh.ErrUserAborted) {
			fmt.Print(ui.ErrorBox(actionErr.Error(), ""))
		}

		if actionErr == nil {
			waitForReturn()
		}
	}
}

// RunGlobalDashboard apresenta o menu interativo global quando a CLI é executada fora de um contexto de projeto.
func RunGlobalDashboard(
	projService *service.ProjectService,
	codeService *service.CodeService,
	fileWatcher *watcher.FileWatcherService,
) error {
	ui.EnterAltScreen()
	defer ui.ExitAltScreen()

	for {
		ui.ClearScreen()

		currDir, _ := os.Getwd()
		fmt.Println(ui.HeaderCard("ASTRIX", "Painel Global", "Transporte STDIO sob demanda", currDir, true))
		fmt.Println()

		var selectedAction string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Menu Principal").
					Options(
						huh.NewOption("Monitorar alterações em tempo real (watch)", "watch"),
						huh.NewOption("Listar projetos cadastrados (ls)", "ls"),
						huh.NewOption("Status geral do sistema (status)", "status"),
						huh.NewOption("Ver configuração MCP (config)", "config"),
						huh.NewOption("Ajuda e documentação de comandos (help)", "help"),
						huh.NewOption("Sair (exit)", "exit"),
					).
					Value(&selectedAction),
			),
		).WithTheme(ui.HuhTheme()).WithKeyMap(ui.HuhKeyMap()).WithShowHelp(true)

		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				ui.ClearScreen()
				return nil
			}
			return err
		}

		if selectedAction == "exit" {
			ui.ClearScreen()
			return nil
		}

		ui.ClearScreen()
		fmt.Println(ui.HeaderCard("ASTRIX", "Painel Global", "Transporte STDIO sob demanda", currDir, true))
		fmt.Println()

		var actionErr error
		switch selectedAction {
		case "watch":
			actionErr = RunWatch(projService, codeService, fileWatcher)
		case "ls":
			actionErr = PrintList(projService, false)
		case "status":
			actionErr = PrintStatus(projService, false)
		case "config":
			actionErr = PrintMCPConfig(false)
		case "help":
			PrintHelp(projService.ProjectRepo())
		}

		if actionErr != nil {
			fmt.Print(ui.ErrorBox(actionErr.Error(), ""))
		}

		waitForReturn()
	}
}

func waitForReturn() {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary)
	descStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	sepStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkMuted)

	fmt.Println()
	fmt.Printf("%s %s  %s  %s %s\n", keyStyle.Render("enter"), descStyle.Render("voltar"), sepStyle.Render("•"), keyStyle.Render("esc"), descStyle.Render("voltar"))

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()
		var buf [1]byte
		_, _ = os.Stdin.Read(buf[:])
		return
	}
	var buf [1]byte
	_, _ = os.Stdin.Read(buf[:])
}
