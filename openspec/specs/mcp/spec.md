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
The MCP server SHALL provide the `get_implementation_bundle` tool to retrieve multiple symbols across different files in a single request.

#### Scenario: Fetching multiple symbols in one call
- **GIVEN** a list of symbol targets `[{"filepath": "...", "symbol_name": "..."}, ...]`
- **WHEN** the client invokes `get_implementation_bundle`
- **THEN** the server SHALL extract all requested symbol implementations
- **AND** it SHALL concatenate and return them organized by file in a single response

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
