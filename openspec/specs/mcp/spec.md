# MCP Server Specification

## Purpose
Exposes an AST-driven codebase intelligence service adhering to the Model Context Protocol (MCP) over native STDIO JSON-RPC 2.0 transport, allowing AI coding assistants to navigate, inspect, and analyze source code without loading entire files into the LLM context.

## Requirements

### Requirement: Native STDIO JSON-RPC Protocol Transport
The MCP server SHALL communicate via standard input/output (STDIO) using the JSON-RPC 2.0 specification, avoiding port conflicts and allowing client-driven lifecycle management.

#### Scenario: Editor initializes connection
- **GIVEN** an MCP client (such as Gemini Antigravity, Cursor, or Claude Code) launches `astrix serve`
- **WHEN** the client sends an `initialize` JSON-RPC request over stdin
- **THEN** the server SHALL respond with server capabilities, name (`astrix`), and supported protocol version over stdout
- **AND** the server SHALL keep listening for tool execution requests without binding to network ports

---

### Requirement: Project Catalog Inspection
The MCP server SHALL provide the `list_projects` tool to return all repositories registered in the Astrix database.

#### Scenario: Client queries available projects
- **GIVEN** the client calls `list_projects` with empty arguments
- **WHEN** the server processes the request
- **THEN** it SHALL return a structured list of all projects including `id`, `name`, `path`, `language`, `status`, `file_count`, and `symbol_count`

---

### Requirement: Hierarchical Structure and Centrality
The MCP server SHALL provide the `get_project_structure` tool to return the project file topology with optional PageRank centrality indicators.

#### Scenario: Client requests project topology with depth limit
- **GIVEN** a valid `project_id` and `max_depth = 2`
- **WHEN** the client calls `get_project_structure`
- **THEN** the server SHALL return a directory tree up to 2 levels deep
- **AND** files with highest dependency centrality SHALL be marked as architectural core nodes

---

### Requirement: AST Symbol Lookup
The MCP server SHALL provide the `lookup_symbol` tool to search for symbol definitions and references across the codebase.

#### Scenario: Searching for symbol definitions
- **GIVEN** a valid `project_id`, `symbol_name = "PrintSkills"`, and `mode = "definition"`
- **WHEN** the client invokes `lookup_symbol`
- **THEN** the server SHALL query the AST symbol index in SQLite
- **AND** it SHALL return matching locations in compact format with file path, line range, symbol kind, and full signature

#### Scenario: Searching for symbol call sites and usages
- **GIVEN** a valid `project_id`, `symbol_name = "DetectContext"`, and `mode = "references"`
- **WHEN** the client invokes `lookup_symbol`
- **THEN** the server SHALL return all call sites and references where the symbol is consumed across the project

---

### Requirement: Surgical Implementation Retrieval
The MCP server SHALL provide the `get_implementation` tool to extract the exact code block of a symbol without reading the full file.

#### Scenario: Extracting a function implementation
- **GIVEN** a valid `project_id`, relative `filepath`, and `symbol_name`
- **WHEN** the client invokes `get_implementation`
- **THEN** the server SHALL locate the start and end lines of the symbol from the AST index
- **AND** the server SHALL read only those lines and return the implementation code block

---

### Requirement: Batch Implementation Bundling
The MCP server SHALL provide the `get_implementation_bundle` tool to retrieve multiple symbols across different files in a single request, accepting targets as a typed JSON array of objects or serialized JSON string.

#### Scenario: Fetching multiple symbols via typed array
- **GIVEN** a native array of symbol targets `[{"filepath": "...", "symbol_name": "..."}, ...]`
- **WHEN** the client invokes `get_implementation_bundle`
- **THEN** the server SHALL extract all requested symbol implementations
- **AND** it SHALL concatenate and return them organized by file in a single response

#### Scenario: Fetching multiple symbols via serialized JSON string
- **GIVEN** a JSON-encoded string representing the list of symbol targets
- **WHEN** the client invokes `get_implementation_bundle`
- **THEN** the server SHALL deserialize the string and extract all requested symbols for backward compatibility

---

### Requirement: File Declaration Outline
The MCP server SHALL provide the `get_file_outline` tool to inspect all declarations in a file without returning full function bodies.

#### Scenario: Inspecting declarations of a source file
- **GIVEN** a valid `project_id` and relative `filepath`
- **WHEN** the client invokes `get_file_outline`
- **THEN** the server SHALL return a structural outline containing lines, kind, name, and signature of each declaration in the file
- **AND** it SHALL support formatted text outline or structured JSON when `format = "json"`

---

### Requirement: Index Status Warnings and Partial Results
The MCP server SHALL prepend a contextual warning to tool execution results when the targeted project is not in `ready` state.

#### Scenario: Querying a project undergoing background indexing
- **GIVEN** a project with status `indexing`
- **WHEN** the client queries code intelligence tools
- **THEN** the response SHALL prepend a warning indicating indexing progress percentage and file counts (e.g., `[INDEXING 45% (9/20 arquivos)]`)

#### Scenario: Querying a project with stale or failed index
- **GIVEN** a project with status `error`
- **WHEN** the client queries code intelligence tools
- **THEN** the response SHALL prepend a `[STALE INDEX]` warning with the failure reason

---

### Requirement: Tool Panic Recovery and Process Resilience
All MCP tool handlers SHALL be guarded with panic recovery to protect the STDIO connection and server process against fatal runtime panics (such as Tree-sitter CGO crashes).

#### Scenario: A tool handler encounters an unexpected panic
- **GIVEN** a tool handler suffers a panic during AST extraction or query execution
- **WHEN** the panic occurs
- **THEN** the server SHALL recover from the panic
- **AND** it SHALL return an MCP tool error result (`isError = true`) with the panic details
- **AND** the STDIO JSON-RPC session SHALL remain alive and ready for subsequent requests

---

### Requirement: Bounded Line Slice Reading
The MCP server SHALL provide the `read_file_lines` tool to safely read a specific slice of lines from a file.

#### Scenario: Reading a range of lines
- **GIVEN** a valid `project_id`, relative `filepath`, `start_line = 10`, and `end_line = 30`
- **WHEN** the client invokes `read_file_lines`
- **THEN** the server SHALL return only lines 10 through 30 of the specified file with line numbers

---

### Requirement: Textual Code Search
The MCP server SHALL provide the `grep_code` tool to perform fast regex or literal pattern search across indexed project files.

#### Scenario: Searching for a text pattern
- **GIVEN** a valid `project_id` and regex `pattern`
- **WHEN** the client invokes `grep_code`
- **THEN** the server SHALL scan project files respecting `.gitignore` rules
- **AND** it SHALL return file paths, line numbers, and matching line content up to the requested limit

---

### Requirement: Structured Configuration Query
The MCP server SHALL provide the `query_structured_file` tool to inspect configuration files (JSON, YAML, CSV) by key path without dumping the entire file into context.

#### Scenario: Extracting a nested config property
- **GIVEN** a valid `project_id`, `filepath = "package.json"`, and `query = "dependencies"`
- **WHEN** the client invokes `query_structured_file`
- **THEN** the server SHALL parse the structured file and return only the requested property value

