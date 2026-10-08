package cli_test

import (
	"astrix/internal/cli"
	"astrix/pkg/storage"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcherLock_MutualExclusionAndFailover(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "watcher.lock")

	lock1 := cli.NewWatcherLock(lockPath)
	lock2 := cli.NewWatcherLock(lockPath)

	// 1. Primeira instância obtém o lock de líder
	acquired1, err := lock1.TryAcquire()
	require.NoError(t, err)
	assert.True(t, acquired1)
	assert.True(t, lock1.IsHeld())

	// 2. Segunda instância tenta obter o mesmo lock e é bloqueada
	acquired2, err := lock2.TryAcquire()
	require.NoError(t, err)
	assert.False(t, acquired2)
	assert.False(t, lock2.IsHeld())

	// 3. Primeira instância é fechada (failover)
	lock1.Release()
	assert.False(t, lock1.IsHeld())

	// 4. Segunda instância agora consegue adquirir o lock (assume a liderança)
	acquired2After, err := lock2.TryAcquire()
	require.NoError(t, err)
	assert.True(t, acquired2After)
	assert.True(t, lock2.IsHeld())

	// Libera lock final
	lock2.Release()
	assert.False(t, lock2.IsHeld())
}

func TestWatcherLock_PIDRegistryAndActiveList(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("ASTRIX_HOME", tempHome)

	currentPID := os.Getpid()
	err := cli.RegisterInstancePID(currentPID)
	require.NoError(t, err)

	// Cria registro de PID morto/fictício que deve ser auto-limpo
	pidsDir, err := cli.GetPidsDir()
	require.NoError(t, err)
	deadPIDFile := filepath.Join(pidsDir, "99999999.pid")
	err = os.WriteFile(deadPIDFile, []byte("99999999"), 0o644)
	require.NoError(t, err)

	activePIDs, err := cli.ListActivePIDs()
	require.NoError(t, err)

	// Deve conter o PID atual e ter descartado o PID morto
	assert.Contains(t, activePIDs, currentPID)
	assert.NotContains(t, activePIDs, 99999999)

	// O arquivo do PID morto deve ter sido removido do disco
	_, statErr := os.Stat(deadPIDFile)
	assert.True(t, os.IsNotExist(statErr))

	// Ao desregistrar, o PID atual é removido
	cli.UnregisterInstancePID(currentPID)
	activeAfter, err := cli.ListActivePIDs()
	require.NoError(t, err)
	assert.NotContains(t, activeAfter, currentPID)
}

func TestStatus_MultipleInstancesWithLeader(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("ASTRIX_HOME", tempHome)

	projService, projectRepo, db := setupTestCLI(t)
	defer db.Close()

	err := projectRepo.Create(&storage.Project{
		ID:       "proj-multi",
		Name:     "Multi Project",
		Path:     "/path/to/multi",
		Language: "go",
		Status:   storage.StatusReady,
	})
	require.NoError(t, err)

	currentPID := os.Getpid()
	err = cli.RegisterInstancePID(currentPID)
	require.NoError(t, err)
	defer cli.UnregisterInstancePID(currentPID)

	err = cli.WriteLeaderPID(currentPID)
	require.NoError(t, err)
	defer cli.RemovePID()

	// 1. Testa saída JSON
	outputJSON := captureOutput(func() {
		err := cli.RunStatus(projService, "--json")
		assert.NoError(t, err)
	})

	var st cli.StatusOutput
	err = json.Unmarshal([]byte(outputJSON), &st)
	require.NoError(t, err)
	assert.True(t, st.MCPServer.Online)
	assert.Equal(t, currentPID, st.MCPServer.LeaderPID)
	assert.Equal(t, 1, st.MCPServer.InstancesCount)

	// 2. Testa saída visual
	outputVisual := captureOutput(func() {
		err := cli.RunStatus(projService)
		assert.NoError(t, err)
	})

	assert.Contains(t, outputVisual, "Servidor MCP:")
	assert.Contains(t, outputVisual, "Líder / Watcher")
}
