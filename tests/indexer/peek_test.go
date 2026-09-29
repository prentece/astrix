package indexer_test

import (
	"astrix/pkg/storage"
	"astrix/pkg/indexer"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeekFile_FormattingAndLimits(t *testing.T) {
	tempDir := t.TempDir()
	tempDB := tempDir + "/test_peek.db"
	database, err := storage.NewDatabase(tempDB)
	require.NoError(t, err)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)

	// Cria arquivo com 150 linhas
	var lines []string
	for i := 1; i <= 150; i++ {
		lines = append(lines, fmt.Sprintf("line content %d", i))
	}
	filePath := "large_file.txt"
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, filePath), []byte(strings.Join(lines, "\n")), 0o644))

	proj := &storage.Project{
		ID:   "proj-peek-test",
		Name: "Peek Test Project",
		Path: tempDir,
	}
	require.NoError(t, projectRepo.Create(proj))

	t.Run("Default window of 50 lines when end_line is 0", func(t *testing.T) {
		out, err := engine.PeekFile(proj, filePath, 1, 0, "")
		require.NoError(t, err)

		// Header checks: contains file, range, language and window info
		assert.Contains(t, out, "[large_file.txt (lines 1-50 of 150) | txt | window: 50/100]")
		// Compact format '<line>: <content>'
		assert.Contains(t, out, "1: line content 1\n")
		assert.Contains(t, out, "50: line content 50\n")
		assert.NotContains(t, out, "51: line content 51")
		// No legacy vertical pipe in lines
		assert.NotContains(t, out, " 1 | ")
		assert.NotContains(t, out, " | line content")
	})

	t.Run("Truncates windows larger than 100 lines", func(t *testing.T) {
		out, err := engine.PeekFile(proj, filePath, 1, 150, "")
		require.NoError(t, err)

		// Truncated to 100 lines (lines 1-100)
		assert.Contains(t, out, "[large_file.txt (lines 1-100 of 150) | txt | window: 100/100]")
		assert.Contains(t, out, "100: line content 100\n")
		assert.NotContains(t, out, "101: line content 101")
	})

	t.Run("Specific slice within range", func(t *testing.T) {
		out, err := engine.PeekFile(proj, filePath, 20, 25, "")
		require.NoError(t, err)

		assert.Contains(t, out, "[large_file.txt (lines 20-25 of 150) | txt | window: 6/100]")
		assert.Contains(t, out, "20: line content 20\n")
		assert.Contains(t, out, "25: line content 25\n")
		assert.NotContains(t, out, "19: line content 19")
		assert.NotContains(t, out, "26: line content 26")
	})

	t.Run("Symbol Context Overlay", func(t *testing.T) {
		// Insere símbolos no intervalo
		err := symbolRepo.SaveSymbols(proj.ID, []*storage.Symbol{
			{
				ProjectID: proj.ID,
				File:      filePath,
				Name:      "ProcessData",
				Kind:      "function",
				StartLine: 22,
				EndLine:   24,
				StartByte: 0,
				EndByte:   10,
			},
		})
		require.NoError(t, err)

		out, err := engine.PeekFile(proj, filePath, 20, 30, "")
		require.NoError(t, err)

		assert.Contains(t, out, "⊕ Symbols in range:")
		assert.Contains(t, out, "ƒ ProcessData (function, lines 22-24) — exact match")
	})

	t.Run("Smart Anchor with existing symbol", func(t *testing.T) {
		out, err := engine.PeekFile(proj, filePath, 0, 0, "ProcessData")
		require.NoError(t, err)

		// Deve centralizar ou posicionar a janela no símbolo (linhas 22-24)
		assert.Contains(t, out, "22: line content 22\n")
		assert.Contains(t, out, "23: line content 23\n")
		assert.Contains(t, out, "24: line content 24\n")
		assert.Contains(t, out, "⊕ Symbols in range:")
		assert.Contains(t, out, "ProcessData")
	})

	t.Run("Smart Anchor fallback for unknown symbol", func(t *testing.T) {
		out, err := engine.PeekFile(proj, filePath, 0, 0, "NonExistentFunction")
		require.NoError(t, err)

		// Deve emitir aviso amigável e mostrar o início do arquivo sem falhar
		assert.Contains(t, out, "⚠ Symbol 'NonExistentFunction' not found in index. Showing file start.")
		assert.Contains(t, out, "1: line content 1\n")
	})
}
