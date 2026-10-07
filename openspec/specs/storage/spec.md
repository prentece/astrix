# Storage Specification

## Purpose
Manages the embedded SQLite database persistence layer, domain model schemas, transactions, migrations, and decoupled repository pattern implementations for projects, symbols, dependencies, and file states.

## Requirements

### Requirement: High-Performance SQLite Configuration
The Storage layer SHALL initialize the local SQLite database (`~/.astrix/astrix.db`) with Write-Ahead Logging (WAL) and optimized concurrency pragmas.

#### Scenario: Database connection initialization
- **GIVEN** Astrix starts and creates the database connection via `NewDatabase`
- **WHEN** the SQLite file is opened
- **THEN** the system SHALL enable `_journal_mode=WAL` to allow non-blocking concurrent readers and writers
- **AND** the system SHALL configure `_busy_timeout=5000` to wait up to 5 seconds before returning lock busy errors
- **AND** the system SHALL enforce `_foreign_keys=ON` for relational integrity

---

### Requirement: Idempotent Automated Schema Migrations
The Storage layer SHALL ensure that all required tables and indexes exist upon connection startup without requiring external migration tooling.

#### Scenario: Running migrations on a fresh or existing database
- **GIVEN** `NewDatabase(dbPath)` is called
- **WHEN** `migrate()` executes
- **THEN** the schema tables (`projects`, `symbols`, `dependency_graph`, `data_models`, `project_file_states`) and their indexes SHALL be created idempotently using `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS`

---

### Requirement: Relational Integrity and Cascade Deletion
The Storage layer SHALL guarantee that deleting a project cascades to all child records automatically via foreign key constraints.

#### Scenario: Project is deleted from database
- **GIVEN** a project with ID `proj-123` has thousands of indexed symbols, edges, data models, and file states
- **WHEN** `projectRepo.Delete("proj-123")` is executed
- **THEN** SQLite foreign keys SHALL automatically delete all associated records in `symbols`, `dependency_graph`, `data_models`, and `project_file_states` in a single transaction

---

### Requirement: Atomic Batch Symbol Persistence
The Storage layer SHALL persist symbols and dependency edges in atomic transactions to guarantee consistency and performance.

#### Scenario: Indexer saves thousands of symbols
- **GIVEN** a batch of newly extracted symbols from an indexed project
- **WHEN** `symbolRepo.SaveSymbols(projectID, symbols)` is executed
- **THEN** all symbols SHALL be inserted inside an explicit `BeginTx` transaction
- **AND** if an error occurs during insertion, the transaction SHALL be rolled back completely

---

### Requirement: Atomic Full Project Index Replacement
The Storage layer SHALL expose `IndexReplacer.ReplaceProjectIndex(snapshot)` (implemented by `IndexStore`) to replace the entire index of a project in a single transaction.

#### Scenario: Full reindex persists a new snapshot
- **GIVEN** an `IndexSnapshot` containing symbols, references, dependency edges, data models, and (optionally) file states
- **WHEN** `ReplaceProjectIndex` is executed
- **THEN** the previous symbols, references, dependency edges, and data models of the project SHALL be deleted and the new data inserted within one `BeginTx` transaction
- **AND** file states SHALL be replaced in the same transaction only when `ReplaceFileStates` is true

#### Scenario: A write fails mid-replacement
- **GIVEN** a project with an existing index
- **WHEN** any step of `ReplaceProjectIndex` fails (e.g. a foreign key violation)
- **THEN** the whole transaction SHALL be rolled back
- **AND** the previous index SHALL remain fully intact and queryable

#### Scenario: Invalid snapshot
- **GIVEN** a `nil` snapshot or one without `ProjectID`
- **WHEN** `ReplaceProjectIndex` is called
- **THEN** it SHALL return an error without touching the database

Repository batch methods (`SaveSymbols`, `SaveReferences`, `SaveDependencies`, `SaveDataModels`, `UpsertBatch`) SHALL share the same transaction-level insert helpers used by `ReplaceProjectIndex`.

---

### Requirement: Atomic Incremental Index Persistence
The Storage layer SHALL expose `IncrementalApplier.ApplyIncrementalDelta(delta)` (implemented by `IndexStore`) to atomically apply deletions, insertions, and file state updates in a single transaction.

#### Scenario: Incremental sync applies changes
- **GIVEN** an `IncrementalIndexDelta` containing deleted files, modified files, newly parsed symbols, callers, dependencies, models, and file states
- **WHEN** `ApplyIncrementalDelta` executes
- **THEN** records of deleted and modified files SHALL be removed from `symbols`, `references_table`, `dependency_graph`, `data_models`, and `project_file_states`
- **AND** newly extracted records and file states SHALL be inserted within the same `BeginTx` transaction
- **AND** if any write fails, the entire transaction SHALL be rolled back completely

---

### Requirement: Dangling State Auto-Recovery
The Storage layer SHALL recover projects that remained in the `indexing` status due to unexpected process termination.

#### Scenario: System crashed during previous indexing run
- **GIVEN** a project was left in `status = 'indexing'` when the process was killed
- **WHEN** Astrix initializes via `Execute()` and calls `ResetDanglingIndexingStatus()`
- **THEN** the project status SHALL be automatically reset to `ready` with an informative error message explaining that the previous indexing was interrupted
