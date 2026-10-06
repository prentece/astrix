# CLI Specification

## Purpose
Provides an interactive Terminal User Interface (TUI) and standalone command-line entry points for developers to manage codebase indexing, inspect project metadata, configure AI agent skills, and interact with the Astrix engine.

## Requirements

### Requirement: Contextual Execution Without Arguments
The CLI SHALL inspect the current working directory when invoked without arguments and automatically route to the appropriate interactive flow based on repository registration state.

#### Scenario: Running inside a registered project
- **GIVEN** the current working directory is a git repository registered in `astrix.db`
- **WHEN** the user executes `astrix` with no arguments
- **THEN** the system SHALL launch `RunDashboard` displaying the project header card and interactive action menu

#### Scenario: Running inside an unregistered code repository
- **GIVEN** the current working directory contains source code files or git markers but is not registered in `astrix.db`
- **WHEN** the user executes `astrix` with no arguments
- **THEN** the system SHALL launch `RunWizard` to prompt for project onboarding and IA agent selection

#### Scenario: Running outside any repository
- **GIVEN** the current working directory is not a code repository
- **WHEN** the user executes `astrix` with no arguments
- **THEN** the system SHALL launch `RunGlobalDashboard` displaying global system status and multi-project options

---

### Requirement: Alternate Screen Buffer Management
All interactive views (dashboard, wizard, clean, skills) SHALL execute inside the terminal Alternate Screen Buffer (`AltScreen`) to eliminate visual clutter and avoid polluting terminal scrollback.

#### Scenario: User cancels or quits with ESC
- **GIVEN** an interactive TUI screen is active (e.g. `astrix clean`, `astrix skills`, or `astrix` dashboard)
- **WHEN** the user presses `Esc` or selects `Cancelar`
- **THEN** the system SHALL immediately exit the alternate screen buffer
- **AND** the system SHALL leave zero residual lines or unselected form artifacts on the terminal prompt
- **AND** the system SHALL terminate with exit code 0 without printing error messages

#### Scenario: Standalone interactive action completes successfully
- **GIVEN** `astrix skills` or `astrix clean` is executed standalone
- **WHEN** the user submits the form and the operation succeeds
- **THEN** the system SHALL exit the alternate screen buffer
- **AND** the system SHALL print the structured result directly to standard stdout

---

### Requirement: Uniform Key-Value List Formatting
All terminal output from standalone commands SHALL start at column 0 with standardized key-value formatting (`ui.KeyValue`) using the Asterix color palette.

#### Scenario: Listing registered projects
- **GIVEN** one or more projects are registered in Astrix
- **WHEN** the user executes `astrix ls`
- **THEN** each project name SHALL start at column 0 without arrow prefixes (`> `)
- **AND** each property (`Linguagem`, `Status`, `Caminho`) SHALL be aligned uniformly at column 0

#### Scenario: Inspecting system status
- **GIVEN** Astrix is initialized
- **WHEN** the user executes `astrix status`
- **THEN** the output SHALL render `Base de Dados`, `Arquivo de Logs`, `Projetos Cadastrados`, `Servidor MCP`, and `Contexto Atual` starting at column 0 with `[STANDBY]` status for MCP

---

### Requirement: Multi-Agent Skills Generation
The CLI SHALL generate tailored rule and skill definitions for multiple AI programming assistants.

#### Scenario: Configuring skills for selected agents
- **GIVEN** a registered project with ID `proj-xyz` and name `my-app`
- **WHEN** the user selects agents (Antigravity, Cursor, Claude Code, GitHub Copilot) in `astrix skills`
- **THEN** the system SHALL write the corresponding skill files in `.agents/skills/astrix/SKILL.md`, `.cursor/rules/astrix.mdc`, `.claude/skills/astrix/SKILL.md`, or `.github/copilot-instructions.md`
- **AND** each generated file SHALL include the project ID and instructions for querying Astrix MCP

---

### Requirement: Safe Project Deletion
The CLI SHALL require explicit confirmation before deleting project metadata, indexes, and skills.

#### Scenario: Confirming project removal
- **GIVEN** the user executes `astrix clean`
- **WHEN** the user selects `Excluir projeto`
- **THEN** the system SHALL remove the project from `astrix.db`
- **AND** the system SHALL remove generated `.astrix/` config and skill directories
- **AND** the system SHALL print confirmation in key-value format (`Status: removido`)

#### Scenario: Aborting project removal
- **GIVEN** the user executes `astrix clean`
- **WHEN** the user selects `Cancelar` or presses `Esc`
- **THEN** the system SHALL exit immediately without modifying the database or printing errors

---

### Requirement: Fast-Path Utility Execution
Utility commands (`version`, `-v`, `--version`, `help`, `-h`, `--help`) SHALL execute immediately without opening the SQLite database, running database migrations, or instantiating the file watcher.

#### Scenario: User requests version
- **GIVEN** the user runs `astrix version` or `astrix -v`
- **WHEN** the command is processed
- **THEN** the system SHALL print the version to stdout immediately and exit with code 0 without touching database files

#### Scenario: User requests help
- **GIVEN** the user runs `astrix --help` or `astrix -h`
- **WHEN** the command is processed
- **THEN** the system SHALL display the help banner and exit with code 0

---

### Requirement: Machine-Readable Script Output (`--json`)
The CLI SHALL support the `--json` flag for inspection commands (`ls` and `status`) to provide clean, scriptable JSON output suitable for automation and tooling.

#### Scenario: Listing projects in JSON format
- **GIVEN** projects are registered in Astrix
- **WHEN** the user executes `astrix ls --json`
- **THEN** the output SHALL be a valid JSON array of project objects formatted with indentation
- **AND** if no projects are registered, it SHALL output an empty JSON array `[]`

#### Scenario: Inspecting status in JSON format
- **GIVEN** the user executes `astrix status --json`
- **WHEN** the command is executed
- **THEN** the output SHALL be a structured JSON object containing `database_path`, `log_path`, `projects_count`, `mcp_server` (`online`, `pid`), and `current_context`

---

### Requirement: Non-Zero Exit Code on Failure
The CLI SHALL return a non-zero exit code (exit code 1) when any command fails or when an unknown command is invoked.

#### Scenario: Unknown command invocation
- **GIVEN** the user executes an invalid command `astrix unknown-cmd`
- **WHEN** the CLI routes the arguments
- **THEN** it SHALL display an error message and help text, returning a non-zero exit code

---

### Requirement: Concurrency-Safe Dangling Indexing Reset
The CLI SHALL verify whether an MCP server process is currently active before resetting dangling `indexing` project states to avoid interrupting legitimate in-flight indexing jobs.

#### Scenario: Running CLI command while MCP server is actively indexing
- **GIVEN** an active `astrix serve` process is indexing a project in background
- **WHEN** the user runs `astrix ls` or `astrix status` from another terminal
- **THEN** the CLI SHALL detect the running MCP server PID
- **AND** it SHALL preserve the in-flight `indexing` project status without resetting it

