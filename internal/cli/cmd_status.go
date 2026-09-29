package cli

import (
	"astrix/internal/cli/ui"
	"astrix/internal/service"
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/lipgloss"
)

func RunStatus(projService *service.ProjectService) error {
	return PrintStatus(projService, false)
}

// PrintStatus renderiza os dados de status do sistema.
func PrintStatus(projService *service.ProjectService, _ ...bool) error {
	projects, err := projService.ListAll()
	if err != nil {
		return fmt.Errorf("falha ao consultar projetos: %w", err)
	}

	dbPath, _ := GetDatabasePath()
	logPath, _ := GetLogPath()

	fmt.Println(ui.KeyValue("Base de Dados", dbPath))
	fmt.Println(ui.KeyValue("Arquivo de Logs", logPath))
	fmt.Println(ui.KeyValue("Projetos Cadastrados", strconv.Itoa(len(projects))))

	// Checagem de processo via PID
	pid, isAlive := ReadPID()
	if isAlive {
		online := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorSuccess).Render(fmt.Sprintf("[ONLINE PID %d]", pid))
		fmt.Println(ui.KeyValue("Servidor MCP", online))
	} else {
		standby := lipgloss.NewStyle().Foreground(ui.ColorMuted).Render("[STANDBY]")
		fmt.Println(ui.KeyValue("Servidor MCP", standby))
	}

	// Estatísticas de contexto
	currDir, _ := os.Getwd()
	ctx, _ := DetectContext(currDir, projService.ProjectRepo())
	if ctx != nil && ctx.IsRegistered {
		fmt.Println(ui.KeyValue("Contexto Atual", fmt.Sprintf("%s (ID: %s)", ctx.Name, ctx.ProjectID)))
	}
	fmt.Println()

	return nil
}
