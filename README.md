<p align="center">
  <h1 align="center">⭐ Astrix</h1>
  <p align="center">AST-powered codebase intelligence for AI coding assistants</p>
</p>

<p align="center">
  <a href="https://github.com/prentece/astrix/releases"><img src="https://img.shields.io/github/v/release/prentece/astrix?style=flat-square&color=blue" alt="Latest Release"></a>
  <a href="https://www.npmjs.com/package/@prentece/astrix"><img src="https://img.shields.io/npm/v/@prentece/astrix?style=flat-square&color=red" alt="npm version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License"></a>
  <a href="https://github.com/prentece/astrix/actions"><img src="https://img.shields.io/github/actions/workflow/status/prentece/astrix/release.yml?style=flat-square" alt="CI"></a>
  <img src="https://img.shields.io/badge/go-1.24+-00ADD8?style=flat-square&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/platforms-linux%20%7C%20macOS%20%7C%20windows-lightgrey?style=flat-square" alt="Platforms">
</p>

---

**Astrix** is a local MCP (Model Context Protocol) server that indexes your codebase using Tree-sitter and exposes a structured, token-efficient API for AI coding assistants. Instead of dumping entire files into the LLM context, Astrix lets your AI agent navigate symbols, inspect implementations, and query project structure with surgical precision.

## ✨ Features

- 🌳 **Polyglot AST Indexing** — Parses Go, TypeScript/JavaScript, Python, Java, and PHP via Tree-sitter
- ⚡ **Incremental Delta Indexing** — Two-Tier Hash evaluation (mtime/size fast check + SHA-256 digest) with atomic SQLite transactions skips unchanged files
- 📡 **MCP Server (STDIO)** — Native JSON-RPC 2.0 transport, zero port conflicts, client-managed lifecycle
- 🔍 **Symbol Lookup** — Find function/class definitions and all their reference sites across the codebase
- 📐 **Architectural Centrality** — PageRank-based dependency graph highlights architectural core nodes ("God Nodes")
- 👁️ **Real-Time File Watcher** — Debounced fsnotify watcher with dual execution modes (active standalone or zero-contention passive stream)
- 🛡️ **Security Boundaries** — Path traversal protection and bounded line-slice validation built-in
- 🤖 **Multi-Agent Skill Generation** — Generates configuration files for Antigravity, Cursor, Claude Code, and GitHub Copilot
- 🖥️ **Interactive TUI** — Context-aware terminal UI powered by Bubble Tea and Lip Gloss

## 🤖 Supported AI Assistants

Astrix integrates as an MCP server with:

| Assistant | Config file generated |
|---|---|
| [Google Antigravity](https://antigravity.dev) | `.agents/skills/astrix/SKILL.md` |
| [Cursor](https://cursor.sh) | `.cursor/rules/astrix.mdc` |
| [Claude Code](https://claude.ai/code) | `.claude/skills/astrix/SKILL.md` |
| [GitHub Copilot](https://github.com/features/copilot) | `.github/copilot-instructions.md` |

## 📦 Installation

### via npm (recommended)

```bash
npm install -g @prentece/astrix
```

Works on Linux, macOS (Intel & Apple Silicon), and Windows — the correct binary for your platform is resolved automatically.

### From source

**Requirements:** Go 1.24+, GCC (for CGO/SQLite)

```bash
git clone https://github.com/prentece/astrix.git
cd astrix
make build
# Binary will be at ./bin/astrix
```

## 🚀 Quick Start

Navigate to any project directory and run:

```bash
astrix
```

Astrix automatically detects context:

- **Inside a registered project** → opens the project dashboard
- **Inside an unregistered repository** → launches the onboarding wizard
- **Outside any repository** → opens the global dashboard

### Register and index a project

```bash
# Follow the interactive wizard
astrix

# Or list already registered projects
astrix ls

# Check system and server status
astrix status

# Force full reindexing of the current repository
astrix index
```

### Real-time file watching

```bash
# Monitor changes in real-time with automated incremental reindexing
astrix watch
```

`astrix watch` automatically checks whether the MCP server (`astrix serve`) is already running in background:
- **Passive Monitor Mode**: If the MCP server is online, `astrix watch` streams real-time events (`[CAPTURADO]`, `[SINCRONIZADO]`, `[AVISO]`) without duplicating file watchers or competing for database locks.
- **Active Standalone Mode**: If no server is running, it starts the `FileWatcherService` directly in the foreground.

### Configure AI agent skills

```bash
astrix skills
```

Select which AI assistants you use — Astrix writes the appropriate skill/rules files so your agents know how to query the MCP server.

### Generate MCP configuration

```bash
# Print standard MCP configuration JSON
astrix config

# Print configuration with absolute binary path
astrix config --absolute

# Print configuration using npx
astrix config --npx
```

### Start the MCP server

```bash
astrix serve
```

Add it to your AI assistant's MCP configuration (`~/.cursor/mcp.json`, Claude Desktop, etc.):

```json
{
  "mcpServers": {
    "astrix": {
      "command": "astrix",
      "args": ["serve"]
    }
  }
}
```

Or using `npx` directly without global installation:

```json
{
  "mcpServers": {
    "astrix": {
      "command": "npx",
      "args": ["-y", "@prentece/astrix", "serve"]
    }
  }
}
```

## 🔧 MCP Tools Reference

Once running, Astrix exposes the following MCP tools to AI agents:

| Tool | Description |
|---|---|
| `list_projects` | List all registered projects with metadata |
| `get_project_structure` | Directory tree with optional centrality depth limit |
| `lookup_symbol` | Find symbol definitions or all reference sites |
| `get_implementation` | Extract the exact code block of a symbol |
| `get_implementation_bundle` | Retrieve multiple symbols across files in one call |
| `read_file_lines` | Read a bounded line slice from any project file |
| `grep_code` | Regex/literal search across indexed project files |
| `query_structured_file` | Inspect JSON/YAML/CSV by key path without dumping the full file |

## 🏗️ Architecture

```
astrix/
├── cmd/astrix/          # Binary entry point
├── internal/
│   ├── cli/             # TUI, commands, and routing logic
│   ├── mcp/             # MCP server, tools, and formatters
│   └── service/         # Business logic orchestration layer
├── pkg/
│   ├── indexer/         # Tree-sitter AST parsing, delta engine, PageRank
│   ├── storage/         # SQLite repositories (WAL, migrations, cascade delete)
│   └── watcher/         # fsnotify-based real-time file watcher
├── openspec/specs/      # Behavior-driven specifications for each component
└── npm/                 # npm package wrapper for cross-platform distribution
```

**Data flow:**

```
Editor saves file
    → fsnotify watcher (1500ms debounce)
    → delta engine (SHA-256 diff)
    → Tree-sitter parser (goroutine pool)
    → SQLite (WAL, atomic batch inserts)
    → MCP tools (symbol lookup, grep, read)
    → AI assistant context (token-efficient)
```

## 🛠️ Development

```bash
# Run all tests
make test

# Run tests without cache (faster feedback loop)
make test-fast

# Tidy dependencies
make tidy

# Clean build artifacts
make clean
```

### Testing npm package locally

```bash
make npm-prepare-local
make npm-test
```

## 🤝 Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/my-feature`)
3. Commit your changes following [Conventional Commits](https://www.conventionalcommits.org/)
4. Open a Pull Request

Before submitting, make sure all tests pass:

```bash
make test
```

The `openspec/specs/` directory contains behavior-driven specifications for every component. New features should include a corresponding spec update.

## 📄 License

MIT © [prentece](https://github.com/prentece)
