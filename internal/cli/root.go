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
func Execute() error {
	args := os.Args[1:]

	// CLI 1: Fast-path para utilitários imediatos (sem abrir banco, migrações ou watcher)
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version", "-v":
			fmt.Printf("astrix v%s\n", Version)
			return nil
		case "help", "--help", "-h":
			PrintHelp()
			return nil
		}
	}

	dbPath, err := GetDatabasePath()
	if err != nil {
		log.Fatalf("[FATAL] Falha ao determinar diretório de banco de dados do Astrix: %v\n", err)
	}

	// Redireciona logs internos do backend para o arquivo astrix.log
	if logPath, err := GetLogPath(); err == nil {
		if logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			log.SetOutput(logFile)
			defer func() { _ = logFile.Close() }()
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
	defer func() { _ = database.Close() }()

	projectRepo := storage.NewProjectRepo(database)

	// CLI 4: Só reseta status pendente se nenhum servidor MCP estiver vivo
	if _, isAlive := ReadPID(); !isAlive {
		_ = projectRepo.ResetDanglingIndexingStatus()
	}

	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)
	engine.SetFileStateRepo(fileStateRepo)
	indexStore := storage.NewIndexStore(database)
	engine.SetIndexReplacer(indexStore)
	engine.SetIncrementalApplier(indexStore)

	projectService := service.NewProjectService(projectRepo, symbolRepo, engine)
	codeService := service.NewCodeService(projectRepo, symbolRepo, depRepo, dataModelRepo, engine)

	fileWatcher, err := watcher.NewFileWatcherService(projectRepo, fileStateRepo, engine)
	if err == nil {
		projectService.SetWatcher(fileWatcher)
	}

	handleErr := func(err error) error {
		if err != nil && !errors.Is(err, huh.ErrUserAborted) {
			fmt.Print(ui.ErrorBox(err.Error(), ""))
			return err
		}
		return nil
	}

	if len(args) == 0 {
		// Sem argumentos: comportamento contextual
		currDir, _ := os.Getwd()
		ctx, err := DetectContext(currDir, projectRepo)
		if err != nil {
			fmt.Printf("Erro ao detectar contexto: %v\n", err)
			return err
		}

		if ctx.IsRegistered {
			// Projeto cadastrado: exibe dashboard visual interativo
			return handleErr(RunDashboard(projectService, codeService, fileWatcher, ctx))
		}

		if ctx.IsProject && !ctx.IsRegistered {
			// Projeto válido mas não cadastrado: dispara wizard interativo
			existing, _ := projectRepo.ListAll()
			isFirstProject := len(existing) == 0

			wizardErr := RunWizard(projectService, ctx)
			if err := handleErr(wizardErr); err != nil {
				return err
			}

			// Boas-vindas: no primeiro projeto cadastrado, abre o painel já pronto para uso
			if wizardErr == nil && isFirstProject {
				if newCtx, err := DetectContext(currDir, projectRepo); err == nil && newCtx.IsRegistered {
					return handleErr(RunDashboard(projectService, codeService, fileWatcher, newCtx))
				}
			}
			return nil
		}

		// Fora de repositório de projeto: abre painel interativo global
		return handleErr(RunGlobalDashboard(projectService, codeService, fileWatcher))
	}

	command := args[0]
	switch command {
	case "ls", "list":
		return handleErr(RunList(projectService, args[1:]...))
	case "status":
		return handleErr(RunStatus(projectService, args[1:]...))
	case "serve", "server":
		if err := RunServe(projectService, codeService, fileWatcher); err != nil {
			fmt.Fprintf(os.Stderr, "Erro no servidor MCP: %v\n", err)
			return err
		}
		return nil
	case "watch":
		return handleErr(RunWatch(projectService, codeService, fileWatcher, args[1:]...))
	case "config":
		return handleErr(PrintMCPConfigWithArgs(args[1:]))
	case "mcp":
		if len(args) <= 1 || args[1] == "serve" {
			if err := RunMCPCommand(args[1:], projectService, codeService, fileWatcher); err != nil {
				fmt.Fprintf(os.Stderr, "Erro no servidor MCP: %v\n", err)
				return err
			}
			return nil
		}
		return handleErr(RunMCPCommand(args[1:], projectService, codeService, fileWatcher))
	case "index", "reindex", "rebuild":
		return handleErr(RunIndex(projectService))
	case "clean":
		return handleErr(RunClean(projectService))
	case "skills":
		return handleErr(RunSkills(projectService))
	case "version", "--version", "-v":
		fmt.Printf("astrix v%s\n", Version)
		return nil
	case "help", "--help", "-h":
		PrintHelp(projectRepo)
		return nil
	default:
		fmt.Print(ui.ErrorBox(fmt.Sprintf("Comando desconhecido: '%s'", command), ""))
		fmt.Println()
		PrintHelp(projectRepo)
		return fmt.Errorf("comando desconhecido: %s", command)
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
	fmt.Println("    watch          Inicia o monitoramento de arquivos em tempo real (auto-sync)")
	fmt.Println("    serve          Inicia o servidor MCP via transporte nativo STDIO")
	fmt.Println("    config         Exibe o JSON de configuração MCP (--absolute: caminho completo, --npx: via npx)")
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
