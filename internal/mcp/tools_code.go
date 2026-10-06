package mcp

import (
	"astrix/pkg/indexer"
	"astrix/internal/service"
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerCodeTools registra as ferramentas de inteligência de código, AST e navegação.
func registerCodeTools(s *server.MCPServer, codeService *service.CodeService) {
	// 1. Tool: lookup_symbol (merge de find_symbol + find_references)
	lookupSymbolTool := mcp.NewTool("lookup_symbol",
		mcp.WithDescription("AST symbol lookup and impact analysis. Use mode='definition' to locate declarations of classes, structs, interfaces, methods, functions across the repository. Use mode='references' to find callers and usage sites."),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("The ID of the project. Read it from '.astrix/config.json' (field 'id') for the current project; for any other project, call list_projects.")),
		mcp.WithString("symbol_name", mcp.Required(), mcp.Description("The exact or partial name of the symbol (e.g. 'NewServer', 'UserService', 'HandleRequest').")),
		mcp.WithString("mode", mcp.Required(), mcp.Description("Lookup mode: 'definition' (find where the symbol is declared/implemented) or 'references' (find all callers and usage sites across the repo).")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of results to return per page (optional, default 25, max 100).")),
		mcp.WithNumber("offset", mcp.Description("Starting offset index for pagination (optional, default 0).")),
		mcp.WithString("format", mcp.Description("Output format: 'text' (default, compact grep/ctags style) or 'json' (clean minimized JSON array).")),
	)
	s.AddTool(lookupSymbolTool, safeToolHandler(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID := getStringParam(req.Params.Arguments, "project_id")
		if projectID == "" {
			return mcp.NewToolResultError("Field 'project_id' is required"), nil
		}

		symbolName := getStringParam(req.Params.Arguments, "symbol_name")
		if symbolName == "" {
			return mcp.NewToolResultError("Field 'symbol_name' is required"), nil
		}

		mode := getStringParam(req.Params.Arguments, "mode")
		if mode == "" {
			return mcp.NewToolResultError("Field 'mode' is required: use 'definition' or 'references'"), nil
		}
		if mode != "definition" && mode != "references" {
			return mcp.NewToolResultError("Field 'mode' must be 'definition' or 'references'"), nil
		}

		limit := getIntParam(req.Params.Arguments, "limit", 25)
		if limit > 100 {
			limit = 100
		}

		offset := getIntParam(req.Params.Arguments, "offset", 0)
		format := getStringParam(req.Params.Arguments, "format")
		asJSON := format == "json"
		warn := checkProjectWarning(codeService, projectID)

		if mode == "definition" {
			symbols, hasMore, err := codeService.FindSymbol(projectID, symbolName, limit, offset)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to find symbol definition: %v", err)), nil
			}
			return mcp.NewToolResultText(prependWarning(FormatSymbols(symbols, asJSON, limit, offset, hasMore), warn)), nil
		}

		// mode == "references"
		refs, hasMore, err := codeService.FindReferences(projectID, symbolName, limit, offset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to find references: %v", err)), nil
		}
		result := FormatReferences(symbolName, refs, asJSON, limit, offset, hasMore)
		if len(refs) == 0 {
			result += "\n[HINT] No references found via AST index. For class-level symbols (types, interfaces), try grep_code as fallback: grep_code(pattern=\"<SymbolName>\")."
		}
		return mcp.NewToolResultText(prependWarning(result, warn)), nil
	}))

	// 2. Tool: get_implementation
	getImplementationTool := mcp.NewTool("get_implementation",
		mcp.WithDescription("Extracts the AST source code block of a function, method, struct, or class definition."),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("The ID of the project. Read it from '.astrix/config.json' (field 'id') for the current project; for any other project, call list_projects.")),
		mcp.WithString("filepath", mcp.Required(), mcp.Description("Relative filepath where the symbol is declared (e.g. 'cmd/server/main.go').")),
		mcp.WithString("path", mcp.Description("Alternative alias for filepath.")),
		mcp.WithString("symbol_name", mcp.Required(), mcp.Description("The name of the symbol whose implementation you want to retrieve.")),
		mcp.WithString("format", mcp.Description("Output format: 'text' (default, raw code block) or 'json'.")),
	)
	s.AddTool(getImplementationTool, safeToolHandler(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID := getStringParam(req.Params.Arguments, "project_id")
		if projectID == "" {
			return mcp.NewToolResultError("Field 'project_id' is required"), nil
		}

		filePath := getFilePathParam(req.Params.Arguments)
		if filePath == "" {
			return mcp.NewToolResultError("Field 'filepath' is required"), nil
		}

		symbolName := getStringParam(req.Params.Arguments, "symbol_name")
		if symbolName == "" {
			return mcp.NewToolResultError("Field 'symbol_name' is required"), nil
		}

		format := getStringParam(req.Params.Arguments, "format")
		asJSON := format == "json"

		impl, err := codeService.GetImplementation(projectID, filePath, symbolName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get implementation: %v", err)), nil
		}

		warn := checkProjectWarning(codeService, projectID)
		return mcp.NewToolResultText(prependWarning(FormatImplementation(symbolName, filePath, impl, asJSON), warn)), nil
	}))

	// 3. Tool: read_file_lines
	readFileLinesTool := mcp.NewTool("read_file_lines",
		mcp.WithDescription("Reads and inspects code or text files within a specified line window, with enclosing symbol context and optional symbol anchor."),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("The ID of the project. Read it from '.astrix/config.json' (field 'id') for the current project; for any other project, call list_projects.")),
		mcp.WithString("filepath", mcp.Required(), mcp.Description("Relative filepath to read (e.g. 'internal/web/handler.go').")),
		mcp.WithString("path", mcp.Description("Alternative alias for filepath.")),
		mcp.WithNumber("start_line", mcp.Description("1-indexed starting line number (optional, default 1). Ignored when anchor_symbol is provided.")),
		mcp.WithNumber("end_line", mcp.Description("1-indexed ending line number (optional, default start_line + 49, max window 100 lines). Ignored when anchor_symbol is provided.")),
		mcp.WithString("anchor_symbol", mcp.Description("Optional symbol name (function, class, struct) to auto-position the read window on its definition.")),
		mcp.WithString("format", mcp.Description("Output format: 'text' (default, line window with context) or 'json'.")),
	)
	s.AddTool(readFileLinesTool, safeToolHandler(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID := getStringParam(req.Params.Arguments, "project_id")
		if projectID == "" {
			return mcp.NewToolResultError("Field 'project_id' is required"), nil
		}

		filePath := getFilePathParam(req.Params.Arguments)
		if filePath == "" {
			return mcp.NewToolResultError("Field 'filepath' is required"), nil
		}

		startLine := getIntParam(req.Params.Arguments, "start_line", 1)
		endLine := getIntParam(req.Params.Arguments, "end_line", 0)
		anchorSymbol := getStringParam(req.Params.Arguments, "anchor_symbol")
		format := getStringParam(req.Params.Arguments, "format")
		asJSON := format == "json"

		content, err := codeService.PeekFile(projectID, filePath, startLine, endLine, anchorSymbol)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read file lines: %v", err)), nil
		}

		warn := checkProjectWarning(codeService, projectID)
		return mcp.NewToolResultText(prependWarning(FormatPeekFile(filePath, startLine, endLine, anchorSymbol, content, asJSON), warn)), nil
	}))

	// 4. Tool: grep_code
	grepTool := mcp.NewTool("grep_code",
		mcp.WithDescription("Search for strings, patterns, or regex across the project respecting .gitignore. Supports case sensitivity, file extension filtering, and context lines."),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("The ID of the project. Read it from '.astrix/config.json' (field 'id') for the current project; for any other project, call list_projects.")),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Regular expression or literal text string to search for across files.")),
		mcp.WithString("path_prefix", mcp.Description("Optional subfolder path to limit the search scope (e.g. 'internal/web').")),
		mcp.WithNumber("max_results", mcp.Description("Maximum number of matching lines to return per page (optional, default 30, max 100).")),
		mcp.WithNumber("offset", mcp.Description("Starting match offset index for pagination (optional, default 0).")),
		mcp.WithString("format", mcp.Description("Output format: 'text' (default, grep format) or 'json'.")),
		mcp.WithBoolean("case_sensitive", mcp.Description("If true, search is case-sensitive. Default is false (case-insensitive).")),
		mcp.WithString("file_extensions", mcp.Description("Comma-separated list of file extensions to include (e.g. '.go,.ts,.py'). If empty, searches all text files.")),
		mcp.WithNumber("context_lines", mcp.Description("Number of lines of context to include before and after each match (0-5, default 0).")),
	)
	s.AddTool(grepTool, safeToolHandler(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID := getStringParam(req.Params.Arguments, "project_id")
		if projectID == "" {
			return mcp.NewToolResultError("Field 'project_id' is required"), nil
		}

		pattern := getStringParam(req.Params.Arguments, "pattern")
		if pattern == "" {
			return mcp.NewToolResultError("Field 'pattern' is required"), nil
		}

		pathPrefix := getStringParam(req.Params.Arguments, "path_prefix")
		maxResults := getIntParam(req.Params.Arguments, "max_results", 30)
		if maxResults > 100 {
			maxResults = 100
		}

		offset := getIntParam(req.Params.Arguments, "offset", 0)
		format := getStringParam(req.Params.Arguments, "format")
		asJSON := format == "json"

		caseSensitive := getBoolParam(req.Params.Arguments, "case_sensitive", false)
		contextLines := getIntParam(req.Params.Arguments, "context_lines", 0)
		includeExts := getStringSliceParam(req.Params.Arguments, "file_extensions")

		opts := indexer.GrepOptions{
			CaseSensitive: caseSensitive,
			IncludeExts:   includeExts,
			ContextLines:  contextLines,
		}

		matches, hasMore, err := codeService.GrepCode(ctx, projectID, pattern, pathPrefix, maxResults, offset, opts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to grep code: %v", err)), nil
		}

		warn := checkProjectWarning(codeService, projectID)
		return mcp.NewToolResultText(prependWarning(FormatGrepMatches(pattern, matches, asJSON, maxResults, offset, hasMore), warn)), nil
	}))

	// 5. Tool: query_structured_file
	queryStructuredTool := mcp.NewTool("query_structured_file",
		mcp.WithDescription("Inspect and query specific nodes in structured files (JSON, YAML, CSV). For JSON uses GJSON path (e.g. 'dependencies.@nestjs/core'), for YAML uses dot notation (e.g. 'services.postgres.ports'), for CSV uses filter expressions."),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("The ID of the project. Read it from '.astrix/config.json' (field 'id') for the current project; for any other project, call list_projects.")),
		mcp.WithString("filepath", mcp.Required(), mcp.Description("Relative filepath to the structured file (e.g. 'package.json', 'docker-compose.yml', 'data.csv').")),
		mcp.WithString("path", mcp.Description("Alternative alias for filepath.")),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search path / filter query expression (JSON: GJSON path, YAML: dot-path, CSV: filter or columns).")),
		mcp.WithString("format", mcp.Description("Output format: 'text' (default) or 'json'.")),
	)
	s.AddTool(queryStructuredTool, safeToolHandler(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID := getStringParam(req.Params.Arguments, "project_id")
		if projectID == "" {
			return mcp.NewToolResultError("Field 'project_id' is required"), nil
		}

		filePath := getFilePathParam(req.Params.Arguments)
		if filePath == "" {
			return mcp.NewToolResultError("Field 'filepath' is required"), nil
		}

		query := getStringParam(req.Params.Arguments, "query")
		if query == "" {
			return mcp.NewToolResultError("Field 'query' is required"), nil
		}

		format := getStringParam(req.Params.Arguments, "format")
		asJSON := format == "json"

		res, err := codeService.QueryStructuredFile(projectID, filePath, query)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to query structured file: %v", err)), nil
		}

		warn := checkProjectWarning(codeService, projectID)
		return mcp.NewToolResultText(prependWarning(FormatStructuredFileResult(filePath, query, res, asJSON), warn)), nil
	}))

	// 6. Tool: get_implementation_bundle
	bundleTool := mcp.NewTool("get_implementation_bundle",
		mcp.WithDescription("Fetches AST implementations of multiple symbols in a single call. Returns all results with partial error reporting for missing symbols."),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("The ID of the project. Read it from '.astrix/config.json' (field 'id') for the current project; for any other project, call list_projects.")),
		WithArray("symbols", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filepath":    map[string]any{"type": "string", "description": "Relative path to file"},
				"symbol_name": map[string]any{"type": "string", "description": "Symbol name"},
			},
			"required": []string{"filepath", "symbol_name"},
		}, "Array of symbol requests or stringified JSON array.", true),
		mcp.WithString("format", mcp.Description("Output format: 'text' (default, sections separated by ### filepath :: symbol) or 'json'.")),
	)
	s.AddTool(bundleTool, safeToolHandler(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID := getStringParam(req.Params.Arguments, "project_id")
		if projectID == "" {
			return mcp.NewToolResultError("Field 'project_id' is required"), nil
		}

		var rawItems []map[string]any
		if rawArray, ok := req.Params.Arguments["symbols"].([]any); ok {
			for _, elem := range rawArray {
				if m, ok := elem.(map[string]any); ok {
					rawItems = append(rawItems, m)
				}
			}
		} else if rawStr := getStringParam(req.Params.Arguments, "symbols"); rawStr != "" {
			if err := parseJSON(rawStr, &rawItems); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Field 'symbols' must be a valid JSON array: %v", err)), nil
			}
		} else {
			return mcp.NewToolResultError("Field 'symbols' is required (must be an array of objects or JSON string)"), nil
		}

		reqs := make([]service.BundleRequest, 0, len(rawItems))
		for _, item := range rawItems {
			fp, _ := item["filepath"].(string)
			if fp == "" {
				fp, _ = item["path"].(string)
			}
			sn, _ := item["symbol_name"].(string)
			reqs = append(reqs, service.BundleRequest{Filepath: fp, SymbolName: sn})
		}

		format := getStringParam(req.Params.Arguments, "format")
		asJSON := format == "json"

		results, err := codeService.GetImplementationBundle(projectID, reqs)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Bundle failed: %v", err)), nil
		}

		warn := checkProjectWarning(codeService, projectID)
		return mcp.NewToolResultText(prependWarning(FormatImplementationBundle(results, asJSON), warn)), nil
	}))

	// 7. Tool: get_file_outline
	fileOutlineTool := mcp.NewTool("get_file_outline",
		mcp.WithDescription("Returns an outline of all declarations (classes, functions, methods, interfaces, structs) in a file with line numbers and signatures, without returning full code bodies."),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("The ID of the project. Read it from '.astrix/config.json' (field 'id') for the current project; for any other project, call list_projects.")),
		mcp.WithString("filepath", mcp.Required(), mcp.Description("Relative filepath within the project (e.g. 'internal/mcp/server.go').")),
		mcp.WithString("path", mcp.Description("Alternative alias for filepath.")),
		mcp.WithString("format", mcp.Description("Output format: 'text' (default, indented outline) or 'json'.")),
	)
	s.AddTool(fileOutlineTool, safeToolHandler(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID := getStringParam(req.Params.Arguments, "project_id")
		if projectID == "" {
			return mcp.NewToolResultError("Field 'project_id' is required"), nil
		}

		filePath := getFilePathParam(req.Params.Arguments)
		if filePath == "" {
			return mcp.NewToolResultError("Field 'filepath' is required"), nil
		}

		format := getStringParam(req.Params.Arguments, "format")
		asJSON := format == "json"

		syms, err := codeService.GetSymbolsByFile(projectID, filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get file outline: %v", err)), nil
		}

		warn := checkProjectWarning(codeService, projectID)
		return mcp.NewToolResultText(prependWarning(FormatFileOutline(filePath, syms, asJSON), warn)), nil
	}))
}
