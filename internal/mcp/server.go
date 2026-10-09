package mcp

import (
	"astrix/internal/service"
	"context"
	"encoding/json"
	"net/http"

	"github.com/mark3labs/mcp-go/server"
)

// Server encapsula o servidor Model Context Protocol (MCP) com transporte STDIO nativo.
// Astrix: High-performance, AST-driven source code intelligence and navigation engine.
type Server struct {
	mcpServer      *server.MCPServer
	projectService *service.ProjectService
	codeService    *service.CodeService
}

// NewServer inicializa o servidor MCP e registra todos os módulos de ferramentas.
func NewServer(
	projectService *service.ProjectService,
	codeService *service.CodeService,
	version ...string,
) *Server {
	ver := "0.2.2"
	if len(version) > 0 && version[0] != "" {
		ver = version[0]
	}
	mcpServer := server.NewMCPServer("astrix", ver)

	s := &Server{
		mcpServer:      mcpServer,
		projectService: projectService,
		codeService:    codeService,
	}

	s.registerAllTools()
	return s
}

// ServeStdio inicia o servidor MCP no fluxo padrão stdin/stdout (port-free, zero conflitos de porta).
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcpServer)
}

// HandleMessage retorna um handler HTTP utilitário para mensagens JSON-RPC (usado em testes automatizados).
func (s *Server) HandleMessage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var rawMessage json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&rawMessage); err != nil {
			http.Error(w, "Invalid JSON-RPC payload", http.StatusBadRequest)
			return
		}

		response := s.mcpServer.HandleMessage(r.Context(), rawMessage)

		w.Header().Set("Content-Type", "application/json")
		if response != nil {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
		} else {
			w.WriteHeader(http.StatusAccepted)
		}
	}
}

// HandleJSONRPCMessage processa diretamente uma mensagem JSON-RPC em memória.
func (s *Server) HandleJSONRPCMessage(ctx context.Context, rawMessage json.RawMessage) any {
	return s.mcpServer.HandleMessage(ctx, rawMessage)
}

// registerAllTools registra os módulos de ferramentas modulares no servidor MCP.
func (s *Server) registerAllTools() {
	registerProjectTools(s.mcpServer, s.projectService, s.codeService)
	registerCodeTools(s.mcpServer, s.codeService)
}
