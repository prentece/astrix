package mcp_test

import (
	"bytes"
	"astrix/pkg/storage"
	"astrix/pkg/indexer"
	"astrix/internal/mcp"
	"astrix/internal/service"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "astrix/pkg/indexer/languages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestMCP(t *testing.T) (*mcp.Server, *storage.DB) {
	tempDB := t.TempDir() + "/mcp_test.db"
	database, err := storage.NewDatabase(tempDB)
	require.NoError(t, err)

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)

	_ = projectRepo.Create(&storage.Project{
		ID:       "proj-mcp-1",
		Name:     "MCP Test Project",
		Path:     "/test/path",
		Language: "go",
		Status:   storage.StatusReady,
	})

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)

	projectService := service.NewProjectService(projectRepo, symbolRepo, engine)
	codeService := service.NewCodeService(projectRepo, symbolRepo, depRepo, dataModelRepo, engine)

	server := mcp.NewServer(projectService, codeService)
	return server, database
}

func TestMCPServer_ListProjectsTool(t *testing.T) {
	server, database := setupTestMCP(t)
	defer database.Close()

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "list_projects",
			"arguments": map[string]any{},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	result, ok := resp["result"].(map[string]any)
	require.True(t, ok)
	content, ok := result["content"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, content)
}

func TestMCPServer_GetProjectStructureTool(t *testing.T) {
	tempProjectDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempProjectDir, "main.go"), []byte("package main"), 0o644)
	_ = os.MkdirAll(filepath.Join(tempProjectDir, "pkg"), 0o755)
	_ = os.WriteFile(filepath.Join(tempProjectDir, "pkg", "util.go"), []byte("package pkg"), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	_ = projectRepo.Create(&storage.Project{
		ID:       "proj-tree-test",
		Name:     "Tree Test Project",
		Path:     tempProjectDir,
		Language: "go",
		Status:   storage.StatusReady,
	})

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "get_project_structure",
			"arguments": map[string]any{
				"project_id":  "proj-tree-test",
				"depth":       2,
				"show_hidden": false,
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	result, ok := resp["result"].(map[string]any)
	require.True(t, ok)
	content, ok := result["content"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, content)

	firstItem, ok := content[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "text", firstItem["type"])
	text, ok := firstItem["text"].(string)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(text, "root/\n"))
	assert.Contains(t, text, "pkg/")
	assert.Contains(t, text, "util.go")
	assert.Contains(t, text, "main.go")
}

func TestMCPServer_QueryStructuredFileTool(t *testing.T) {
	tempProjectDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempProjectDir, "package.json"), []byte(`{"name":"astrix","version":"2.0.0","dependencies":{"@nestjs/core":"7.6.14"}}`), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	_ = projectRepo.Create(&storage.Project{
		ID:       "proj-query-test",
		Name:     "Query Test Project",
		Path:     tempProjectDir,
		Language: "javascript",
		Status:   storage.StatusReady,
	})

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "query_structured_file",
			"arguments": map[string]any{
				"project_id": "proj-query-test",
				"filepath":   "package.json",
				"query":      "dependencies.@nestjs/core",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	result, ok := resp["result"].(map[string]any)
	require.True(t, ok)
	content, ok := result["content"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, content)

	firstItem, ok := content[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "text", firstItem["type"])
	text, ok := firstItem["text"].(string)
	require.True(t, ok)
	assert.Equal(t, "7.6.14", text)
}

func TestMCPServer_ReadFileLines_CanonicalParameters(t *testing.T) {
	tempProjectDir := t.TempDir()
	sampleContent := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	_ = os.WriteFile(filepath.Join(tempProjectDir, "engine.go"), []byte(sampleContent), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	_ = projectRepo.Create(&storage.Project{
		ID:       "proj-peek-canonical",
		Name:     "Peek Test",
		Path:     tempProjectDir,
		Language: "go",
		Status:   storage.StatusReady,
	})

	// 1. Canonical parameter call: filepath, start_line, end_line
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "read_file_lines",
			"arguments": map[string]any{
				"project_id": "proj-peek-canonical",
				"filepath":   "engine.go",
				"start_line": 1,
				"end_line":   3,
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	result, ok := resp["result"].(map[string]any)
	require.True(t, ok)
	content, ok := result["content"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, content)

	firstItem, ok := content[0].(map[string]any)
	require.True(t, ok)
	text, ok := firstItem["text"].(string)
	require.True(t, ok)

	assert.Contains(t, text, "1: line 1")
	assert.Contains(t, text, "3: line 3")

	// 2. Strict rejection when missing required filepath
	payloadInvalid := map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "read_file_lines",
			"arguments": map[string]any{
				"project_id": "proj-peek-canonical",
			},
		},
	}
	bodyInv, _ := json.Marshal(payloadInvalid)
	reqInv := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(bodyInv))
	wInv := httptest.NewRecorder()
	server.HandleMessage()(wInv, reqInv)
	var respInv map[string]any
	_ = json.Unmarshal(wInv.Body.Bytes(), &respInv)
	resInv, _ := respInv["result"].(map[string]any)
	contentInv, _ := resInv["content"].([]any)
	itemInv, _ := contentInv[0].(map[string]any)
	assert.Equal(t, "Field 'filepath' is required", itemInv["text"])

	// 3. Anchor symbol parameter call
	payloadAnchor := map[string]any{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "read_file_lines",
			"arguments": map[string]any{
				"project_id":    "proj-peek-canonical",
				"filepath":      "engine.go",
				"anchor_symbol": "NonExistent",
			},
		},
	}
	bodyAnchor, _ := json.Marshal(payloadAnchor)
	reqAnchor := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(bodyAnchor))
	wAnchor := httptest.NewRecorder()
	server.HandleMessage()(wAnchor, reqAnchor)
	var respAnchor map[string]any
	_ = json.Unmarshal(wAnchor.Body.Bytes(), &respAnchor)
	resAnchor, _ := respAnchor["result"].(map[string]any)
	contentAnchor, _ := resAnchor["content"].([]any)
	itemAnchor, _ := contentAnchor[0].(map[string]any)
	assert.Contains(t, itemAnchor["text"], "⚠ Symbol 'NonExistent' not found in index")
}

func TestMCPServer_FindSymbolAndImplementation(t *testing.T) {
	tempProjectDir := t.TempDir()
	sourceCode := `package main

func CalculateTotal(a int, b int) int {
	return a + b
}
`
	_ = os.WriteFile(filepath.Join(tempProjectDir, "math.go"), []byte(sourceCode), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)

	_ = projectRepo.Create(&storage.Project{
		ID:       "proj-sym-test",
		Name:     "Symbol Test Project",
		Path:     tempProjectDir,
		Language: "go",
		Status:   storage.StatusReady,
	})

	_ = symbolRepo.SaveSymbols("proj-sym-test", []*storage.Symbol{
		{
			ProjectID:      "proj-sym-test",
			File:           "math.go",
			Name:           "CalculateTotal",
			Kind:           storage.KindFunction,
			Language:       "go",
			StartLine:      3,
			EndLine:        5,
			StartByte:      14,
			EndByte:        68,
			RelevanceScore: 1.0,
		},
	})

	// 1. lookup_symbol (mode=definition)
	findPayload := map[string]any{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "lookup_symbol",
			"arguments": map[string]any{
				"project_id":  "proj-sym-test",
				"symbol_name": "CalculateTotal",
				"mode":        "definition",
			},
		},
	}
	body, _ := json.Marshal(findPayload)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	res, _ := resp["result"].(map[string]any)
	content, _ := res["content"].([]any)
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	assert.Contains(t, text, "CalculateTotal")
	assert.Contains(t, text, "math.go")

	// 2. get_implementation
	implPayload := map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "get_implementation",
			"arguments": map[string]any{
				"project_id":  "proj-sym-test",
				"filepath":    "math.go",
				"symbol_name": "CalculateTotal",
			},
		},
	}
	bodyImpl, _ := json.Marshal(implPayload)
	reqImpl := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(bodyImpl))
	wImpl := httptest.NewRecorder()
	server.HandleMessage()(wImpl, reqImpl)
	assert.Equal(t, http.StatusOK, wImpl.Code)

	var respImpl map[string]any
	_ = json.Unmarshal(wImpl.Body.Bytes(), &respImpl)
	resImpl, _ := respImpl["result"].(map[string]any)
	contentImpl, _ := resImpl["content"].([]any)
	itemImpl, _ := contentImpl[0].(map[string]any)
	textImpl, _ := itemImpl["text"].(string)
	assert.Contains(t, textImpl, "func CalculateTotal(a int, b int) int {")
	assert.Contains(t, textImpl, "return a + b")
}

func TestMCPServer_FindReferences(t *testing.T) {
	server, database := setupTestMCP(t)
	defer database.Close()

	symbolRepo := storage.NewSymbolRepo(database)
	_ = symbolRepo.SaveReferences("proj-mcp-1", []*storage.CallerInfo{
		{
			ProjectID:  "proj-mcp-1",
			SymbolName: "CalculateTotal",
			File:       "service.go",
			Line:       42,
			Text:       "total := CalculateTotal(x, y)",
		},
	})

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      8,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "lookup_symbol",
			"arguments": map[string]any{
				"project_id":  "proj-mcp-1",
				"symbol_name": "CalculateTotal",
				"mode":        "references",
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	res, _ := resp["result"].(map[string]any)
	content, _ := res["content"].([]any)
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	assert.Contains(t, text, "service.go:42")
	assert.Contains(t, text, "total := CalculateTotal(x, y)")
}

func TestMCPServer_GrepCode(t *testing.T) {
	tempProjectDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempProjectDir, "app.go"), []byte("package main\n\n// TODO: add authentication\nfunc Run() {}\n"), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	_ = projectRepo.Create(&storage.Project{
		ID:       "proj-grep-test",
		Name:     "Grep Test",
		Path:     tempProjectDir,
		Language: "go",
		Status:   storage.StatusReady,
	})

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      9,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "grep_code",
			"arguments": map[string]any{
				"project_id": "proj-grep-test",
				"pattern":    "TODO: add authentication",
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	res, _ := resp["result"].(map[string]any)
	content, _ := res["content"].([]any)
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	assert.Contains(t, text, "app.go:3")
	assert.Contains(t, text, "// TODO: add authentication")
}

func TestMCPServer_GrepCode_CaseSensitive(t *testing.T) {
	tempProjectDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempProjectDir, "main.go"), []byte("package main\n\nfunc RunServer() {}\nfunc runClient() {}\n"), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	_ = projectRepo.Create(&storage.Project{
		ID:       "proj-grep-case",
		Name:     "Grep Case Test",
		Path:     tempProjectDir,
		Language: "go",
		Status:   storage.StatusReady,
	})

	// Case-insensitive (default) - deve achar ambos
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      10,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "grep_code",
			"arguments": map[string]any{
				"project_id": "proj-grep-case",
				"pattern":    "run",
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	res, _ := resp["result"].(map[string]any)
	content, _ := res["content"].([]any)
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	assert.Contains(t, text, "RunServer")
	assert.Contains(t, text, "runClient")

	// Case-sensitive - deve achar apenas runClient
	payload2 := map[string]any{
		"jsonrpc": "2.0",
		"id":      11,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "grep_code",
			"arguments": map[string]any{
				"project_id":     "proj-grep-case",
				"pattern":        "run",
				"case_sensitive": true,
			},
		},
	}
	body2, _ := json.Marshal(payload2)
	req2 := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	server.HandleMessage()(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	var resp2 map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	res2, _ := resp2["result"].(map[string]any)
	content2, _ := res2["content"].([]any)
	item2, _ := content2[0].(map[string]any)
	text2, _ := item2["text"].(string)
	assert.NotContains(t, text2, "RunServer")
	assert.Contains(t, text2, "runClient")
}

func TestMCPServer_GrepCode_FileExtensions(t *testing.T) {
	tempProjectDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempProjectDir, "app.go"), []byte("package main\n// MARKER_FIND_ME\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tempProjectDir, "style.css"), []byte("/* MARKER_FIND_ME */\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tempProjectDir, "index.ts"), []byte("// MARKER_FIND_ME\n"), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	_ = projectRepo.Create(&storage.Project{
		ID:       "proj-grep-ext",
		Name:     "Grep Ext Test",
		Path:     tempProjectDir,
		Language: "go",
		Status:   storage.StatusReady,
	})

	// Filtrar apenas .go
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      12,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "grep_code",
			"arguments": map[string]any{
				"project_id":      "proj-grep-ext",
				"pattern":         "MARKER_FIND_ME",
				"file_extensions": ".go",
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	res, _ := resp["result"].(map[string]any)
	content, _ := res["content"].([]any)
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	assert.Contains(t, text, "app.go")
	assert.NotContains(t, text, "style.css")
	assert.NotContains(t, text, "index.ts")
}

func TestMCPServer_GrepCode_ContextLines(t *testing.T) {
	tempProjectDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempProjectDir, "code.go"), []byte("line1\nline2\nTARGET_LINE\nline4\nline5\n"), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	_ = projectRepo.Create(&storage.Project{
		ID:       "proj-grep-ctx",
		Name:     "Grep Context Test",
		Path:     tempProjectDir,
		Language: "go",
		Status:   storage.StatusReady,
	})

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      13,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "grep_code",
			"arguments": map[string]any{
				"project_id":    "proj-grep-ctx",
				"pattern":       "TARGET_LINE",
				"context_lines": 2,
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	res, _ := resp["result"].(map[string]any)
	content, _ := res["content"].([]any)
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	// Deve conter as linhas de contexto
	assert.Contains(t, text, "line1")
	assert.Contains(t, text, "line2")
	assert.Contains(t, text, "TARGET_LINE")
	assert.Contains(t, text, "line4")
	assert.Contains(t, text, "line5")
}

func TestMCPServer_GrepCode_ContextLines_ProximateMatches(t *testing.T) {
	tempProjectDir := t.TempDir()
	// Gap pair at 10/12 (line 11 shared context) + adjacent pair at 20/21
	content := strings.Join([]string{
		"package main", // 1
		"",             // 2
		"",             // 3
		"",             // 4
		"",             // 5
		"",             // 6
		"",             // 7
		"",             // 8
		"",             // 9
		"HIT_A",        // 10
		"middle",       // 11 gap
		"HIT_B",        // 12
		"",             // 13
		"",             // 14
		"",             // 15
		"",             // 16
		"",             // 17
		"",             // 18
		"",             // 19
		"HIT_C",        // 20
		"HIT_D",        // 21 adjacent
		"",
	}, "\n")
	_ = os.WriteFile(filepath.Join(tempProjectDir, "code.go"), []byte(content), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	_ = projectRepo.Create(&storage.Project{
		ID:       "proj-grep-prox",
		Name:     "Grep Proximate Test",
		Path:     tempProjectDir,
		Language: "go",
		Status:   storage.StatusReady,
	})

	callGrep := func(args map[string]any) string {
		payload := map[string]any{
			"jsonrpc": "2.0",
			"id":      14,
			"method":  "tools/call",
			"params": map[string]any{
				"name":      "grep_code",
				"arguments": args,
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
		w := httptest.NewRecorder()
		server.HandleMessage()(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		res, _ := resp["result"].(map[string]any)
		contentArr, _ := res["content"].([]any)
		item, _ := contentArr[0].(map[string]any)
		text, _ := item["text"].(string)
		return text
	}

	t.Run("Text_DedupsOverlappingContext", func(t *testing.T) {
		out := callGrep(map[string]any{
			"project_id":    "proj-grep-prox",
			"pattern":       "HIT_",
			"context_lines": 1,
		})
		t.Logf("--- PROXIMATE TEXT ---\n%s", out)

		// Gap line 11 appears once as context (not twice)
		assert.Equal(t, 1, strings.Count(out, "code.go:11-"))
		assert.Equal(t, 0, strings.Count(out, "code.go:11:"))

		// Adjacent pair mains appear once each; not as context
		assert.Equal(t, 1, strings.Count(out, "code.go:20:"))
		assert.Equal(t, 1, strings.Count(out, "code.go:21:"))
		assert.Equal(t, 0, strings.Count(out, "code.go:20-"))
		assert.Equal(t, 0, strings.Count(out, "code.go:21-"))

		// Gap pair mains
		assert.Equal(t, 1, strings.Count(out, "code.go:10:"))
		assert.Equal(t, 1, strings.Count(out, "code.go:12:"))

		// Non-overlapping context outside clusters still present
		assert.Contains(t, out, "code.go:9-")
		assert.Contains(t, out, "code.go:13-")
	})

	t.Run("JSON_KeepsContextArrays", func(t *testing.T) {
		out := callGrep(map[string]any{
			"project_id":    "proj-grep-prox",
			"pattern":       "HIT_",
			"context_lines": 1,
			"format":        "json",
		})
		assert.Contains(t, out, `"context_after"`)
		assert.Contains(t, out, `"context_before"`)
	})
}

func TestMCPServer_GrepCode_BinarySkip(t *testing.T) {
	tempProjectDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempProjectDir, "code.go"), []byte("package main\n// BINARY_MARKER\n"), 0o644)
	// Cria um arquivo binário com bytes nulos
	binaryContent := []byte("BINARY_MARKER\x00\x00\x00some binary data")
	_ = os.WriteFile(filepath.Join(tempProjectDir, "data.bin"), binaryContent, 0o644)
	// Cria um arquivo com extensão binária
	_ = os.WriteFile(filepath.Join(tempProjectDir, "image.png"), []byte("BINARY_MARKER fake png"), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	_ = projectRepo.Create(&storage.Project{
		ID:       "proj-grep-bin",
		Name:     "Grep Binary Test",
		Path:     tempProjectDir,
		Language: "go",
		Status:   storage.StatusReady,
	})

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      14,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "grep_code",
			"arguments": map[string]any{
				"project_id": "proj-grep-bin",
				"pattern":    "BINARY_MARKER",
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	res, _ := resp["result"].(map[string]any)
	content, _ := res["content"].([]any)
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	// Deve conter apenas o match do arquivo .go, não dos binários
	assert.Contains(t, text, "code.go")
	assert.NotContains(t, text, "data.bin")
	assert.NotContains(t, text, "image.png")
}

func TestMCPServer_GrepCode_PracticalExamples(t *testing.T) {
	tempProjectDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempProjectDir, "server.go"), []byte("package main\n\n// Initialize server\nport := 9090\nstartServer(port)\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tempProjectDir, "config.yml"), []byte("server:\n  port: 9090\n  env: dev\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tempProjectDir, "readme.md"), []byte("# Server\nPORT=9090 by default\n"), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	_ = projectRepo.Create(&storage.Project{
		ID:       "proj-practical",
		Name:     "Practical Test",
		Path:     tempProjectDir,
		Language: "go",
		Status:   storage.StatusReady,
	})

	callGrep := func(args map[string]any) string {
		payload := map[string]any{
			"jsonrpc": "2.0",
			"id":      99,
			"method":  "tools/call",
			"params": map[string]any{
				"name":      "grep_code",
				"arguments": args,
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
		w := httptest.NewRecorder()
		server.HandleMessage()(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		res, _ := resp["result"].(map[string]any)
		content, _ := res["content"].([]any)
		item, _ := content[0].(map[string]any)
		return item["text"].(string)
	}

	// 1. Case-sensitive: "PORT" vs "port"
	t.Run("Practical_CaseSensitive", func(t *testing.T) {
		outInsensitive := callGrep(map[string]any{
			"project_id": "proj-practical",
			"pattern":    "port",
		})
		t.Logf("\n--- CASE INSENSITIVE ('port'): ---\n%s", outInsensitive)
		assert.Contains(t, outInsensitive, "server.go")
		assert.Contains(t, outInsensitive, "config.yml")
		assert.Contains(t, outInsensitive, "readme.md")

		outSensitive := callGrep(map[string]any{
			"project_id":     "proj-practical",
			"pattern":        "PORT",
			"case_sensitive": true,
		})
		t.Logf("\n--- CASE SENSITIVE ('PORT'): ---\n%s", outSensitive)
		assert.Contains(t, outSensitive, "readme.md")
		assert.NotContains(t, outSensitive, "server.go")
	})

	// 2. Filtro por extensão
	t.Run("Practical_FileExtensions", func(t *testing.T) {
		outYml := callGrep(map[string]any{
			"project_id":      "proj-practical",
			"pattern":         "9090",
			"file_extensions": []string{".yml"},
		})
		t.Logf("\n--- FILE EXTENSION FILTER ['.yml']: ---\n%s", outYml)
		assert.Contains(t, outYml, "config.yml")
		assert.NotContains(t, outYml, "server.go")
		assert.NotContains(t, outYml, "readme.md")
	})

	// 3. Linhas de contexto
	t.Run("Practical_ContextLines", func(t *testing.T) {
		outCtx := callGrep(map[string]any{
			"project_id":    "proj-practical",
			"pattern":       "port := 9090",
			"context_lines": 1,
		})
		t.Logf("\n--- CONTEXT LINES (1 before, 1 after): ---\n%s", outCtx)
		assert.Contains(t, outCtx, "server.go:3- // Initialize server")
		assert.Contains(t, outCtx, "server.go:4: port := 9090")
		assert.Contains(t, outCtx, "server.go:5- startServer(port)")
	})

	// 4. Formato JSON com paginação e contexto
	t.Run("Practical_JSONFormat", func(t *testing.T) {
		outJSON := callGrep(map[string]any{
			"project_id":    "proj-practical",
			"pattern":       "port := 9090",
			"context_lines": 1,
			"format":        "json",
		})
		t.Logf("\n--- JSON OUTPUT: ---\n%s", outJSON)
		assert.Contains(t, outJSON, `"context_before"`)
		assert.Contains(t, outJSON, `"context_after"`)
	})
}

func TestMCPServer_GetImplementationBundle(t *testing.T) {
	tempDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempDir, "calc.go"), []byte(`package calc

func Add(a, b int) int { return a + b }

func Subtract(a, b int) int { return a - b }
`), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	_ = projectRepo.Create(&storage.Project{
		ID:       "proj-bundle-1",
		Name:     "Bundle Test",
		Path:     tempDir,
		Language: "go",
		Status:   storage.StatusReady,
	})

	callBundle := func(args map[string]any) (map[string]any, string) {
		payload := map[string]any{
			"jsonrpc": "2.0",
			"id":      99,
			"method":  "tools/call",
			"params": map[string]any{
				"name":      "get_implementation_bundle",
				"arguments": args,
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.HandleMessage()(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		result := resp["result"].(map[string]any)
		content := result["content"].([]any)
		text := content[0].(map[string]any)["text"].(string)
		return result, text
	}

	t.Run("bundle with symbols", func(t *testing.T) {
		symbolsJSON := `[{"filepath":"calc.go","symbol_name":"Add"},{"filepath":"calc.go","symbol_name":"Subtract"}]`
		_, text := callBundle(map[string]any{
			"project_id": "proj-bundle-1",
			"symbols":    symbolsJSON,
		})
		t.Logf("Bundle text output:\n%s", text)
		// The engine reads the file directly; result should at least contain the symbol names in headers
		assert.Contains(t, text, "calc.go")
		assert.Contains(t, text, "Add")
	})

	t.Run("bundle with missing symbol returns partial result", func(t *testing.T) {
		symbolsJSON := `[{"filepath":"calc.go","symbol_name":"Add"},{"filepath":"calc.go","symbol_name":"DoesNotExist"}]`
		_, text := callBundle(map[string]any{
			"project_id": "proj-bundle-1",
			"symbols":    symbolsJSON,
		})
		t.Logf("Partial output:\n%s", text)
		// Should still have the bundle header (partial error tolerance)
		assert.Contains(t, text, "calc.go")
	})


	t.Run("bundle with json format", func(t *testing.T) {
		symbolsJSON := `[{"filepath":"calc.go","symbol_name":"Add"}]`
		_, text := callBundle(map[string]any{
			"project_id": "proj-bundle-1",
			"symbols":    symbolsJSON,
			"format":     "json",
		})
		t.Logf("JSON format output:\n%s", text)
		assert.Contains(t, text, `"results"`)
		assert.Contains(t, text, `"count"`)
	})
}

func TestMCPServer_LookupSymbol_Go_VarTaxonomy(t *testing.T) {
	tempProjectDir := t.TempDir()
	sourceCode := `package main

const AppVersion = "1.0.0"

var GlobalConfig = "active"
`
	_ = os.WriteFile(filepath.Join(tempProjectDir, "config.go"), []byte(sourceCode), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)

	proj := &storage.Project{
		ID:       "proj-go-tax",
		Name:     "Go Taxonomy Test",
		Path:     tempProjectDir,
		Language: "go",
		Status:   storage.StatusReady,
	}
	_ = projectRepo.Create(proj)

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)
	err := engine.IndexProject(proj.ID)
	require.NoError(t, err)

	callLookup := func(name string, format string) string {
		args := map[string]any{
			"project_id":  "proj-go-tax",
			"symbol_name": name,
			"mode":        "definition",
		}
		if format != "" {
			args["format"] = format
		}
		payload := map[string]any{
			"jsonrpc": "2.0",
			"id":      99,
			"method":  "tools/call",
			"params": map[string]any{
				"name":      "lookup_symbol",
				"arguments": args,
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
		w := httptest.NewRecorder()
		server.HandleMessage()(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		res, _ := resp["result"].(map[string]any)
		content, _ := res["content"].([]any)
		item, _ := content[0].(map[string]any)
		return item["text"].(string)
	}

	t.Run("const is classified as constant", func(t *testing.T) {
		text := callLookup("AppVersion", "text")
		assert.Contains(t, text, "[constant]")
		assert.Contains(t, text, "AppVersion")
		assert.NotContains(t, text, "[variable]")
	})

	t.Run("var is classified as variable not constant", func(t *testing.T) {
		text := callLookup("GlobalConfig", "text")
		assert.Contains(t, text, "[variable]")
		assert.Contains(t, text, "GlobalConfig")
		assert.NotContains(t, text, "[constant]")
	})

	t.Run("var JSON format has kind variable", func(t *testing.T) {
		text := callLookup("GlobalConfig", "json")
		assert.Contains(t, text, `"kind": "variable"`)
		assert.NotContains(t, text, `"kind": "constant"`)
	})
}

