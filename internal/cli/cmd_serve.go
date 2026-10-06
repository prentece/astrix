package cli

import (
	"astrix/internal/mcp"
	"astrix/internal/service"
	"astrix/pkg/watcher"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// RunServe inicia o servidor MCP via transporte STDIO (sem portas TCP) e ativa o FileWatcher em segundo plano.
func RunServe(
	projectService *service.ProjectService,
	codeService *service.CodeService,
	fileWatcher *watcher.FileWatcherService,
) error {
	if existingPid, isAlive := ReadPID(); isAlive && existingPid != os.Getpid() {
		return fmt.Errorf("outro servidor MCP já está ativo com PID %d", existingPid)
	}
	_ = WritePID()
	defer RemovePID()

	// Redireciona logs exclusivamente para o arquivo ~/.astrix/astrix.log
	// CRÍTICO: stdout e stderr não devem ser poluídos com logs de depuração durante STDIO
	logPath, err := GetLogPath()
	if err == nil {
		if logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			defer logFile.Close()
			log.SetOutput(logFile)
		} else {
			log.SetOutput(io.Discard)
		}
	} else {
		log.SetOutput(io.Discard)
	}

	// Inicia o monitor de arquivos em background
	if fileWatcher != nil {
		if err := fileWatcher.Start(); err != nil {
			log.Printf("[WATCHER WARN] Falha ao iniciar monitor de arquivos: %v\n", err)
		} else {
			defer fileWatcher.Stop()
			log.Println("[WATCHER] Monitor de arquivos fsnotify ativo em background.")
		}
	}

	mcpServer := mcp.NewServer(projectService, codeService, Version)
	log.Println("[MCP] Servidor Astrix MCP inicializado via transporte STDIO (port-free).")

	if err := mcpServer.ServeStdio(); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || strings.Contains(err.Error(), "context canceled") {
			return nil
		}
		return err
	}
	return nil
}
