package cli

import (
	"astrix/internal/cli/ui"
	"astrix/internal/service"
	"astrix/pkg/watcher"
	"fmt"
	"os"
)

// RunMCPCommand processa os subcomandos de `astrix mcp [serve|config]`.
// Quando chamado sem subcomandos, inicia diretamente o servidor MCP via STDIO.
func RunMCPCommand(
	args []string,
	projectService *service.ProjectService,
	codeService *service.CodeService,
	fileWatcher *watcher.FileWatcherService,
) error {
	subCmd := "serve"
	if len(args) > 0 {
		subCmd = args[0]
	}

	switch subCmd {
	case "serve":
		return RunServe(projectService, codeService, fileWatcher)
	case "config":
		return PrintMCPConfig(true)
	case "help", "--help", "-h":
		PrintMCPHelp()
		return nil
	default:
		fmt.Println(ui.ErrorBox("Subcomando Inválido", fmt.Sprintf("Subcomando 'astrix mcp %s' desconhecido.", subCmd)))
		PrintMCPHelp()
		return nil
	}
}

// PrintMCPConfig exibe o snippet de configuração pronto para cadastro nos clientes de IA.
func PrintMCPConfig(_ ...bool) error {
	exePath, err := os.Executable()
	if err != nil {
		exePath = "astrix"
	}

	configSnippet := fmt.Sprintf(`{
  "mcpServers": {
    "astrix": {
      "command": %q,
      "args": ["serve"]
    }
  }
}`, exePath)

	fmt.Println(ui.FormatJSONHighlight(configSnippet))
	return nil
}

// PrintMCPHelp exibe a ajuda dos comandos do MCP.
func PrintMCPHelp() {
	fmt.Println(ui.Banner("Comandos do Servidor MCP"))
	fmt.Println("  Uso: astrix mcp [comando]")
	fmt.Println()
	fmt.Println("  Comandos:")
	fmt.Println("    serve   Executa o servidor MCP via transporte STDIO (com file watcher automático)")
	fmt.Println("    config  Exibe a configuração JSON para colar no editor/IA")
	fmt.Println()
}

