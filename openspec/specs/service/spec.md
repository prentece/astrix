# Service Layer Specification

## Purpose
Orchestrates business logic, project lifecycle operations, code intelligence queries, and cross-cutting concerns between the CLI, MCP server, Indexer Engine, Storage layer, and File Watcher.

## Requirements

### Requirement: Project Lifecycle Orchestration
The Service layer SHALL coordinate project registration, listing, deletion, and reindexing across repositories and background services.

#### Scenario: Registering a new project
- **GIVEN** a valid project name, absolute directory path, and detected language
- **WHEN** `ProjectService.RegisterProject` is called
- **THEN** it SHALL generate a deterministic unique ID (`proj-<hash>`)
- **AND** it SHALL persist the project in `ProjectRepository` with `status = 'ready'`
- **AND** if a `ProjectWatcher` is configured, it SHALL immediately register the project for background monitoring

#### Scenario: Deleting an existing project
- **GIVEN** a registered project with ID `proj-xyz`
- **WHEN** `ProjectService.DeleteProject` is called
- **THEN** it SHALL first unregister the project from the file watcher to prevent ghost events
- **AND** it SHALL delete the project from `ProjectRepository` with full cascade to symbols and dependencies

---

### Requirement: Canonical Path Validation and Duplicate Prevention
The Service layer SHALL ensure that no two projects point to the same filesystem directory.

#### Scenario: Registering a path that is already registered
- **GIVEN** a directory `/home/user/project` is already registered as `proj-1`
- **WHEN** `ProjectService.RegisterProject` is invoked with the same path or a symlink resolving to it
- **THEN** the service SHALL resolve canonical paths using `filepath.Abs` and `filepath.EvalSymlinks`
- **AND** it SHALL return an error indicating that the repository is already registered

---

### Requirement: AST Symbol Navigation and Impact Analysis
The Service layer SHALL route symbol search and reference discovery requests to the storage repository with bounds enforcement.

#### Scenario: Finding a symbol by name
- **GIVEN** a valid `project_id`, `symbolName`, `limit`, and `offset`
- **WHEN** `CodeService.FindSymbol` is invoked
- **THEN** the service SHALL validate parameters and query `SymbolRepository.FindSymbol`
- **AND** it SHALL return matching symbols and a boolean indicating if more pages are available

#### Scenario: Analyzing symbol impact and callers
- **GIVEN** a valid `project_id` and `symbolName`
- **WHEN** `CodeService.FindReferences` is invoked
- **THEN** the service SHALL query usage sites and call dependencies across the project graph

---

### Requirement: Security Boundary and Path Traversal Protection
The Service layer SHALL prevent path traversal vulnerabilities when reading files or extracting implementations.

#### Scenario: Client requests a file path with parent directory navigation
- **GIVEN** a request with `filepath = "../../../etc/passwd"`
- **WHEN** `CodeService.ReadFileLines` or `CodeService.GetImplementation` is called
- **THEN** the service SHALL resolve the target path against the project root directory
- **AND** if the target path falls outside the project root, the service SHALL reject the request with a permission/security error

---

### Requirement: Bounded Line Slicing Protection
The Service layer SHALL validate requested line boundaries against actual file line counts.

#### Scenario: Client requests lines beyond the end of a file
- **GIVEN** a file with 50 total lines and a request for `start_line = 60` and `end_line = 100`
- **WHEN** `CodeService.ReadFileLines` is called
- **THEN** the service SHALL detect that `start_line` exceeds the file length
- **AND** it SHALL return a descriptive out-of-bounds error instead of an internal crash
