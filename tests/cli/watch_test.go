package cli_test

import (
	"astrix/internal/cli"
	"astrix/internal/service"
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"astrix/pkg/watcher"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	_ "astrix/pkg/indexer/languages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestWatchEnv(t *testing.T) (*service.ProjectService, *service.CodeService, *watcher.FileWatcherService, *storage.DB, string) {
	t.Helper()
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := storage.NewDatabase(dbPath)
	require.NoError(t, err)

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)
	fileStateRepo := storage.NewSQLFileStateRepository(database.Conn())

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)
	engine.SetFileStateRepo(fileStateRepo)
	indexStore := storage.NewIndexStore(database)
	engine.SetIndexReplacer(indexStore)
	engine.SetIncrementalApplier(indexStore)

	projService := service.NewProjectService(projectRepo, symbolRepo, engine)
	codeService := service.NewCodeService(projectRepo, symbolRepo, depRepo, dataModelRepo, engine)

	watcherService, err := watcher.NewFileWatcherService(projectRepo, fileStateRepo, engine)
	require.NoError(t, err)
	watcherService.SetDebounceDuration(50 * time.Millisecond)

	return projService, codeService, watcherService, database, tmpDir
}

func TestCLI_FormatLogLineForTerminal(t *testing.T) {
	// 1. Linha vazia ou em branco
	assert.Empty(t, cli.FormatLogLineForTerminal(""))
	assert.Empty(t, cli.FormatLogLineForTerminal("   \n\t"))

	// 2. Linha irrelevante / descarte
	assert.Empty(t, cli.FormatLogLineForTerminal("2026/10/06 random debug log line"))

	// 3. [WATCHER AUTO-SYNC] -> [CAPTURADO] (sem ícone)
	syncLog := "2026/10/06 12:00:00 [WATCHER AUTO-SYNC] [astrix] arquivo main.go modificado"
	resSync := cli.FormatLogLineForTerminal(syncLog)
	assert.Contains(t, resSync, "[CAPTURADO]")
	assert.NotContains(t, resSync, "⚡")
	assert.Contains(t, resSync, "[astrix] arquivo main.go modificado")

	// 4. [DELTA INDEXER SUCCESS] -> [SINCRONIZADO] (sem ícone)
	deltaLog := "2026/10/06 12:00:00 [DELTA INDEXER SUCCESS] [astrix] 3 símbolos sincronizados em 12ms"
	resDelta := cli.FormatLogLineForTerminal(deltaLog)
	assert.Contains(t, resDelta, "[SINCRONIZADO]")
	assert.NotContains(t, resDelta, "✓")
	assert.Contains(t, resDelta, "3 símbolos sincronizados em 12ms")

	// 5. [DELTA INDEXER WARN] -> [AVISO] (sem ícone)
	warnLog := "2026/10/06 12:00:00 [DELTA INDEXER WARN] falha ao parsear bloco incremental"
	resWarn := cli.FormatLogLineForTerminal(warnLog)
	assert.Contains(t, resWarn, "[AVISO]")
	assert.NotContains(t, resWarn, "⚠")
	assert.Contains(t, resWarn, "falha ao parsear bloco incremental")

	// 6. [WATCHER WARN] -> [WATCHER WARN] (unificado com demais watchers)
	watcherWarn := "2026/10/06 12:00:00 [WATCHER WARN] buffer cheio"
	resWatcherWarn := cli.FormatLogLineForTerminal(watcherWarn)
	assert.Contains(t, resWatcherWarn, "[WATCHER WARN]")
	assert.Contains(t, resWatcherWarn, "buffer cheio")

	// 7. [INDEXER SUCCESS] -> [INDEXADO] (sem ícone)
	indexLog := "2026/10/06 12:00:00 [INDEXER SUCCESS] 42 símbolos catalogados"
	resIndex := cli.FormatLogLineForTerminal(indexLog)
	assert.Contains(t, resIndex, "[INDEXADO]")
	assert.NotContains(t, resIndex, "★")
	assert.Contains(t, resIndex, "42 símbolos catalogados")

	// 8. [WATCHER INIT] -> [WATCHER INIT] (unificado)
	initLog := "2026/10/06 12:00:00 [WATCHER INIT] monitorando 1 projeto"
	resInit := cli.FormatLogLineForTerminal(initLog)
	assert.Contains(t, resInit, "[WATCHER INIT]")
}

func TestCLI_RunWatch_ActiveMode_NilWatcher(t *testing.T) {
	projService, codeService, _, db, _ := setupTestWatchEnv(t)
	defer db.Close()

	sigCh := make(chan os.Signal, 1)
	err := cli.RunWatchWithSignal(projService, codeService, nil, sigCh)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "serviço de monitoramento de arquivos não inicializado")
}

func TestCLI_RunWatch_ActiveMode_GracefulExit(t *testing.T) {
	projService, codeService, watcherService, db, _ := setupTestWatchEnv(t)
	defer db.Close()

	sigCh := make(chan os.Signal, 1)

	// Dispara término gracioso após iniciar
	go func() {
		time.Sleep(100 * time.Millisecond)
		sigCh <- syscall.SIGINT
	}()

	err := cli.RunWatchWithSignal(projService, codeService, watcherService, sigCh)
	assert.NoError(t, err)
}

func TestCLI_RunWatch_PassiveMode_GracefulExit(t *testing.T) {
	projService, codeService, watcherService, db, tmpDir := setupTestWatchEnv(t)
	defer db.Close()

	t.Setenv("ASTRIX_HOME", tmpDir)

	// Simula servidor MCP online gravando PID
	err := cli.WritePID()
	require.NoError(t, err)
	defer cli.RemovePID()

	// Cria e preenche arquivo de log para leitura
	logPath, err := cli.GetLogPath()
	require.NoError(t, err)
	err = os.WriteFile(logPath, []byte("2026/10/06 [WATCHER AUTO-SYNC] inicializado\n"), 0o644)
	require.NoError(t, err)

	sigCh := make(chan os.Signal, 1)

	// Simula escrita no log enquanto monitor passivo está rodando e encerra com SIGTERM
	go func() {
		time.Sleep(50 * time.Millisecond)
		f, errOpen := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
		if errOpen == nil {
			_, _ = f.WriteString("2026/10/06 [DELTA INDEXER SUCCESS] teste passivo\n")
			_ = f.Close()
		}
		time.Sleep(200 * time.Millisecond)
		sigCh <- syscall.SIGTERM
	}()

	err = cli.RunWatchWithSignal(projService, codeService, watcherService, sigCh)
	assert.NoError(t, err)
}

func TestCLI_RunWatch_RepeatedExecution_InSameProcess(t *testing.T) {
	projService, codeService, watcherService, db, _ := setupTestWatchEnv(t)
	defer db.Close()

	// 1ª execução do watch no mesmo processo (ex: menu do dashboard)
	sigCh1 := make(chan os.Signal, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		sigCh1 <- syscall.SIGINT
	}()
	err1 := cli.RunWatchWithSignal(projService, codeService, watcherService, sigCh1)
	assert.NoError(t, err1)

	// 2ª execução do watch no mesmo processo com a mesma instância de watcherService
	sigCh2 := make(chan os.Signal, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		sigCh2 <- syscall.SIGINT
	}()
	err2 := cli.RunWatchWithSignal(projService, codeService, watcherService, sigCh2)
	assert.NoError(t, err2)
}

