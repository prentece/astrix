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

---

### Requirement: 1500ms Debounce Window for Burst I/O
The File Watcher SHALL aggregate rapid consecutive filesystem events into a single delta reindexing operation.

#### Scenario: Multiple rapid writes occur
- **GIVEN** an editor or formatter writes to files multiple times within milliseconds
- **WHEN** events are received by the watcher
- **THEN** the debounce timer SHALL be reset upon each event
- **AND** reindexing SHALL execute only after 1500ms of complete silence on the project

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
The File Watcher SHALL automatically attach watches to newly created subdirectories within watched repositories.

#### Scenario: Developer creates a new directory
- **GIVEN** a new folder is created inside a watched project
- **WHEN** the watcher receives a `Create` event where `FileInfo.IsDir()` is true
- **THEN** the watcher SHALL verify that the folder is not ignored
- **AND** the watcher SHALL add the new directory to `fsnotify.Watcher` to monitor files inside it

---

### Requirement: Graceful Teardown and Resource Cleanup
The File Watcher SHALL cleanly release operating system file descriptors and goroutines upon termination.

#### Scenario: Watcher service shutdown
- **GIVEN** Astrix is stopping or unwatching a project
- **WHEN** `Close()` or `UnwatchProject(projectID)` is invoked
- **THEN** the watcher SHALL cancel active debounce timers
- **AND** the watcher SHALL remove watches from `fsnotify` and cancel the background context
