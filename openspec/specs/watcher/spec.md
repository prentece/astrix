# File Watcher Specification

## Purpose
Monitors repository filesystem events in real-time using fsnotify, filters ignored paths, groups burst I/O events with an intelligent debounce window, and triggers automated incremental reindexing.

## Requirements

### Requirement: Real-Time Operating System Event Monitoring
The File Watcher SHALL listen to filesystem mutation events (`Create`, `Write`, `Remove`, `Rename`) across all watched project directories.

#### Scenario: Developer saves a file in the project
- **GIVEN** a registered project is actively watched by `FileWatcherService`
- **WHEN** the developer or editor modifies and saves a file
- **THEN** the watcher SHALL intercept the write event and route it to the project's debounce queue

#### Scenario: Multiple projects with shared prefix or nested structures
- **GIVEN** projects located at `/workspace/proj` and `/workspace/proj-other`, or nested `/workspace/proj/sub`
- **WHEN** a filesystem event occurs
- **THEN** the watcher SHALL identify the project using path containment boundaries (not raw string prefix)
- **AND** if multiple projects match, it SHALL select the most specific (longest path) project

---

### Requirement: 1500ms Debounce Window for Burst I/O
The File Watcher SHALL aggregate rapid consecutive filesystem events into a single delta reindexing operation without losing events arriving during ongoing synchronization.

#### Scenario: Multiple rapid writes occur
- **GIVEN** an editor or formatter writes to files multiple times within milliseconds
- **WHEN** events are received by the watcher
- **THEN** the debounce timer SHALL be reset upon each event
- **AND** reindexing SHALL execute only after 1500ms of complete silence on the project

#### Scenario: Events occur during an active synchronization
- **GIVEN** a project is currently executing `TriggerSync` (`syncingMap[projectID] == true`)
- **WHEN** new filesystem events arrive for that project
- **THEN** the watcher SHALL mark the project as dirty during sync
- **AND** once the active sync completes, the watcher SHALL automatically reschedule a debounced sync to process the newly arrived changes

---

### Requirement: Early Gitignore and Non-Code Discard
The File Watcher SHALL immediately discard events on ignored files without performing disk operations or scheduling debounce timers.

#### Scenario: File in .gitignore or vendor is modified
- **GIVEN** a file inside `node_modules`, `vendor`, `.git`, or any rule in `.gitignore` changes
- **WHEN** the watcher receives the event
- **THEN** the event SHALL be matched against the compiled in-memory gitignore rules
- **AND** the event SHALL be dropped immediately without queueing delta processing

---

### Requirement: Dynamic Recursive Directory Watch Attachment
The File Watcher SHALL automatically attach watches to newly created subdirectories (including nested directory trees) within watched repositories.

#### Scenario: Developer creates a new directory or nested tree
- **GIVEN** a new folder (or recursive tree such as `cmd/api`) is created inside a watched project
- **WHEN** the watcher receives a `Create` event where `FileInfo.IsDir()` is true
- **THEN** the watcher SHALL verify that the folder is not ignored
- **AND** the watcher SHALL recursively add all non-ignored directories to `fsnotify.Watcher` and register them in memory

---

### Requirement: Graceful Teardown and Resource Cleanup
The File Watcher SHALL cleanly release operating system file descriptors and goroutines upon termination.

#### Scenario: Watcher service shutdown or project unwatch
- **GIVEN** Astrix is stopping or unwatching a project
- **WHEN** `Close()` or `UnwatchProject(projectID)` is invoked
- **THEN** the watcher SHALL cancel active debounce timers and discard pending flags
- **AND** the watcher SHALL remove watches from `fsnotify` using the in-memory registered directory list, even if the directory on disk was already deleted
- **AND** background polling routines SHALL exit immediately when context is cancelled

---

### Requirement: Dual-Mode Watch Execution (Embedded vs Standalone CLI)
The File Watcher architecture SHALL support running both embedded within the MCP server process (`astrix serve`) and standalone in the interactive terminal (`astrix watch`), preventing concurrent watcher conflicts via PID checks.

#### Scenario: Standalone CLI starts watcher when no server is running
- **GIVEN** no MCP server is running (`ReadPID()` is inactive)
- **WHEN** `astrix watch` starts
- **THEN** the CLI SHALL initialize and start `FileWatcherService` directly, monitoring registered projects and auto-syncing modifications to SQLite

#### Scenario: Standalone CLI defers watcher when server is already running
- **GIVEN** an MCP server is running (`ReadPID()` is active)
- **WHEN** `astrix watch` starts
- **THEN** the CLI SHALL NOT start a concurrent `FileWatcherService`
- **AND** the CLI SHALL stream the centralized log output in passive mode, preventing SQLite write contention

---

### Requirement: Single-Leader Watcher Coordination and Multi-Instance Failover
The File Watcher SHALL coordinate leadership via operating system file locks (`watcher.lock`), ensuring that only one process executes active `fsnotify` file watching even when multiple MCP server instances or terminal sessions are running concurrently.

#### Scenario: Multiple MCP editor windows run simultaneously
- **GIVEN** a first MCP instance is running and holds the `watcher.lock` (acting as Leader)
- **WHEN** additional MCP instances are launched (e.g. secondary editor windows)
- **THEN** secondary instances SHALL operate as replicas, serving MCP queries without starting a concurrent `fsnotify` watcher
- **AND** secondary instances SHALL poll the lock in the background

#### Scenario: Active Leader process closes or terminates
- **GIVEN** the Leader MCP process is closed or terminated
- **WHEN** the OS kernel releases the `watcher.lock`
- **THEN** a secondary replica instance SHALL acquire the lock within 2 seconds
- **AND** it SHALL automatically promote itself to Leader and start `FileWatcherService` without user intervention

---

### Requirement: Restartable Lifecycle
The `FileWatcherService` SHALL support repeated `Start()` and `Stop()` cycles within the same process without errors, dynamically allocating a new OS watcher and context upon each restart.

#### Scenario: Watcher restarted after stopping
- **GIVEN** `FileWatcherService` has been started and subsequently stopped via `Stop()`
- **WHEN** `Start()` is invoked again on the same instance
- **THEN** the service SHALL instantiate a fresh `fsnotify.Watcher` and cancellation context
- **AND** it SHALL re-register project directories without reporting `fsnotify: watcher already closed`


