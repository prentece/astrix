# Astrix

> **Fast Code Intelligence & AST Engine for AI Coding Agents**

Astrix indexa codebases com velocidade nativa através de árvores sintáticas (Tree-sitter) e expõe ferramentas contextuais via **Model Context Protocol (MCP)** e CLI interativo para agentes de IA (Cursor, Windsurf, Claude Code, Gemini CLI, Antigravity, etc.).

---

## 🚀 Instalação Rápida

### Executar diretamente sem instalar:
```bash
npx @prentece/astrix
```

### Instalar globalmente no sistema:
```bash
npm install -g @prentece/astrix
```

Depois de instalado globalmente, use diretamente o comando `astrix`:
```bash
astrix --help
```

---

## 🛠️ Comandos Principais

```bash
# Inicia o menu interativo com todas as ações
astrix

# Exibe a configuração do servidor MCP para editores/agentes
astrix config

# Inicia o servidor MCP via STDIO
astrix serve

# Lista projetos gerenciados
astrix ls

# Exibe status dos serviços e índices
astrix status

# Força a reindexação sintática do repositório atual
astrix index

# Configura as SKILLs para agentes (.agents, .cursor, .claude)
astrix skills
```

---

## 💻 Plataformas Suportadas

O pacote `@prentece/astrix` instala automaticamente o binário nativo pré-compilado para o seu sistema:

- **macOS**: Apple Silicon (`darwin-arm64`) e Intel (`darwin-x64`)
- **Linux**: x86_64 (`linux-x64`) e ARM64 (`linux-arm64`)
- **Windows**: x86_64 (`win32-x64`)

---

## 📄 Licença

MIT © [prentece](https://github.com/prentece)
