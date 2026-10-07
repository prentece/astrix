# Astrix

> **Fast Code Intelligence & AST Engine for AI Coding Agents**

Astrix indexes codebases with native speed using Tree-sitter ASTs and exposes token-efficient context tools via the **Model Context Protocol (MCP)** and an interactive CLI for AI coding agents (Cursor, Claude Code, Google Antigravity, Windsurf, GitHub Copilot, etc.).

---

## 🚀 Quick Start

### Run directly without installing:
```bash
npx @prentece/astrix
```

### Install globally:
```bash
npm install -g @prentece/astrix
```

Once installed globally, run `astrix` from any terminal:
```bash
astrix --help
```

---

## 🤖 Setup with AI Assistants (MCP)

Add Astrix to your assistant's MCP configuration (`~/.cursor/mcp.json`, Claude Desktop, Antigravity, etc.):

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

> **Tip:** If installed globally, you can simplify the command to `"command": "astrix"`, `"args": ["serve"]`.

---

## 🛠️ Core Commands

```bash
# Launch interactive context-aware dashboard & onboarding wizard
astrix

# Start the MCP server via STDIO (port-free)
astrix serve

# Monitor filesystem changes in real-time with automatic incremental reindexing
astrix watch

# Print MCP configuration snippet for editors/agents (--absolute or --npx)
astrix config

# Configure AI agent skills (.agents, .cursor, .claude, .github)
astrix skills

# Check status of registered projects, SQLite database, and active MCP server
astrix status

# List all registered projects
astrix ls

# Force full reindexing of the current repository
astrix index
```

---

## 💻 Supported Platforms

The `@prentece/astrix` package automatically resolves and executes the precompiled native binary for your operating system:

- **macOS**: Apple Silicon (`darwin-arm64`) and Intel (`darwin-x64`)
- **Linux**: x86_64 (`linux-x64`) and ARM64 (`linux-arm64`)
- **Windows**: x86_64 (`win32-x64`)

---

## 📄 License

MIT © [prentece](https://github.com/prentece)
