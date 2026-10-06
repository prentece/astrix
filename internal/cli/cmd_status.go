package cli

import (
	"astrix/internal/cli/ui"
	"astrix/internal/service"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/lipgloss"
)

// StatusOutput representa a estrutura da saída JSON do comando status.
type StatusOutput struct {
	DatabasePath   string          `json:"database_path"`
	LogPath        string          `json:"log_path"`
	ProjectsCount  int             `json:"projects_count"`
	MCPServer      MCPServerStatus `json:"mcp_server"`
	CurrentContext *ContextStatus  `json:"current_context,omitempty"`
}

// MCPServerStatus detalha o status do processo MCP em execução.
type MCPServerStatus struct {
	Online bool `json:"online"`
	PID    int  `json:"pid,omitempty"`
}

// ContextStatus descreve o contexto do projeto atual na raiz.
type ContextStatus struct {
	Registered bool   `json:"registered"`
	ProjectID  string `json:"project_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Path       string `json:"path,omitempty"`
}

// RunStatus executa o comando status.
func RunStatus(projService *service.ProjectService, args ...string) error {
	isJSON := false
	for _, a := range args {
		if a == "--json" {
			isJSON = true
			break
		}
	}
	return PrintStatus(projService, isJSON)
}

// PrintStatus renderiza os dados de status do sistema.
func PrintStatus(projService *service.ProjectService, isJSON ...bool) error {
	projects, err := projService.ListAll()
	if err != nil {
		return fmt.Errorf("falha ao consultar projetos: %w", err)
	}

	dbPath, _ := GetDatabasePath()
	logPath, _ := GetLogPath()
	pid, isAlive := ReadPID()

	currDir, _ := os.Getwd()
	ctx, _ := DetectContext(currDir, projService.ProjectRepo())

	if len(isJSON) > 0 && isJSON[0] {
		out := StatusOutput{
			DatabasePath:  dbPath,
			LogPath:       logPath,
			ProjectsCount: len(projects),
			MCPServer: MCPServerStatus{
				Online: isAlive,
				PID:    pid,
			},
		}
		if ctx != nil && ctx.IsRegistered {
			out.CurrentContext = &ContextStatus{
				Registered: true,
				ProjectID:  ctx.ProjectID,
				Name:       ctx.Name,
				Path:       ctx.RootDir,
			}
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("falha ao formatar JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println(ui.KeyValue("Base de Dados", dbPath))
	fmt.Println(ui.KeyValue("Arquivo de Logs", logPath))
	fmt.Println(ui.KeyValue("Projetos Cadastrados", strconv.Itoa(len(projects))))

	// Checagem de processo via PID
	if isAlive {
		online := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorSuccess).Render(fmt.Sprintf("[ONLINE PID %d]", pid))
		fmt.Println(ui.KeyValue("Servidor MCP", online))
	} else {
		standby := lipgloss.NewStyle().Foreground(ui.ColorMuted).Render("[STANDBY]")
		fmt.Println(ui.KeyValue("Servidor MCP", standby))
	}

	// Estatísticas de contexto
	if ctx != nil && ctx.IsRegistered {
		fmt.Println(ui.KeyValue("Contexto Atual", fmt.Sprintf("%s (ID: %s)", ctx.Name, ctx.ProjectID)))
	}
	fmt.Println()

	return nil
}
