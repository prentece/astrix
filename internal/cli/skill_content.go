package cli

import "strings"

// ReferenceRelPath is the path (relative to the skill dir) of the workflows reference.
const ReferenceRelPath = "references/workflows.md"

// md converts the '§' placeholder into a backtick so templates can be written as raw strings.
func md(s string) string { return strings.ReplaceAll(s, "§", "`") }

const skillBody = `# Astrix: code navigation via MCP

Use the **Astrix MCP server** to inspect, navigate, debug and refactor code through AST-based
indexes instead of reading whole files. Astrix indexes **multiple projects**; every tool takes a
**§project_id§**.

## Resolving §project_id§

1. **Current project**: read §.astrix/config.json§ at the repository root and use its §"id"§ field.
2. **Another project** (or if the file is missing): call §list_projects§ and pick the project by
   name/path. Then pass that ID in §project_id§ to navigate it.
3. Never guess or invent an ID. Cache the ID for the rest of the session.

## Tools

| Tool | Required args | Purpose |
|------|---------------|---------|
| §list_projects§ | – | List all indexed projects (IDs, names, paths). |
| §get_project_structure§ | §project_id§ (opt. §path§) | File tree; use to learn the topology. |
| §get_file_outline§ | §project_id§, §filepath§ | Structural outline of declarations (types, functions, methods) with lines and signatures. |
| §lookup_symbol§ | §project_id§, §symbol_name§, §mode§ (§definition§ or §references§) | Find declarations or all usages. |
| §get_implementation§ | §project_id§, §filepath§, §symbol_name§ | Exact source of one function/method/struct/class. |
| §get_implementation_bundle§ | §project_id§, §symbols§ (array or JSON string of §{filepath, symbol_name}§) | Several symbols in one call. |
| §read_file_lines§ | §project_id§, §filepath§, §start_line§, §end_line§ (opt. §anchor_symbol§) | A specific line window. |
| §grep_code§ | §project_id§, §pattern§ (opt. §path_prefix§, §file_extensions§) | Text/regex search. |
| §query_structured_file§ | §project_id§, §filepath§, §query§ | Query JSON/YAML/CSV by path without loading the file. |

All tools accept an optional §format§ (§text§ default, or §json§).

## Best practices

- Use §get_file_outline§ to inspect the declarations and API of a file without reading full function bodies.
- Prefer §lookup_symbol§ + §get_implementation§ over reading entire files.
- Before renaming/refactoring, run §lookup_symbol§ with §mode="references"§ for impact analysis.
- Use §get_implementation_bundle§ instead of several §get_implementation§ calls.
- Use §grep_code§ only when the symbol name is unknown; narrow with §path_prefix§.
- For cross-project questions, resolve the other project's ID and repeat the same tools there.

## Examples

For step-by-step combinations of these tools (explore, trace, impact analysis, cross-project
navigation, config inspection), see [references/workflows.md](references/workflows.md).
`

const referenceBody = `# Astrix tool combination workflows

Placeholders: §<ID>§ = project ID from §.astrix/config.json§ (current project) or from
§list_projects§ (any other project).

## 0. Resolve the project ID

§§§
read .astrix/config.json            -> {"id": "<ID>", "name": "..."}
list_projects {}                    -> use when the project is not the current one
§§§

## 1. Explore an unfamiliar project

1. §get_project_structure {"project_id": "<ID>"}§ – learn top-level layout.
2. §get_project_structure {"project_id": "<ID>", "path": "internal/service"}§ – drill down into folders.
3. §get_file_outline {"project_id": "<ID>", "filepath": "internal/service/code_service.go"}§ – see declarations and signatures.
4. §query_structured_file {"project_id": "<ID>", "filepath": "package.json", "query": "scripts"}§ – entry points/deps.
5. §get_implementation {"project_id": "<ID>", "filepath": "cmd/app/main.go", "symbol_name": "main"}§ – start of execution.

## 2. Inspect a file before reading implementations

1. §get_file_outline {"project_id": "<ID>", "filepath": "pkg/storage/db.go"}§ → returns types, methods, functions with signatures and line ranges.
2. Pick only the specific symbol you need: §get_implementation {"project_id": "<ID>", "filepath": "pkg/storage/db.go", "symbol_name": "NewDatabase"}§.

## 3. Find and read a symbol

1. §lookup_symbol {"project_id": "<ID>", "symbol_name": "UserService", "mode": "definition"}§ → returns filepath + line.
2. §get_implementation {"project_id": "<ID>", "filepath": "<path from step 1>", "symbol_name": "UserService"}§.
3. Need surrounding code? §read_file_lines {"project_id": "<ID>", "filepath": "<path>", "anchor_symbol": "UserService"}§.

## 4. Impact analysis before refactoring

1. §lookup_symbol {"project_id": "<ID>", "symbol_name": "CreateOrder", "mode": "references"}§ – every call site.
2. §get_implementation_bundle {"project_id": "<ID>", "symbols": "[{\"filepath\":\"a.go\",\"symbol_name\":\"CreateOrder\"},{\"filepath\":\"b.go\",\"symbol_name\":\"Handler\"}]"}§ – read definition and main callers at once.
3. Refactor, then repeat step 1 to confirm no stale usages remain.

## 5. Trace a call flow / debug an error

1. §grep_code {"project_id": "<ID>", "pattern": "invalid token", "file_extensions": ".go"}§ – locate the error message.
2. §read_file_lines {"project_id": "<ID>", "filepath": "<hit>", "start_line": <hit-20>, "end_line": <hit+20>}§.
3. §lookup_symbol§ (§references§) on the enclosing function to walk up the callers.

## 6. Cross-project navigation

1. §list_projects {}§ → find the library/service project and its ID (<OTHER_ID>).
2. §lookup_symbol {"project_id": "<OTHER_ID>", "symbol_name": "Client", "mode": "definition"}§.
3. §get_implementation {"project_id": "<OTHER_ID>", ...}§ – read the API you integrate with, then return to <ID>.

## 7. Inspect configuration

- §query_structured_file {"project_id": "<ID>", "filepath": "docker-compose.yml", "query": "services.api.ports"}§
- §query_structured_file {"project_id": "<ID>", "filepath": "package.json", "query": "dependencies"}§

## Anti-patterns

- Hardcoding or guessing a project ID.
- Reading a whole file when §get_file_outline§, §get_implementation§ or §read_file_lines§ suffices.
- Calling §get_implementation§ repeatedly instead of §get_implementation_bundle§.
`

// BuildSkillContent returns the skill/instructions markdown for the given agent.
// It deliberately contains no project ID: IDs are resolved at runtime from
// .astrix/config.json or the list_projects tool.
func BuildSkillContent(agentID string) string {
	var sb strings.Builder
	switch agentID {
	case "antigravity", "claude":
		sb.WriteString("---\nname: astrix\n")
		sb.WriteString("description: Navigate and inspect code across indexed projects using Astrix MCP (AST-based symbol lookup, implementations, grep, structured file queries). Use when exploring, debugging or refactoring code.\n---\n\n")
	case "cursor":
		sb.WriteString("---\ndescription: Code navigation guidelines using the Astrix MCP\nglobs: *\nalwaysApply: true\n---\n\n")
	}
	sb.WriteString(md(skillBody))
	if !HasReferenceFile(agentID) {
		// Single-file agents: inline the workflows.
		sb.WriteString("\n---\n\n")
		sb.WriteString(strings.Replace(md(referenceBody), "# Astrix tool combination workflows", "## Tool combination workflows", 1))
	}
	return sb.String()
}

// BuildReferenceContent returns the content of references/workflows.md.
func BuildReferenceContent() string { return md(referenceBody) }

// HasReferenceFile reports whether the agent uses a skill folder with a references/ directory.
func HasReferenceFile(agentID string) bool {
	return agentID == "antigravity" || agentID == "claude"
}
