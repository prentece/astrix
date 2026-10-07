package cli

import (
	"astrix/internal/cli/ui"
	"astrix/internal/service"
	"astrix/pkg/watcher"
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// RunWatch inicia o monitoramento em tempo real do Astrix no terminal.
// Se o servidor MCP já estiver ativo em background, conecta-se no modo monitor passivo (streaming de logs).
// Caso contrário, assume o modo ativo (standalone) rodando o FileWatcherService em primeiro plano.
func RunWatch(
	projService *service.ProjectService,
	codeService *service.CodeService,
	fileWatcher *watcher.FileWatcherService,
	args ...string,
) error {
	return RunWatchWithSignal(projService, codeService, fileWatcher, nil, args...)
}

// RunWatchWithSignal permite injetar um canal de sinal customizado para controle de ciclo de vida e testes.
func RunWatchWithSignal(
	projService *service.ProjectService,
	_ *service.CodeService,
	fileWatcher *watcher.FileWatcherService,
	sigCh <-chan os.Signal,
	_ ...string,
) error {
	currDir, _ := os.Getwd()
	ctx, _ := DetectContext(currDir, projService.ProjectRepo())

	mcpPID, isMCPOnline := ReadPID()

	if sigCh == nil {
		internalSigCh := make(chan os.Signal, 1)
		signal.Notify(internalSigCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(internalSigCh)
		sigCh = internalSigCh
	}

	if isMCPOnline {
		return runPassiveWatch(ctx, mcpPID, sigCh)
	}

	return runActiveWatch(projService, fileWatcher, ctx, sigCh)
}

// runPassiveWatch monitora o servidor MCP ativo através do fluxo do arquivo de log em tempo real.
func runPassiveWatch(_ *ProjectContext, mcpPID int, sigCh <-chan os.Signal) error {
	printWatchHeader(true, mcpPID)

	logPath, err := GetLogPath()
	if err != nil {
		return fmt.Errorf("falha ao localizar arquivo de log: %w", err)
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_RDONLY, 0o644)
	if err != nil {
		return fmt.Errorf("falha ao abrir arquivo de log: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Posiciona a leitura a partir do momento atual
	_, _ = file.Seek(0, io.SeekEnd)
	reader := bufio.NewReader(file)

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sigCh:
			fmt.Println()
			fmt.Println(ui.SuccessBox("Monitor passivo encerrado", "Servidor MCP permanece ativo em background."))
			return nil

		case <-ticker.C:
			for {
				line, err := reader.ReadString('\n')
				if len(line) > 0 {
					formatted := FormatLogLineForTerminal(strings.TrimRight(line, "\r\n"))
					if formatted != "" {
						fmt.Println(formatted)
					}
				}
				if err != nil {
					break
				}
			}
		}
	}
}

// runActiveWatch inicializa o FileWatcherService em primeiro plano no terminal.
func runActiveWatch(
	_ *service.ProjectService,
	fileWatcher *watcher.FileWatcherService,
	_ *ProjectContext,
	sigCh <-chan os.Signal,
) error {
	if fileWatcher == nil {
		return fmt.Errorf("serviço de monitoramento de arquivos não inicializado")
	}

	printWatchHeader(false, 0)

	// Redireciona os logs do engine/watcher para formatador ao vivo no stdout
	logPath, _ := GetLogPath()
	var logFile *os.File
	if logPath != "" {
		logFile, _ = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if logFile != nil {
			defer func() { _ = logFile.Close() }()
		}
	}

	bridge := &terminalLogBridge{
		logFile: logFile,
	}
	log.SetOutput(bridge)
	defer func() {
		if logFile != nil {
			log.SetOutput(logFile)
		} else {
			log.SetOutput(io.Discard)
		}
	}()

	if err := fileWatcher.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar monitor de arquivos: %w", err)
	}
	defer fileWatcher.Stop()

	<-sigCh
	fmt.Println()
	fmt.Println(ui.SuccessBox("Monitor ativo encerrado", "Recursos e watches liberados com sucesso."))
	return nil
}

func printWatchHeader(isPassive bool, mcpPID int) {
	var modeBadge, mcpBadge string
	if isPassive {
		modeBadge = lipgloss.NewStyle().Bold(true).Foreground(ui.ColorSuccess).Render("MODO MONITOR PASSIVO")
		mcpBadge = lipgloss.NewStyle().Bold(true).Foreground(ui.ColorSuccess).Render(fmt.Sprintf("[ONLINE PID %d]", mcpPID))
	} else {
		modeBadge = lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary).Render("MODO ATIVO (STANDALONE)")
		mcpBadge = lipgloss.NewStyle().Foreground(ui.ColorMuted).Render("[STANDBY]")
	}

	fmt.Printf("%s %s\n", ui.SubtitleStyle.Render("Modo:        "), modeBadge)
	fmt.Printf("%s %s\n", ui.SubtitleStyle.Render("Servidor MCP:"), mcpBadge)
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Foreground(ui.ColorMuted).Render("Pressione Ctrl+C para sair."))
	fmt.Println()
}

// terminalLogBridge intercepta logs internos para gravar em arquivo e exibir formatado no terminal.
type terminalLogBridge struct {
	logFile *os.File
}

func (b *terminalLogBridge) Write(p []byte) (n int, err error) {
	if b.logFile != nil {
		_, _ = b.logFile.Write(p)
	}
	line := strings.TrimRight(string(p), "\r\n")
	formatted := FormatLogLineForTerminal(line)
	if formatted != "" {
		fmt.Println(formatted)
	}
	return len(p), nil
}

// FormatLogLineForTerminal formata mensagens de log do Astrix com estilo elegante para o terminal.
func FormatLogLineForTerminal(rawLine string) string {
	rawLine = strings.TrimSpace(rawLine)
	if rawLine == "" {
		return ""
	}

	now := time.Now().Format("15:04:05")
	timePrefix := lipgloss.NewStyle().Foreground(ui.ColorDarkMuted).Render("[" + now + "]")

	switch {
	case strings.Contains(rawLine, "[WATCHER AUTO-SYNC]"):
		idx := strings.Index(rawLine, "[WATCHER AUTO-SYNC]")
		msg := strings.TrimSpace(rawLine[idx+len("[WATCHER AUTO-SYNC]"):])
		badge := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary).Render("[CAPTURADO]")
		return fmt.Sprintf("%s %s %s", timePrefix, badge, msg)

	case strings.Contains(rawLine, "[DELTA INDEXER SUCCESS]"):
		idx := strings.Index(rawLine, "[DELTA INDEXER SUCCESS]")
		msg := strings.TrimSpace(rawLine[idx+len("[DELTA INDEXER SUCCESS]"):])
		badge := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorSuccess).Render("[SINCRONIZADO]")
		return fmt.Sprintf("%s %s %s", timePrefix, badge, msg)

	case strings.Contains(rawLine, "[DELTA INDEXER WARN]"):
		idx := strings.Index(rawLine, "[DELTA INDEXER WARN]")
		msg := strings.TrimSpace(rawLine[idx+len("[DELTA INDEXER WARN]"):])
		badge := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWarning).Render("[AVISO]")
		return fmt.Sprintf("%s %s %s", timePrefix, badge, msg)

	case strings.Contains(rawLine, "[INDEXER SUCCESS]"):
		idx := strings.Index(rawLine, "[INDEXER SUCCESS]")
		msg := strings.TrimSpace(rawLine[idx+len("[INDEXER SUCCESS]"):])
		badge := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorSuccess).Render("[INDEXADO]")
		return fmt.Sprintf("%s %s %s", timePrefix, badge, msg)

	case strings.Contains(rawLine, "[WATCHER"):
		idx := strings.Index(rawLine, "[WATCHER")
		watcherContent := strings.TrimSpace(rawLine[idx:])
		return fmt.Sprintf("%s %s", timePrefix, lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(watcherContent))

	default:
		return ""
	}
}
