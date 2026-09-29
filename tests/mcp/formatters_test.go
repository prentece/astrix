package mcp_test

import (
	"astrix/internal/mcp"
	"astrix/pkg/storage"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatSymbols_CompactText(t *testing.T) {
	symbols := []*storage.Symbol{
		{
			ID:        18671,
			ProjectID: "proj-uuid-123",
			File:      "src/api/paymentGateway/model/paymentGateway.dto.ts",
			Name:      "UserSessionHub",
			Kind:      storage.KindClass,
			Signature: "class UserSessionHub<T>",
			Language:  "typescript",
			StartLine: 4,
			EndLine:   9,
			StartByte: 108,
			EndByte:   211,
		},
		{
			ID:        18695,
			ProjectID: "proj-uuid-123",
			File:      "src/api/paymentMethod/controller/paymentMethod.controller.ts",
			Name:      "getChangeEligibilityUser",
			Kind:      storage.KindMethod,
			Signature: "getChangeEligibilityUser(req: Request)",
			Language:  "typescript",
			StartLine: 61,
			EndLine:   65,
			StartByte: 1883,
			EndByte:   2092,
		},
	}

	// 1. Formato Compacto (Texto) sem mais páginas
	textOutput := mcp.FormatSymbols(symbols, false, 25, 0, false)
	assert.NotEmpty(t, textOutput)

	// Não deve conter IDs de banco nem UUIDs nem start/end bytes
	assert.NotContains(t, textOutput, "proj-uuid-123")
	assert.NotContains(t, textOutput, "18671")
	assert.NotContains(t, textOutput, "start_byte")
	assert.NotContains(t, textOutput, "end_byte")
	assert.NotContains(t, textOutput, "typescript")

	// Deve conter formato {file}:{start_line}-{end_line} [{kind}] {signature}
	lines := strings.Split(textOutput, "\n")
	assert.Len(t, lines, 2)
	assert.Equal(t, "src/api/paymentGateway/model/paymentGateway.dto.ts:4-9 [class] class UserSessionHub<T>", lines[0])
	assert.Equal(t, "src/api/paymentMethod/controller/paymentMethod.controller.ts:61-65 [method] getChangeEligibilityUser(req: Request)", lines[1])

	// 2. Formato Compacto (Texto) COM mais páginas (hasMore = true)
	textWithMore := mcp.FormatSymbols(symbols, false, 2, 0, true)
	assert.Contains(t, textWithMore, "More results available: call with offset=2")

	// 3. Formato JSON Enxuto
	jsonOutput := mcp.FormatSymbols(symbols, true, 2, 0, true)
	assert.NotEmpty(t, jsonOutput)

	// Valida ausência de campos internos
	assert.NotContains(t, jsonOutput, "proj-uuid-123")
	assert.NotContains(t, jsonOutput, "18671")
	assert.NotContains(t, jsonOutput, "start_byte")
	assert.NotContains(t, jsonOutput, "end_byte")
	assert.NotContains(t, jsonOutput, "language")

	// Valida metadados de paginação no JSON
	assert.Contains(t, jsonOutput, `"has_more": true`)
	assert.Contains(t, jsonOutput, `"next_offset": 2`)

	// Valida ausência de escape HTML
	assert.Contains(t, jsonOutput, "class UserSessionHub<T>")
	assert.NotContains(t, jsonOutput, "\\u003c")
	assert.NotContains(t, jsonOutput, "\\u003e")
}

func TestFormatReferences_CompactText(t *testing.T) {
	refs := []*storage.CallerInfo{
		{
			ID:         100,
			ProjectID:  "proj-uuid-123",
			File:       "src/app.ts",
			Line:       42,
			Text:       "const hub = new UserSessionHub();",
			SymbolName: "UserSessionHub",
		},
	}

	text := mcp.FormatReferences("UserSessionHub", refs, false, 30, 0, false)
	assert.Equal(t, "src/app.ts:42: const hub = new UserSessionHub();", text)
	assert.NotContains(t, text, "proj-uuid-123")
	assert.NotContains(t, text, "100")

	// Com paginação
	textWithMore := mcp.FormatReferences("UserSessionHub", refs, false, 1, 0, true)
	assert.Contains(t, textWithMore, "More references available: call with offset=1")
}


func TestFormatProjects(t *testing.T) {
	projects := []*storage.Project{
		{
			ID:          "5c53778d-fd61-42b4-85bb-2f393659c605",
			Name:        "Scripts Mongo",
			Path:        "/mnt/host/nio/app/scripts-mongo",
			Language:    "javascript",
			Status:      storage.StatusReady,
			FileCount:   4,
			SymbolCount: 0,
		},
		{
			ID:          "980c18f5-51b0-4181-ad82-55e6330d6101",
			Name:        "Backend",
			Path:        "/mnt/host/nio/app/backend",
			Language:    "typescript",
			Status:      storage.StatusReady,
			FileCount:   155,
			SymbolCount: 378,
		},
	}

	// 1. Tabular Text Output (default)
	text := mcp.FormatProjects(projects, false)
	assert.Contains(t, text, "PROJECT_ID")
	assert.Contains(t, text, "NAME")
	assert.Contains(t, text, "Scripts Mongo")
	assert.Contains(t, text, "Backend")
	// Não deve ter timestamps verbose
	assert.NotContains(t, text, "created_at")
	assert.NotContains(t, text, "updated_at")
	assert.NotContains(t, text, "indexed_at")

	// 2. Lean JSON Output
	jsonOut := mcp.FormatProjects(projects, true)
	assert.Contains(t, jsonOut, `"id": "5c53778d-fd61-42b4-85bb-2f393659c605"`)
	assert.Contains(t, jsonOut, `"name": "Scripts Mongo"`)
	assert.NotContains(t, jsonOut, "created_at")
	assert.NotContains(t, jsonOut, "indexed_at")
}

func TestFormatGrepMatches_ContextDedup_AdjacentMatches(t *testing.T) {
	matches := []storage.GrepMatch{
		{
			File:          "a.go",
			Line:          10,
			Content:       "matchA",
			ContextBefore: []string{"prev"},
			ContextAfter:  []string{"matchB"},
		},
		{
			File:          "a.go",
			Line:          11,
			Content:       "matchB",
			ContextBefore: []string{"matchA"},
			ContextAfter:  []string{"next"},
		},
	}
	out := mcp.FormatGrepMatches("match", matches, false, 30, 0, false)
	assert.Equal(t, 1, strings.Count(out, "a.go:10:"))
	assert.Equal(t, 1, strings.Count(out, "a.go:11:"))
	assert.NotContains(t, out, "a.go:10-")
	assert.NotContains(t, out, "a.go:11-")
	expected := "a.go:9- prev\na.go:10: matchA\na.go:11: matchB\na.go:12- next"
	assert.Equal(t, expected, out)
}

func TestFormatGrepMatches_ContextDedup_GapLine(t *testing.T) {
	matches := []storage.GrepMatch{
		{
			File:          "a.go",
			Line:          10,
			Content:       "matchA",
			ContextBefore: nil,
			ContextAfter:  []string{"gap"},
		},
		{
			File:          "a.go",
			Line:          12,
			Content:       "matchB",
			ContextBefore: []string{"gap"},
			ContextAfter:  nil,
		},
	}
	out := mcp.FormatGrepMatches("match", matches, false, 30, 0, false)
	assert.Equal(t, 1, strings.Count(out, "a.go:11-"))
	expected := "a.go:10: matchA\na.go:11- gap\na.go:12: matchB"
	assert.Equal(t, expected, out)
}

func TestFormatGrepMatches_IsolatedMatch_ExactFormat(t *testing.T) {
	matches := []storage.GrepMatch{
		{
			File:          "server.go",
			Line:          4,
			Content:       "port := 9090",
			ContextBefore: []string{"// Initialize server"},
			ContextAfter:  []string{"startServer(port)"},
		},
	}
	out := mcp.FormatGrepMatches("port", matches, false, 30, 0, false)
	assert.Equal(t,
		"server.go:3- // Initialize server\nserver.go:4: port := 9090\nserver.go:5- startServer(port)",
		out)
}

func TestFormatGrepMatches_ClusterOfThree_Context1(t *testing.T) {
	matches := []storage.GrepMatch{
		{
			File:          "a.go",
			Line:          10,
			Content:       "hit1",
			ContextBefore: nil,
			ContextAfter:  []string{"hit2"},
		},
		{
			File:          "a.go",
			Line:          11,
			Content:       "hit2",
			ContextBefore: []string{"hit1"},
			ContextAfter:  []string{"hit3"},
		},
		{
			File:          "a.go",
			Line:          12,
			Content:       "hit3",
			ContextBefore: []string{"hit2"},
			ContextAfter:  nil,
		},
	}
	out := mcp.FormatGrepMatches("hit", matches, false, 30, 0, false)
	assert.Equal(t, "a.go:10: hit1\na.go:11: hit2\na.go:12: hit3", out)
	assert.Equal(t, 0, strings.Count(out, "a.go:10-"))
	assert.Equal(t, 0, strings.Count(out, "a.go:11-"))
	assert.Equal(t, 0, strings.Count(out, "a.go:12-"))
}

func TestFormatGrepMatches_MultiFile_NoCrossDedup(t *testing.T) {
	matches := []storage.GrepMatch{
		{File: "a.go", Line: 5, Content: "alpha"},
		{File: "b.go", Line: 5, Content: "beta"},
	}
	out := mcp.FormatGrepMatches("x", matches, false, 30, 0, false)
	assert.Equal(t, "a.go:5: alpha\nb.go:5: beta", out)
}

func TestFormatGrepMatches_JSON_UnchangedShape(t *testing.T) {
	matches := []storage.GrepMatch{
		{
			File:          "a.go",
			Line:          10,
			Content:       "matchA",
			ContextBefore: nil,
			ContextAfter:  []string{"matchB"},
		},
		{
			File:          "a.go",
			Line:          11,
			Content:       "matchB",
			ContextBefore: []string{"matchA"},
			ContextAfter:  nil,
		},
	}
	out := mcp.FormatGrepMatches("match", matches, true, 30, 0, false)
	assert.Contains(t, out, `"context_after"`)
	assert.Contains(t, out, `"context_before"`)
	assert.Contains(t, out, `"matchA"`)
	assert.Contains(t, out, `"matchB"`)
}

func TestFormatGrepMatches_HasMoreFooter(t *testing.T) {
	matches := []storage.GrepMatch{
		{File: "a.go", Line: 1, Content: "x"},
	}
	out := mcp.FormatGrepMatches("x", matches, false, 1, 0, true)
	assert.Contains(t, out, "a.go:1: x")
	assert.Contains(t, out, "More matches available: call with offset=1")
}

func TestFormatGrepMatches_NoMatches(t *testing.T) {
	assert.Equal(t, "No matches found for pattern 'x'.", mcp.FormatGrepMatches("x", nil, false, 30, 0, false))
	assert.Equal(t, "No more matches found for pattern 'x' at offset 5.", mcp.FormatGrepMatches("x", nil, false, 30, 5, false))
}
