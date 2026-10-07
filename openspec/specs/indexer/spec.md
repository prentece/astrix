# Indexer Specification

## Purpose
Coordinates the multi-language Abstract Syntax Tree (AST) parsing, symbol extraction, dependency graph generation, and incremental delta analysis of codebases using Tree-sitter.

## Requirements

### Requirement: Polyglot Tree-sitter AST Parsing
The Indexer Engine SHALL parse source code files across supported programming languages (Go, TypeScript/JavaScript, Python, Java, PHP) into concrete AST representations.

#### Scenario: Parsing a Go source file
- **GIVEN** a Go file containing package declarations, functions, types, and methods
- **WHEN** the file is parsed by the Go grammar extractor
- **THEN** the engine SHALL extract functions, methods, structs, interfaces, signatures, line ranges, and visibility

#### Scenario: Parsing a TypeScript source file
- **GIVEN** a TypeScript or JavaScript file containing classes, interfaces, and exported functions
- **WHEN** the file is parsed by the TypeScript grammar extractor
- **THEN** the engine SHALL extract class declarations, method signatures, interfaces, and export scopes

---

### Requirement: Incremental Delta Indexing via Hashing
The Indexer Engine SHALL avoid reprocessing unchanged files by maintaining cryptographic SHA-256 hashes and file modification timestamps in `file_states`.

#### Scenario: Reindexing an unchanged file
- **GIVEN** a file whose SHA-256 hash matches the hash recorded in `file_states`
- **WHEN** incremental reindexing is triggered
- **THEN** the engine SHALL skip Tree-sitter parsing for that file and retain existing symbols in the database

#### Scenario: Reindexing a modified file
- **GIVEN** a file whose content hash differs from `file_states`
- **WHEN** incremental reindexing is triggered
- **THEN** the engine SHALL delete existing symbols for that file
- **AND** the engine SHALL parse the updated content, extract new symbols, and update `file_states`

#### Scenario: Handling deleted files
- **GIVEN** a file previously indexed that no longer exists on disk
- **WHEN** reindexing runs
- **THEN** the engine SHALL delete all associated symbols, dependencies, and file state entries for that file

---

### Requirement: Architectural Dependency Graph and Centrality
The Indexer Engine SHALL compute structural dependency relationships between files and calculate PageRank centrality metrics.

#### Scenario: Building the import dependency graph
- **GIVEN** a project with cross-file imports and module dependencies
- **WHEN** the project indexing completes
- **THEN** the engine SHALL persist directed edges (`source_file` -> `target_file`) in `dependency_graphs`
- **AND** the engine SHALL compute centrality scores to identify core components and God Nodes

#### Scenario: Disambiguating homonym target symbols
- **GIVEN** an unresolved dependency edge with a target symbol defined in multiple files
- **WHEN** target files are resolved via `ResolveTargetFiles`
- **THEN** candidate files located in the same directory as `source_file` SHALL be prioritized over other directories
- **AND** production code files SHALL be prioritized over test files (`_test.go`, `/tests/`)

#### Scenario: Skipping centrality recalculation when digest is unchanged
- **GIVEN** an incremental delta where no files were added or deleted
- **AND** all modified files have identical `DigestHash` (only internal function body changes)
- **WHEN** `ProcessIncrementalDelta` executes
- **THEN** `CalculateProjectCentrality` and `ResolveTargetFiles` SHALL be skipped to preserve CPU and I/O resources
- **AND** the project status SHALL remain `ready` with preserved centrality scores

---

### Requirement: Resilient Parsing with Error Recovery
The Indexer Engine SHALL tolerate partial syntax errors without crashing or halting the indexing pipeline.

#### Scenario: File contains a syntax error
- **GIVEN** a file with malformed syntax (e.g. unclosed bracket or incomplete statement)
- **WHEN** Tree-sitter parses the file
- **THEN** the engine SHALL extract all valid symbols in reachable subtrees
- **AND** the engine SHALL skip only the malformed `ERROR` nodes and continue indexing remaining files

---

### Requirement: Concurrency and Worker Pool Management
The Indexer Engine SHALL distribute file parsing across a concurrent worker pool scaled to available CPU cores.

#### Scenario: Indexing a large repository
- **GIVEN** a repository with hundreds of source files
- **WHEN** `IndexProject` or `ReindexProject` is invoked
- **THEN** the engine SHALL spawn `runtime.NumCPU()` worker goroutines to parse files concurrently
- **AND** atomic progress updates SHALL be tracked in `progressMap` for clients to monitor

#### Scenario: Incremental delta parallel parsing
- **GIVEN** an incremental delta with multiple added or modified files
- **WHEN** `ProcessIncrementalDelta` is invoked
- **THEN** the engine SHALL distribute candidate files across a concurrent worker pool
- **AND** each worker SHALL execute AST parsing and digest extraction in a single pass using cached Tree-sitter queries

---

### Requirement: Atomic Full Reindex Persistence
The Indexer Engine SHALL persist the result of a full reindex without ever leaving the project with a partial or empty index.

#### Scenario: Reindex with an atomic index replacer configured
- **GIVEN** the engine has an `IndexReplacer` (set via `SetIndexReplacer`)
- **WHEN** `IndexProject` finishes parsing all files
- **THEN** the old index SHALL remain queryable during the whole parsing phase
- **AND** clearing the old index and writing symbols, references, dependencies, data models, and file states SHALL happen in a single transaction
- **AND** target-file resolution and centrality scoring SHALL run after that transaction as derived, recomputable data

#### Scenario: Persistence fails
- **GIVEN** the transactional write returns an error
- **WHEN** `IndexProject` handles the failure
- **THEN** the previous index SHALL remain intact
- **AND** the project status SHALL be set to `error` with a descriptive message and the error SHALL be returned

#### Scenario: No atomic replacer configured
- **GIVEN** the engine has no `IndexReplacer`
- **WHEN** `IndexProject` runs
- **THEN** it SHALL fall back to per-repository operations, clearing the old index only after parsing completes (not atomic)

---

### Requirement: Project-Root Confinement for File Access
The Indexer SHALL resolve every client-supplied relative path with `SafeJoin` and MUST NOT read or walk outside the project root, including through symlinks (see the Service specification for the observable behavior).

---

### Requirement: Repository Walking and Filtering
The repository scanner SHALL discover indexable code files while honoring root `.gitignore`, `.git/info/exclude`, and nested `.gitignore` files, ignoring binary files and oversized files.

#### Scenario: Nested gitignores and exclude rules
- **GIVEN** a project with `.gitignore` in subdirectories or local exclude rules in `.git/info/exclude`
- **WHEN** `ScanRepository` or `LoadGitIgnore` processes the directory tree
- **THEN** all nested and exclude patterns SHALL be combined and enforced relative to the project root

#### Scenario: Binary and oversized file exclusion
- **GIVEN** a binary file (containing NUL bytes `0x00`) or a file exceeding `MaxIndexableFileSize` (2MB)
- **WHEN** `ScanRepository` inspects the candidate file
- **THEN** the file SHALL be skipped from AST parsing without generating error logs

---

### Requirement: Unified Language Registry
All components requiring language detection SHALL derive the language from the central `LanguageConfig` registry as the single source of truth, with fallback for non-AST documentation and structured config formats.

#### Scenario: Detecting file language
- **GIVEN** a file with a supported language extension (e.g. `.go`, `.ts`, `.py`, `.java`, `.php`)
- **WHEN** language detection is performed
- **THEN** it SHALL resolve directly from the registered `LanguageConfig`
