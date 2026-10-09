package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// GetAstrixHomeDir retorna o diretório global ~/.astrix do usuário no SO atual (Linux, macOS, Windows).
// Suporta override via variável de ambiente ASTRIX_HOME.
func GetAstrixHomeDir() (string, error) {
	if customHome := os.Getenv("ASTRIX_HOME"); customHome != "" {
		if err := os.MkdirAll(customHome, 0o755); err != nil {
			return "", fmt.Errorf("falha ao criar ASTRIX_HOME em %s: %w", customHome, err)
		}
		return customHome, nil
	}

	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("falha ao obter diretório home do usuário: %w", err)
	}

	astrixDir := filepath.Join(userHome, ".astrix")
	if err := os.MkdirAll(astrixDir, 0o755); err != nil {
		return "", fmt.Errorf("falha ao criar diretório %s: %w", astrixDir, err)
	}

	return astrixDir, nil
}

// GetDatabasePath retorna o caminho completo para o banco de dados central astrix.db.
// Prioridade:
// 1. Variável de ambiente DB_PATH
// 2. ~/.astrix/astrix.db
func GetDatabasePath() (string, error) {
	if customDB := os.Getenv("DB_PATH"); customDB != "" {
		dir := filepath.Dir(customDB)
		if dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		return customDB, nil
	}

	homeDir, err := GetAstrixHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, "astrix.db"), nil
}

// GetLogPath retorna o caminho para o arquivo de log global ~/.astrix/astrix.log.
func GetLogPath() (string, error) {
	homeDir, err := GetAstrixHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "astrix.log"), nil
}

// GetWatcherLockPath retorna o caminho para o arquivo de lock exclusivo do watcher ~/.astrix/watcher.lock.
func GetWatcherLockPath() (string, error) {
	homeDir, err := GetAstrixHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "watcher.lock"), nil
}

// GetPidsDir retorna o caminho para o diretório ~/.astrix/pids onde cada instância registra seu arquivo de presença.
func GetPidsDir() (string, error) {
	homeDir, err := GetAstrixHomeDir()
	if err != nil {
		return "", err
	}
	pidsDir := filepath.Join(homeDir, "pids")
	if err := os.MkdirAll(pidsDir, 0o755); err != nil {
		return "", fmt.Errorf("falha ao criar diretório pids: %w", err)
	}
	return pidsDir, nil
}

// RegisterInstancePID registra o PID da instância MCP em ~/.astrix/pids/<pid>.pid.
func RegisterInstancePID(pid int) error {
	pidsDir, err := GetPidsDir()
	if err != nil {
		return err
	}
	pidFile := filepath.Join(pidsDir, fmt.Sprintf("%d.pid", pid))
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0o644)
}

// UnregisterInstancePID remove o arquivo de presença da instância ao encerrar.
func UnregisterInstancePID(pid int) {
	if pidsDir, err := GetPidsDir(); err == nil {
		_ = os.Remove(filepath.Join(pidsDir, fmt.Sprintf("%d.pid", pid)))
	}
}

// IsProcessAlive verifica se um processo com o determinado PID está vivo no SO.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Em sistemas Unix, Signal(0) verifica se o processo existe sem enviar sinal destrutivo
	return process.Signal(syscall.Signal(0)) == nil
}

// ListActivePIDs retorna a lista ordenada de PIDs de instâncias ativas no sistema,
// realizando auto-limpeza de registros órfãos deixados por processos encerrados abruptamente.
func ListActivePIDs() ([]int, error) {
	pidsDir, err := GetPidsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(pidsDir)
	if err != nil {
		return nil, err
	}

	var active []int
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pid") {
			continue
		}
		pidStr := strings.TrimSuffix(entry.Name(), ".pid")
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 0 {
			_ = os.Remove(filepath.Join(pidsDir, entry.Name()))
			continue
		}

		if IsProcessAlive(pid) {
			active = append(active, pid)
		} else {
			// Limpa registro de processo zumbi/finalizado
			_ = os.Remove(filepath.Join(pidsDir, entry.Name()))
		}
	}
	return active, nil
}

// GetPidPath retorna o caminho para o arquivo de PID ~/.astrix/astrix.pid (Líder).
func GetPidPath() (string, error) {
	homeDir, err := GetAstrixHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "astrix.pid"), nil
}

// WritePID grava o PID do processo atual no arquivo astrix.pid.
func WritePID() error {
	return WriteLeaderPID(os.Getpid())
}

// WriteLeaderPID grava o PID informado no arquivo astrix.pid como líder ativo.
func WriteLeaderPID(pid int) error {
	pidPath, err := GetPidPath()
	if err != nil {
		return err
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0o644)
}

// RemovePID remove o arquivo astrix.pid se contiver o PID do processo atual ou se estiver inválido.
func RemovePID() {
	pidPath, err := GetPidPath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return
	}
	if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid == os.Getpid() {
		_ = os.Remove(pidPath)
	}
}

// ReadPID lê o PID gravado e verifica se o processo líder (ou qualquer réplica ativa) ainda está ativo.
func ReadPID() (int, bool) {
	pidPath, err := GetPidPath()
	if err == nil {
		if data, err := os.ReadFile(pidPath); err == nil {
			pidStr := strings.TrimSpace(string(data))
			if pid, err := strconv.Atoi(pidStr); err == nil && IsProcessAlive(pid) {
				return pid, true
			}
		}
	}

	// Fallback para lista de instâncias registradas
	if active, err := ListActivePIDs(); err == nil && len(active) > 0 {
		return active[0], true
	}

	return 0, false
}
