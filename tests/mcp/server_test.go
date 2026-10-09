package mcp_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPServer_InitializeIdentity(t *testing.T) {
	server, database := setupTestMCP(t)
	defer database.Close()

	// JSON-RPC Initialize request
	initPayload := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"clientInfo": {
				"name": "test-client",
				"version": "1.0.0"
			}
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/mcp/message", bytes.NewBufferString(initPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "antigravity-ide")
	w := httptest.NewRecorder()

	server.HandleMessage()(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ServerInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
	}

	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "astrix", resp.Result.ServerInfo.Name)
	assert.Equal(t, "0.2.1", resp.Result.ServerInfo.Version)
}
