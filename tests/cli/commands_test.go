package cli_test

import (
	"astrix/internal/cli"
	"astrix/internal/service"
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureOutput(f func()) string {
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	f()
	_ = w.Close()
	os.Stdout = oldStdout
	return <-outChan
}

func setupTestCLI(t *testing.T) (*service.ProjectService, *storage.ProjectRepo, *storage.DB) {
	tempDB := filepath.Join(t.TempDir(), "cli_test.db")
	database, err := storage.NewDatabase(tempDB)
	require.NoError(t, err)

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)
	projService := service.NewProjectService(projectRepo, symbolRepo, engine)
	return projService, projectRepo, database
}

func TestCLI_RunList_JSON(t *testing.T) {
	projService, projectRepo, db := setupTestCLI(t)
	defer db.Close()

	// 1. Lista vazia em JSON
	outputEmpty := captureOutput(func() {
		err := cli.RunList(projService, "--json")
		assert.NoError(t, err)
	})

	var projectsEmpty []storage.Project
	err := json.Unmarshal([]byte(outputEmpty), &projectsEmpty)
	require.NoError(t, err)
	assert.Empty(t, projectsEmpty)

	// 2. Insere projetos e lista em JSON
	err = projectRepo.Create(&storage.Project{
		ID:       "proj-1",
		Name:     "Project One",
		Path:     "/path/to/one",
		Language: "go",
		Status:   storage.StatusReady,
	})
	require.NoError(t, err)

	outputFilled := captureOutput(func() {
		err := cli.RunList(projService, "--json")
		assert.NoError(t, err)
	})

	var projectsFilled []storage.Project
	err = json.Unmarshal([]byte(outputFilled), &projectsFilled)
	require.NoError(t, err)
	require.Len(t, projectsFilled, 1)
	assert.Equal(t, "proj-1", projectsFilled[0].ID)
	assert.Equal(t, "Project One", projectsFilled[0].Name)
}

func TestCLI_RunStatus_JSON(t *testing.T) {
	projService, projectRepo, db := setupTestCLI(t)
	defer db.Close()

	err := projectRepo.Create(&storage.Project{
		ID:       "proj-status",
		Name:     "Status Project",
		Path:     "/path/to/status",
		Language: "typescript",
		Status:   storage.StatusReady,
	})
	require.NoError(t, err)

	output := captureOutput(func() {
		err := cli.RunStatus(projService, "--json")
		assert.NoError(t, err)
	})

	var st cli.StatusOutput
	err = json.Unmarshal([]byte(output), &st)
	require.NoError(t, err)
	assert.Equal(t, 1, st.ProjectsCount)
	assert.NotEmpty(t, st.DatabasePath)
}

func TestCLI_FastPath_VersionAndHelp(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// 1. Testa 'version'
	os.Args = []string{"astrix", "version"}
	versionOut := captureOutput(func() {
		err := cli.Execute()
		assert.NoError(t, err)
	})
	assert.Contains(t, versionOut, "astrix v")

	// 2. Testa '-v'
	os.Args = []string{"astrix", "-v"}
	vOut := captureOutput(func() {
		err := cli.Execute()
		assert.NoError(t, err)
	})
	assert.Contains(t, vOut, "astrix v")

	// 3. Testa '--help'
	os.Args = []string{"astrix", "--help"}
	helpOut := captureOutput(func() {
		err := cli.Execute()
		assert.NoError(t, err)
	})
	assert.Contains(t, helpOut, "Interface de Linha de Comando")
	assert.Contains(t, helpOut, "Uso: astrix [comando]")
}

func TestCLI_ResetDanglingIndexingStatus_Conditional(t *testing.T) {
	_, projectRepo, db := setupTestCLI(t)
	defer db.Close()

	// Cria projeto com status StatusIndexing
	proj := &storage.Project{
		ID:       "proj-dangling",
		Name:     "Dangling Proj",
		Path:     "/path/dangling",
		Language: "go",
		Status:   storage.StatusIndexing,
	}
	err := projectRepo.Create(proj)
	require.NoError(t, err)

	// Simula processo MCP ativo gravando PID deste próprio processo de teste
	err = cli.WritePID()
	require.NoError(t, err)
	defer cli.RemovePID()

	pid, isAlive := cli.ReadPID()
	require.True(t, isAlive)
	assert.Equal(t, os.Getpid(), pid)

	// Quando o processo está vivo (isAlive == true), a CLI não executa o reset
	if !isAlive {
		_ = projectRepo.ResetDanglingIndexingStatus()
	}
	p, err := projectRepo.GetByID("proj-dangling")
	require.NoError(t, err)
	assert.Equal(t, storage.StatusIndexing, p.Status)

	// Quando o processo NÃO está ativo (RemovePID), o reset é executado
	cli.RemovePID()
	_, isAliveAfterRemoval := cli.ReadPID()
	assert.False(t, isAliveAfterRemoval)

	if !isAliveAfterRemoval {
		err = projectRepo.ResetDanglingIndexingStatus()
		require.NoError(t, err)
	}

	pReset, err := projectRepo.GetByID("proj-dangling")
	require.NoError(t, err)
	assert.Equal(t, storage.StatusReady, pReset.Status)
}
