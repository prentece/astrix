package mcp

import (
	"astrix/internal/service"
	"astrix/pkg/storage"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// getStringParam extrai estritamente uma string da chave informada.
func getStringParam(args map[string]any, key string) string {
	if val, ok := args[key].(string); ok {
		return strings.TrimSpace(val)
	}
	return ""
}

// getIntParam extrai estritamente um número inteiro da chave informada.
func getIntParam(args map[string]any, key string, defaultVal int) int {
	if val, ok := args[key]; ok {
		if f, ok := val.(float64); ok && f >= 0 {
			return int(f)
		}
		if i, ok := val.(int); ok && i >= 0 {
			return i
		}
	}
	return defaultVal
}

// getBoolParam extrai um booleano da chave informada com valor default.
func getBoolParam(args map[string]any, key string, defaultVal bool) bool {
	if val, ok := args[key].(bool); ok {
		return val
	}
	return defaultVal
}

// getStringSliceParam extrai um slice de strings, suportando tanto array JSON ([]any / []string) quanto string separada por vírgula.
func getStringSliceParam(args map[string]any, key string) []string {
	val, ok := args[key]
	if !ok || val == nil {
		return nil
	}
	var res []string
	switch v := val.(type) {
	case string:
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				res = append(res, s)
			}
		}
	case []string:
		for _, s := range v {
			s = strings.TrimSpace(s)
			if s != "" {
				res = append(res, s)
			}
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					res = append(res, s)
				}
			}
		}
	}
	return res
}

// parseJSON desserializa uma string JSON para o destino informado.
func parseJSON(raw string, dest any) error {
	return json.Unmarshal([]byte(raw), dest)
}

// getFilePathParam extrai o caminho do arquivo aceitando tanto 'filepath' quanto o alias 'path'.
func getFilePathParam(args map[string]any) string {
	fp := getStringParam(args, "filepath")
	if fp == "" {
		fp = getStringParam(args, "path")
	}
	return fp
}

// safeToolHandler envolve um handler de tool em um bloco recover para que qualquer panic
// não derrube o processo MCP e a conexão STDIO com o cliente.
func safeToolHandler(fn func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (res *mcp.CallToolResult, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[MCP TOOL PANIC] Recuperado de panic na tool: %v\n%s\n", r, string(debug.Stack()))
				res = mcp.NewToolResultError(fmt.Sprintf("Internal error: unexpected panic recovered: %v", r))
				err = nil
			}
		}()
		return fn(ctx, req)
	}
}

// checkProjectWarning retorna um alerta descritivo caso o projeto esteja sendo indexado, pendente ou em erro.
func checkProjectWarning(codeService *service.CodeService, projectID string) string {
	if codeService == nil || projectID == "" {
		return ""
	}
	proj, prog, err := codeService.GetProjectStatus(projectID)
	if err != nil || proj == nil {
		return ""
	}
	if proj.Status == storage.StatusIndexing {
		if prog != nil && prog.IsIndexing && prog.TotalFiles > 0 {
			return fmt.Sprintf("[INDEXING %d%% (%d/%d arquivos)] Este projeto está sendo indexado. Os resultados podem estar parciais.", prog.Percent, prog.ProcessedFiles, prog.TotalFiles)
		}
		return "[INDEXING] Este projeto está em processo de indexação. Os resultados podem estar parciais."
	}
	if proj.Status == storage.StatusPending {
		return "[PENDING] A indexação deste projeto ainda não foi concluída. Os resultados podem estar incompletos."
	}
	if proj.Status == storage.StatusError {
		errMsg := proj.ErrorMessage
		if errMsg == "" {
			errMsg = "falha desconhecida"
		}
		return fmt.Sprintf("[STALE INDEX] Erro na última indexação: %s. Os dados podem estar desatualizados.", errMsg)
	}
	return ""
}

// prependWarning prefixa o aviso de estado ao texto retornado caso exista.
func prependWarning(text, warning string) string {
	if warning == "" {
		return text
	}
	return warning + "\n\n" + text
}

// WithArray define um schema de array com itemSchema no InputSchema da ferramenta.
func WithArray(name string, itemSchema map[string]any, description string, required bool) mcp.ToolOption {
	return func(t *mcp.Tool) {
		if t.InputSchema.Properties == nil {
			t.InputSchema.Properties = make(map[string]interface{})
		}
		t.InputSchema.Properties[name] = map[string]interface{}{
			"type":        "array",
			"items":       itemSchema,
			"description": description,
		}
		if required {
			t.InputSchema.Required = append(t.InputSchema.Required, name)
		}
	}
}


