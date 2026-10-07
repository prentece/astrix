package mcp_test

import (
	"astrix/internal/mcp"
	"astrix/internal/service"
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPServer_ProjectStatusWarnings(t *testing.T) {
	tempProjectDir1 := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempProjectDir1, "main.go"), []byte("package main\n\nfunc Main() {}\n"), 0o644)

	tempProjectDir2 := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempProjectDir2, "error.go"), []byte("package main\n"), 0o644)

	server, database := setupTestMCP(t)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)

	// 1. Projeto com StatusIndexing
	indexingProj := &storage.Project{
		ID:       "proj-indexing",
		Name:     "Indexing Project",
		Path:     tempProjectDir1,
		Language: "go",
		Status:   storage.StatusIndexing,
	}
	err := projectRepo.Create(indexingProj)
	require.NoError(t, err)

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "get_project_structure",
			"arguments": map[string]any{
				"project_id": "proj-indexing",
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
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	result, _ := resp["result"].(map[string]any)
	content, _ := result["content"].([]any)
	require.NotEmpty(t, content)
	first, _ := content[0].(map[string]any)
	text := first["text"].(string)
	assert.Contains(t, text, "[INDEXING")

	// 2. Projeto com StatusError (stale)
	errorProj := &storage.Project{
		ID:           "proj-error",
		Name:         "Error Project",
		Path:         tempProjectDir2,
		Language:     "go",
		Status:       storage.StatusError,
		ErrorMessage: "syntax error on line 42",
	}
	err = projectRepo.Create(errorProj)
	require.NoError(t, err)

	payload["params"] = map[string]any{
		"name": "get_project_structure",
		"arguments": map[string]any{
			"project_id": "proj-error",
		},
	}
	body, _ = json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	w = httptest.NewRecorder()
	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	result, _ = resp["result"].(map[string]any)
	content, _ = result["content"].([]any)
	require.NotEmpty(t, content)
	first, _ = content[0].(map[string]any)
	text = first["text"].(string)
	assert.Contains(t, text, "[STALE INDEX]")
	assert.Contains(t, text, "syntax error on line 42")
}

func TestMCPServer_GetFileOutline(t *testing.T) {
	tempProjectDir := t.TempDir()
	sourceCode := `package service

type UserService struct {
	repo string
}

func NewUserService(repo string) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetUser(id string) (string, error) {
	return "user-" + id, nil
}
`
	_ = os.WriteFile(filepath.Join(tempProjectDir, "service.go"), []byte(sourceCode), 0o644)

	tempDB := filepath.Join(t.TempDir(), "outline_test.db")
	database, err := storage.NewDatabase(tempDB)
	require.NoError(t, err)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)

	proj := &storage.Project{
		ID:       "proj-outline",
		Name:     "Outline Test Project",
		Path:     tempProjectDir,
		Language: "go",
		Status:   storage.StatusReady,
	}
	_ = projectRepo.Create(proj)

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)
	engine.SetIndexReplacer(storage.NewIndexStore(database))
	err = engine.IndexProject(proj.ID)
	require.NoError(t, err)

	projectService := serviceNewProjectService(projectRepo, symbolRepo, engine)
	codeService := serviceNewCodeService(projectRepo, symbolRepo, depRepo, dataModelRepo, engine)
	server := newServerWithVersion(projectService, codeService, "0.1.0")

	// 1. Formato texto padrão
	payloadText := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "get_file_outline",
			"arguments": map[string]any{
				"project_id": "proj-outline",
				"filepath":   "service.go",
			},
		},
	}
	body, _ := json.Marshal(payloadText)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	result, _ := resp["result"].(map[string]any)
	content, _ := result["content"].([]any)
	require.NotEmpty(t, content)
	first, _ := content[0].(map[string]any)
	text := first["text"].(string)

	assert.Contains(t, text, "Outline for service.go")
	assert.Contains(t, text, "UserService")
	assert.Contains(t, text, "NewUserService")
	assert.Contains(t, text, "GetUser")

	// 2. Formato JSON
	payloadJSON := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "get_file_outline",
			"arguments": map[string]any{
				"project_id": "proj-outline",
				"path":       "service.go", // Testa alias 'path'
				"format":     "json",
			},
		},
	}
	body, _ = json.Marshal(payloadJSON)
	req = httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	w = httptest.NewRecorder()
	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var respJSON map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &respJSON)
	require.NoError(t, err)
	resultJSON, _ := respJSON["result"].(map[string]any)
	contentJSON, _ := resultJSON["content"].([]any)
	require.NotEmpty(t, contentJSON)
	firstJSON, _ := contentJSON[0].(map[string]any)
	rawJSON := firstJSON["text"].(string)

	var outlineParsed struct {
		Filepath string `json:"filepath"`
		Symbols  []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
			Line int    `json:"line"`
		} `json:"symbols"`
	}
	err = json.Unmarshal([]byte(rawJSON), &outlineParsed)
	require.NoError(t, err)
	assert.Equal(t, "service.go", outlineParsed.Filepath)
	assert.GreaterOrEqual(t, len(outlineParsed.Symbols), 3)
}

func TestMCPServer_GetImplementationBundle_NativeArray(t *testing.T) {
	tempProjectDir := t.TempDir()
	sourceCode := `package mathutil

func Add(a, b int) int {
	return a + b
}

func Subtract(a, b int) int {
	return a - b
}
`
	_ = os.WriteFile(filepath.Join(tempProjectDir, "calc.go"), []byte(sourceCode), 0o644)

	tempDB := filepath.Join(t.TempDir(), "bundle_test.db")
	database, err := storage.NewDatabase(tempDB)
	require.NoError(t, err)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)

	proj := &storage.Project{
		ID:       "proj-bundle-native",
		Name:     "Bundle Native Test",
		Path:     tempProjectDir,
		Language: "go",
		Status:   storage.StatusReady,
	}
	_ = projectRepo.Create(proj)

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)
	engine.SetIndexReplacer(storage.NewIndexStore(database))
	err = engine.IndexProject(proj.ID)
	require.NoError(t, err)

	projectService := serviceNewProjectService(projectRepo, symbolRepo, engine)
	codeService := serviceNewCodeService(projectRepo, symbolRepo, depRepo, dataModelRepo, engine)
	server := newServerWithVersion(projectService, codeService, "0.1.0")

	// Chama com array nativo em vez de string JSON serializada
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "get_implementation_bundle",
			"arguments": map[string]any{
				"project_id": "proj-bundle-native",
				"symbols": []any{
					map[string]any{"filepath": "calc.go", "symbol_name": "Add"},
					map[string]any{"filepath": "calc.go", "symbol_name": "Subtract"},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.HandleMessage()(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	result, _ := resp["result"].(map[string]any)
	content, _ := result["content"].([]any)
	require.NotEmpty(t, content)
	first, _ := content[0].(map[string]any)
	text := first["text"].(string)

	assert.Contains(t, text, "[Bundle: 2/2 symbols retrieved]")
	assert.Contains(t, text, "func Add(a, b int) int")
	assert.Contains(t, text, "func Subtract(a, b int) int")
}

func serviceNewProjectService(repo storage.ProjectRepository, sym storage.SymbolRepository, engine *indexer.Engine) *service.ProjectService {
	return service.NewProjectService(repo, sym, engine)
}

func serviceNewCodeService(repo storage.ProjectRepository, sym storage.SymbolRepository, dep storage.DependencyGraphRepository, dm storage.DataModelRepository, engine *indexer.Engine) *service.CodeService {
	return service.NewCodeService(repo, sym, dep, dm, engine)
}

func newServerWithVersion(p *service.ProjectService, c *service.CodeService, v string) *mcp.Server {
	return mcp.NewServer(p, c, v)
}

func TestMCPServer_SafeJoinProtection(t *testing.T) {
	// Cria raiz pai com subdiretório de projeto e arquivo externo confidencial
	baseDir := t.TempDir()
	outsideSecret := filepath.Join(baseDir, "outside_secret.txt")
	err := os.WriteFile(outsideSecret, []byte("SUPER_SECRET_TOKEN=12345\n"), 0o644)
	require.NoError(t, err)

	projectDir := filepath.Join(baseDir, "myproject")
	err = os.Mkdir(projectDir, 0o755)
	require.NoError(t, err)

	// Arquivo legítimo dentro do projeto
	err = os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main\n\nfunc Run() {}\n"), 0o644)
	require.NoError(t, err)

	// Symlink dentro do projeto apontando para o arquivo externo (pode falhar no Windows sem permissões)
	symlinkPath := filepath.Join(projectDir, "symlink_secret.txt")
	symlinkAvailable := true
	if err := os.Symlink(outsideSecret, symlinkPath); err != nil {
		symlinkAvailable = false
		t.Logf("symlink indisponível no ambiente de teste (ex: Windows sem privilégios): %v", err)
	}

	tempDB := t.TempDir() + "/safejoin_test.db"
	database, err := storage.NewDatabase(tempDB)
	require.NoError(t, err)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)

	proj := &storage.Project{
		ID:       "proj-safejoin",
		Name:     "SafeJoin Test",
		Path:     projectDir,
		Language: "go",
		Status:   storage.StatusReady,
	}
	err = projectRepo.Create(proj)
	require.NoError(t, err)

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)
	engine.SetIndexReplacer(storage.NewIndexStore(database))
	err = engine.IndexProject(proj.ID)
	require.NoError(t, err)

	projectService := serviceNewProjectService(projectRepo, symbolRepo, engine)
	codeService := serviceNewCodeService(projectRepo, symbolRepo, depRepo, dataModelRepo, engine)
	mcpServer := newServerWithVersion(projectService, codeService, "0.1.0")

	// 1. Testar read_file_lines com path traversal lexical ("../outside_secret.txt")
	callTool := func(toolName string, args map[string]any) map[string]any {
		payload := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]any{
				"name":      toolName,
				"arguments": args,
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewReader(body))
		w := httptest.NewRecorder()
		mcpServer.HandleMessage()(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		jsonErr := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, jsonErr)
		result, ok := resp["result"].(map[string]any)
		require.True(t, ok, "resposta deve conter result")
		return result
	}

	// Traversal lexical via read_file_lines
	res := callTool("read_file_lines", map[string]any{
		"project_id": "proj-safejoin",
		"filepath":   "../outside_secret.txt",
	})
	assert.Equal(t, true, res["isError"], "deve retornar isError=true")
	contentList, _ := res["content"].([]any)
	require.NotEmpty(t, contentList)
	firstItem := contentList[0].(map[string]any)
	errorText := firstItem["text"].(string)
	assert.Contains(t, errorText, "caminho fora da raiz do projeto")
	assert.NotContains(t, errorText, "SUPER_SECRET_TOKEN")

	// Symlink escapista via read_file_lines
	if symlinkAvailable {
		resSymlink := callTool("read_file_lines", map[string]any{
			"project_id": "proj-safejoin",
			"filepath":   "symlink_secret.txt",
		})
		assert.Equal(t, true, resSymlink["isError"], "deve retornar isError=true para symlink externo")
		contentListSym, _ := resSymlink["content"].([]any)
		require.NotEmpty(t, contentListSym)
		firstItemSym := contentListSym[0].(map[string]any)
		errorTextSym := firstItemSym["text"].(string)
		assert.Contains(t, errorTextSym, "caminho fora da raiz do projeto")
		assert.NotContains(t, errorTextSym, "SUPER_SECRET_TOKEN")
	}

	// Traversal lexical via get_implementation
	resImpl := callTool("get_implementation", map[string]any{
		"project_id":  "proj-safejoin",
		"filepath":    "../outside_secret.txt",
		"symbol_name": "AnySymbol",
	})
	assert.Equal(t, true, resImpl["isError"], "deve retornar isError=true no get_implementation com traversal")

	// Leitura legítima dentro do projeto deve funcionar normalmente
	resLegit := callTool("read_file_lines", map[string]any{
		"project_id": "proj-safejoin",
		"filepath":   "main.go",
	})
	assert.NotEqual(t, true, resLegit["isError"], "leitura de arquivo legítimo não deve falhar")
	contentListLegit, _ := resLegit["content"].([]any)
	require.NotEmpty(t, contentListLegit)
	firstItemLegit := contentListLegit[0].(map[string]any)
	assert.Contains(t, firstItemLegit["text"].(string), "func Run()")
}
