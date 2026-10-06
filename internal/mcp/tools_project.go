package mcp

import (
	"astrix/internal/service"
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerProjectTools registra as ferramentas de descoberta e topologia de projetos.
func registerProjectTools(s *server.MCPServer, projectService *service.ProjectService, codeService *service.CodeService) {
	// Tool: list_projects
	listProjectsTool := mcp.NewTool("list_projects",
		mcp.WithDescription("Discover all registered repositories in the Astrix catalog and their 'project_id', name, language and path."),
		mcp.WithString("format", mcp.Description("Output format: 'text' (default tabular ASCII) or 'json'.")),
	)
	s.AddTool(listProjectsTool, safeToolHandler(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projects, err := projectService.ListAll()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list projects: %v", err)), nil
		}
		format := getStringParam(req.Params.Arguments, "format")
		asJSON := format == "json"
		return mcp.NewToolResultText(FormatProjects(projects, asJSON)), nil
	}))

	// Tool: get_project_structure
	getStructureTool := mcp.NewTool("get_project_structure",
		mcp.WithDescription("Inspect project directory layout and file tree in text format. Respects .gitignore and excludes build/dependency dirs by default."),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("The ID of the project. Read it from '.astrix/config.json' (field 'id') for the current project; for any other project, call list_projects.")),
		mcp.WithString("path", mcp.Description("Relative subfolder path to explore (optional, defaults to project root '').")),
		mcp.WithNumber("depth", mcp.Description("Maximum recursion depth from the specified path (optional, default: 2).")),
		mcp.WithBoolean("show_hidden", mcp.Description("Whether to show hidden files and folders starting with '.' (optional, default: false).")),
		mcp.WithString("format", mcp.Description("Output format: 'text' (default, compact tree) or 'json'.")),
	)
	s.AddTool(getStructureTool, safeToolHandler(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID := getStringParam(req.Params.Arguments, "project_id")
		if projectID == "" {
			return mcp.NewToolResultError("Field 'project_id' is required"), nil
		}

		subPath := getStringParam(req.Params.Arguments, "path")
		depth := getIntParam(req.Params.Arguments, "depth", 2)
		showHidden, _ := req.Params.Arguments["show_hidden"].(bool)
		format := getStringParam(req.Params.Arguments, "format")
		asJSON := format == "json"

		treeText, err := codeService.GetProjectStructureTree(projectID, subPath, depth, showHidden)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to build directory tree: %v", err)), nil
		}

		output := FormatStructureTree(treeText, asJSON)
		warn := checkProjectWarning(codeService, projectID)
		return mcp.NewToolResultText(prependWarning(output, warn)), nil
	}))
}

