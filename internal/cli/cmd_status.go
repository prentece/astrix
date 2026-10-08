package cli

import (
	"astrix/internal/cli/ui"
	"astrix/internal/service"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

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

// MCPServerStatus detalha o status dos processos MCP em execução.
type MCPServerStatus struct {
	Online         bool  `json:"online"`
	PID            int   `json:"pid,omitempty"`
	InstancesCount int   `json:"instances_count,omitempty"`
	LeaderPID      int   `json:"leader_pid,omitempty"`
	ReplicaPIDs    []int `json:"replica_pids,omitempty"`
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

	activePIDs, _ := ListActivePIDs()
	leaderPID, isLeaderAlive := ReadPID()

	allPIDsMap := make(map[int]bool)
	for _, p := range activePIDs {
		allPIDsMap[p] = true
	}
	if isLeaderAlive && leaderPID > 0 && !allPIDsMap[leaderPID] {
		activePIDs = append(activePIDs, leaderPID)
		allPIDsMap[leaderPID] = true
	}

	var replicaPIDs []int
	for _, p := range activePIDs {
		if p != leaderPID {
			replicaPIDs = append(replicaPIDs, p)
		}
	}
	isOnline := isLeaderAlive || len(activePIDs) > 0

	currDir, _ := os.Getwd()
	ctx, _ := DetectContext(currDir, projService.ProjectRepo())

	if len(isJSON) > 0 && isJSON[0] {
		out := StatusOutput{
			DatabasePath:  dbPath,
			LogPath:       logPath,
			ProjectsCount: len(projects),
			MCPServer: MCPServerStatus{
				Online:         isOnline,
				PID:            leaderPID,
				InstancesCount: len(activePIDs),
				LeaderPID:      leaderPID,
				ReplicaPIDs:    replicaPIDs,
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

	// Checagem de processo via PID e Líder
	if isOnline {
		var statusText string
		if len(activePIDs) <= 1 {
			targetPID := leaderPID
			if targetPID == 0 && len(activePIDs) > 0 {
				targetPID = activePIDs[0]
			}
			statusText = fmt.Sprintf("[ONLINE PID %d (Líder / Watcher)]", targetPID)
		} else {
			var replicaStr []string
			for _, r := range replicaPIDs {
				replicaStr = append(replicaStr, fmt.Sprintf("PID %d", r))
			}
			statusText = fmt.Sprintf("[ONLINE %d instâncias — Líder: PID %d (Watcher), Réplicas: %s]",
				len(activePIDs), leaderPID, strings.Join(replicaStr, ", "))
		}
		online := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorSuccess).Render(statusText)
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
