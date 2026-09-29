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

// GetPidPath retorna o caminho para o arquivo de PID ~/.astrix/astrix.pid.
func GetPidPath() (string, error) {
	homeDir, err := GetAstrixHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "astrix.pid"), nil
}

// WritePID grava o PID do processo atual no arquivo astrix.pid.
func WritePID() error {
	pidPath, err := GetPidPath()
	if err != nil {
		return err
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

// RemovePID remove o arquivo de PID ao encerrar o processo.
func RemovePID() {
	if pidPath, err := GetPidPath(); err == nil {
		_ = os.Remove(pidPath)
	}
}

// ReadPID lê o PID gravado e verifica se o processo ainda está ativo.
func ReadPID() (int, bool) {
	pidPath, err := GetPidPath()
	if err != nil {
		return 0, false
	}
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, false
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return 0, false
	}

	// Verifica se o processo está vivo no SO
	process, err := os.FindProcess(pid)
	if err != nil {
		return pid, false
	}
	// Em sistemas Unix, Signal(0) verifica se o processo existe sem enviar sinal destrutivo
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return pid, false
	}
	return pid, true
}
