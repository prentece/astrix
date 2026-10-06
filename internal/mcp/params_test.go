package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeToolHandler_RecoversPanic(t *testing.T) {
	panicHandler := safeToolHandler(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		panic("simulated fatal tree-sitter crash")
	})

	req := mcp.CallToolRequest{}
	res, err := panicHandler(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError)
	require.NotEmpty(t, res.Content)

	textContent, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "Internal error: unexpected panic recovered: simulated fatal tree-sitter crash")
}

func TestCheckProjectWarning(t *testing.T) {
	// StatusReady: sem warning
	warn := checkProjectWarning(nil, "")
	assert.Empty(t, warn)

	// Mock manual via prependWarning
	assert.Equal(t, "content", prependWarning("content", ""))
	assert.Equal(t, "[WARN]\n\ncontent", prependWarning("content", "[WARN]"))
}

func TestGetFilePathParam_Aliases(t *testing.T) {
	// 1. Testa chave canônica 'filepath'
	args1 := map[string]any{"filepath": "pkg/indexer/engine.go"}
	assert.Equal(t, "pkg/indexer/engine.go", getFilePathParam(args1))

	// 2. Testa alias 'path'
	args2 := map[string]any{"path": "pkg/indexer/walker.go"}
	assert.Equal(t, "pkg/indexer/walker.go", getFilePathParam(args2))

	// 3. 'filepath' tem precedência sobre 'path' se ambos existirem
	args3 := map[string]any{"filepath": "primary.go", "path": "secondary.go"}
	assert.Equal(t, "primary.go", getFilePathParam(args3))

	// 4. Nenhum fornecido
	args4 := map[string]any{}
	assert.Equal(t, "", getFilePathParam(args4))
}

func TestGetStringSliceParam_Formats(t *testing.T) {
	// 1. Array de strings
	args1 := map[string]any{"items": []string{"a", "b", "c"}}
	assert.Equal(t, []string{"a", "b", "c"}, getStringSliceParam(args1, "items"))

	// 2. Array de any ([]interface{})
	args2 := map[string]any{"items": []any{"x", "y"}}
	assert.Equal(t, []string{"x", "y"}, getStringSliceParam(args2, "items"))

	// 3. String separada por vírgula
	args3 := map[string]any{"items": "foo, bar, baz"}
	assert.Equal(t, []string{"foo", "bar", "baz"}, getStringSliceParam(args3, "items"))
}
