package cli

import (
	"astrix/internal/mcp"
	"astrix/internal/service"
	"astrix/pkg/watcher"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// RunServe inicia o servidor MCP via transporte STDIO (sem portas TCP) e ativa o FileWatcher no processo líder.
// Suporta múltiplas instâncias concorrentes (ex: múltiplas telas do Antigravity/Cursor/VSCode):
// - Todas as instâncias atendem chamadas MCP em sua própria sessão STDIO.
// - Apenas UMA instância (Líder) executa o FileWatcher fsnotify, eleita via lock de arquivo no SO.
// - Instâncias secundárias operam em modo réplica e assumem o FileWatcher automaticamente caso o líder encerre (failover transparente).
func RunServe(
	projectService *service.ProjectService,
	codeService *service.CodeService,
	fileWatcher *watcher.FileWatcherService,
) error {
	// Registra presença do PID desta instância para contagem de processos ativos
	currentPID := os.Getpid()
	_ = RegisterInstancePID(currentPID)
	defer UnregisterInstancePID(currentPID)

	// Redireciona logs exclusivamente para o arquivo ~/.astrix/astrix.log
	// CRÍTICO: stdout e stderr não devem ser poluídos com logs de depuração durante STDIO
	logPath, err := GetLogPath()
	if err == nil {
		if logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			defer func() { _ = logFile.Close() }()
			log.SetOutput(logFile)
		} else {
			log.SetOutput(io.Discard)
		}
	} else {
		log.SetOutput(io.Discard)
	}

	// Garante parada limpa do fileWatcher no encerramento deste processo (se estiver ativo)
	if fileWatcher != nil {
		defer fileWatcher.Stop()
	}

	// Coordenação de Líder para o FileWatcher
	lockPath, err := GetWatcherLockPath()
	if err != nil {
		log.Printf("[MCP WARN] Falha ao determinar caminho do lock do watcher: %v\n", err)
	}

	watcherLock := NewWatcherLock(lockPath)
	defer func() {
		watcherLock.Release()
		RemovePID()
	}()

	serveCtx, cancelServeCtx := context.WithCancel(context.Background())
	defer cancelServeCtx()

	isLeader := false
	if lockPath != "" {
		acquired, lockErr := watcherLock.TryAcquire()
		if lockErr != nil {
			log.Printf("[MCP WARN] Erro ao tentar adquirir lock de líder do watcher: %v\n", lockErr)
		} else if acquired {
			isLeader = true
		}
	}

	if isLeader {
		_ = WriteLeaderPID(currentPID)
		log.Printf("[MCP LEADER] Instância (PID %d) assumiu como Líder. Iniciando FileWatcher em background.\n", currentPID)
		if fileWatcher != nil {
			if err := fileWatcher.Start(); err != nil {
				log.Printf("[WATCHER WARN] Falha ao iniciar monitor de arquivos no líder: %v\n", err)
			} else {
				log.Println("[WATCHER LEADER] Monitor de arquivos fsnotify ativo.")
			}
		}
	} else {
		log.Printf("[MCP REPLICA] Instância (PID %d) ativa em modo réplica. FileWatcher gerenciado pelo líder.\n", currentPID)
		// Goroutine de standby: se o líder atual encerrar, esta réplica assume o FileWatcher
		if fileWatcher != nil && lockPath != "" {
			go runWatcherFailoverLoop(serveCtx, watcherLock, fileWatcher, currentPID)
		}
	}

	mcpServer := mcp.NewServer(projectService, codeService, Version)
	log.Printf("[MCP] Servidor Astrix MCP inicializado via transporte STDIO (PID %d).\n", currentPID)

	if err := mcpServer.ServeStdio(); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || strings.Contains(err.Error(), "context canceled") {
			return nil
		}
		return err
	}
	return nil
}

// runWatcherFailoverLoop monitora a liberação do lock de líder a cada 2 segundos.
// Quando o líder anterior encerra, a primeira réplica a detectar adquire o lock e assume o FileWatcher.
func runWatcherFailoverLoop(
	ctx context.Context,
	lock *WatcherLock,
	fileWatcher *watcher.FileWatcherService,
	pid int,
) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			acquired, err := lock.TryAcquire()
			if err != nil {
				log.Printf("[WATCHER FAILOVER] Erro ao tentar obter lock: %v\n", err)
				continue
			}
			if acquired {
				_ = WriteLeaderPID(pid)
				log.Printf("[WATCHER LEADER] Instância (PID %d) promovida a Líder após encerramento do anterior. Ativando FileWatcher.\n", pid)
				if err := fileWatcher.Start(); err != nil {
					log.Printf("[WATCHER WARN] Falha ao iniciar monitor de arquivos no failover: %v\n", err)
				} else {
					log.Println("[WATCHER LEADER] Monitor de arquivos fsnotify ativo com sucesso após failover.")
				}
				return
			}
		}
	}
}
