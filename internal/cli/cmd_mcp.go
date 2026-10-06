package cli

import (
	"astrix/internal/cli/ui"
	"astrix/internal/service"
	"astrix/pkg/watcher"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		return PrintMCPConfigWithArgs(args[1:])
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
	return PrintMCPConfigWithArgs(nil)
}

// PrintMCPConfigWithArgs exibe o snippet MCP respeitando as flags --absolute e --npx.
// Por padrão usa apenas "astrix" quando o binário está resolvível no PATH.
func PrintMCPConfigWithArgs(args []string) error {
	useAbsolute, useNpx := false, false
	for _, a := range args {
		switch a {
		case "--absolute", "-a":
			useAbsolute = true
		case "--npx":
			useNpx = true
		default:
			return fmt.Errorf("flag desconhecida '%s' (use --absolute ou --npx)", a)
		}
	}
	if useAbsolute && useNpx {
		return fmt.Errorf("as flags --absolute e --npx são mutuamente exclusivas")
	}

	var command string
	serveArgs := `["serve"]`
	onPath := false

	switch {
	case useNpx:
		command = "npx"
		serveArgs = `["-y", "@prentece/astrix", "serve"]`
	case useAbsolute:
		command = resolveExecutablePath()
	default:
		if isAstrixOnPath() {
			command = "astrix"
			onPath = true
		} else {
			command = resolveExecutablePath()
		}
	}

	configSnippet := fmt.Sprintf(`{
  "mcpServers": {
    "astrix": {
      "command": %q,
      "args": %s
    }
  }
}`, command, serveArgs)

	fmt.Println(ui.FormatJSONHighlight(configSnippet))
	if onPath {
		fmt.Println()
		fmt.Println("Se o seu cliente de IA não encontrar o comando, use: astrix config --absolute")
	}
	return nil
}

// resolveExecutablePath retorna o caminho absoluto do binário em execução.
func resolveExecutablePath() string {
	exePath, err := os.Executable()
	if err != nil {
		return "astrix"
	}
	return exePath
}

// isAstrixOnPath verifica se "astrix" no PATH aponta para o mesmo binário em execução.
func isAstrixOnPath() bool {
	lookup, err := exec.LookPath("astrix")
	if err != nil {
		return false
	}
	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	a, errA := filepath.EvalSymlinks(lookup)
	b, errB := filepath.EvalSymlinks(exePath)
	if errA != nil || errB != nil {
		return false
	}
	return a == b
}

// PrintMCPHelp exibe a ajuda dos comandos do MCP.
func PrintMCPHelp() {
	fmt.Println(ui.Banner("Comandos do Servidor MCP"))
	fmt.Println("  Uso: astrix mcp [comando]")
	fmt.Println()
	fmt.Println("  Comandos:")
	fmt.Println("    serve   Executa o servidor MCP via transporte STDIO (com file watcher automático)")
	fmt.Println("    config  Exibe a configuração JSON (flags: --absolute, --npx)")
	fmt.Println()
}

