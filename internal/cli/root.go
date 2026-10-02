package cli

import (
	"astrix/internal/cli/ui"
	"astrix/internal/service"
	"astrix/pkg/indexer"
	_ "astrix/pkg/indexer/languages"
	"astrix/pkg/storage"
	"astrix/pkg/watcher"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Execute é o ponto de entrada principal da CLI do Astrix.
func Execute() {
	args := os.Args[1:]

	dbPath, err := GetDatabasePath()
	if err != nil {
		log.Fatalf("[FATAL] Falha ao determinar diretório de banco de dados do Astrix: %v\n", err)
	}

	// Redireciona logs internos do backend para o arquivo astrix.log
	if logPath, err := GetLogPath(); err == nil {
		if logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			log.SetOutput(logFile)
			defer logFile.Close()
		} else {
			log.SetOutput(io.Discard)
		}
	} else {
		log.SetOutput(io.Discard)
	}

	// Inicializa infraestrutura básica
	database, err := storage.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("[FATAL] Falha ao inicializar banco de dados: %v\n", err)
	}
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	_ = projectRepo.ResetDanglingIndexingStatus()
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)
	engine.SetFileStateRepo(fileStateRepo)

	projectService := service.NewProjectService(projectRepo, symbolRepo, engine)
	codeService := service.NewCodeService(projectRepo, symbolRepo, depRepo, dataModelRepo, engine)

	fileWatcher, err := watcher.NewFileWatcherService(projectRepo, fileStateRepo, engine)
	if err == nil {
		projectService.SetWatcher(fileWatcher)
	}

	printErr := func(err error) {
		if err != nil && !errors.Is(err, huh.ErrUserAborted) {
			fmt.Print(ui.ErrorBox(err.Error(), ""))
		}
	}

	if len(args) == 0 {
		// Sem argumentos: comportamento contextual
		currDir, _ := os.Getwd()
		ctx, err := DetectContext(currDir, projectRepo)
		if err != nil {
			fmt.Printf("Erro ao detectar contexto: %v\n", err)
			return
		}

		if ctx.IsRegistered {
			// Projeto cadastrado: exibe dashboard visual interativo
			printErr(RunDashboard(projectService, codeService, fileWatcher, ctx))
			return
		}

		if ctx.IsProject && !ctx.IsRegistered {
			// Projeto válido mas não cadastrado: dispara wizard interativo
			printErr(RunWizard(projectService, ctx))
			return
		}

		// Fora de repositório de projeto: abre painel interativo global
		printErr(RunGlobalDashboard(projectService, codeService, fileWatcher))
		return
	}

	command := args[0]
	switch command {
	case "ls", "list":
		printErr(RunList(projectService))
	case "status":
		printErr(RunStatus(projectService))
	case "serve", "server":
		printErr(RunServe(projectService, codeService, fileWatcher))
	case "config":
		printErr(PrintMCPConfig(true))
	case "mcp":
		printErr(RunMCPCommand(args[1:], projectService, codeService, fileWatcher))
	case "index", "reindex", "rebuild":
		printErr(RunIndex(projectService))
	case "clean":
		printErr(RunClean(projectService))
	case "skills":
		printErr(RunSkills(projectService))
	case "version", "--version", "-v":
		fmt.Printf("astrix v%s\n", Version)
	case "help", "--help", "-h":
		PrintHelp(projectRepo)
	default:
		fmt.Print(ui.ErrorBox(fmt.Sprintf("Comando desconhecido: '%s'", command), ""))
		fmt.Println()
		PrintHelp(projectRepo)
	}
}

// PrintHelp exibe a documentação estilizada de uso da CLI.
func PrintHelp(projectRepo ...storage.ProjectRepository) {
	currDir, _ := os.Getwd()
	ctx, _ := DetectContext(currDir, projectRepo...)

	fmt.Println(ui.Banner("Interface de Linha de Comando"))

	if ctx != nil && ctx.IsRegistered {
		fmt.Println("  " + lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary).Render(fmt.Sprintf("Contexto Atual: %s (ID: %s)", ctx.Name, ctx.ProjectID)))
		fmt.Println()
	}

	cmdTitle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary)

	fmt.Println("  Uso: astrix [comando]")
	fmt.Println()

	fmt.Println("  " + cmdTitle.Render("Comandos Globais:"))
	fmt.Println("    (sem comando)  Abre o menu interativo com todas as ações disponíveis")
	fmt.Println("    serve          Inicia o servidor MCP via transporte nativo STDIO")
	fmt.Println("    config         Exibe o JSON de configuração MCP para editores/IA")
	fmt.Println("    ls             Lista todos os projetos cadastrados no banco")
	fmt.Println("    status         Exibe o status dos serviços em background e projetos")
	fmt.Println()

	fmt.Println("  " + cmdTitle.Render("Comandos do Projeto (executados na raiz do repositório):"))
	fmt.Println("    index          Força a reindexação sintática do repositório atual")
	fmt.Println("    skills         Reconfigura as SKILLs para os agentes de IA (.agents, .cursor, .claude)")
	fmt.Println("    clean          Remove o projeto com confirmação de segurança")
	fmt.Println()

	fmt.Println("  " + cmdTitle.Render("Opções:"))
	fmt.Println("    -v, --version  Exibe a versão instalada")
	fmt.Println("    -h, --help     Exibe esta mensagem de ajuda")
	fmt.Println()
}
