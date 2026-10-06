package mcp

import (
	"bytes"
	"astrix/pkg/storage"
	"astrix/internal/service"
	"encoding/json"
	"fmt"
	"strings"
)

// LeanSymbol representa a projeção enxuta de um símbolo sem metadados internos.
type LeanSymbol struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Signature string `json:"signature,omitempty"`
}

// LeanCaller representa a referência de chamada sem identificadores de banco.
type LeanCaller struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// PaginationInfo contém os metadados de paginação para o LLM.
type PaginationInfo struct {
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	Count      int  `json:"count"`
	HasMore    bool `json:"has_more"`
	NextOffset *int `json:"next_offset,omitempty"`
}

type PagedSymbolsResponse struct {
	Symbols    []LeanSymbol   `json:"symbols"`
	Pagination PaginationInfo `json:"pagination"`
}

type PagedReferencesResponse struct {
	References []LeanCaller   `json:"references"`
	Pagination PaginationInfo `json:"pagination"`
}

type PagedGrepResponse struct {
	Matches    []storage.GrepMatch `json:"matches"`
	Pagination PaginationInfo     `json:"pagination"`
}

// FormatSymbols compacta a lista de símbolos AST no formato grep/ctags ou JSON enxuto com paginação.
func FormatSymbols(symbols []*storage.Symbol, asJSON bool, limit, offset int, hasMore bool) string {
	if len(symbols) == 0 {
		if offset > 0 {
			return fmt.Sprintf("No more symbols found at offset %d.", offset)
		}
		return "No symbols found."
	}

	var nextOffset *int
	if hasMore {
		next := offset + len(symbols)
		nextOffset = &next
	}

	if asJSON {
		var list []LeanSymbol
		for _, s := range symbols {
			list = append(list, LeanSymbol{
				File:      s.File,
				StartLine: s.StartLine,
				EndLine:   s.EndLine,
				Kind:      string(s.Kind),
				Name:      s.Name,
				Signature: s.Signature,
			})
		}
		return ToCleanJSON(PagedSymbolsResponse{
			Symbols: list,
			Pagination: PaginationInfo{
				Limit:      limit,
				Offset:     offset,
				Count:      len(list),
				HasMore:    hasMore,
				NextOffset: nextOffset,
			},
		})
	}

	var sb strings.Builder
	for i, s := range symbols {
		if i > 0 {
			sb.WriteString("\n")
		}

		lineRange := fmt.Sprintf("%d", s.StartLine)
		if s.EndLine > s.StartLine {
			lineRange = fmt.Sprintf("%d-%d", s.StartLine, s.EndLine)
		}

		desc := s.Name
		if s.Signature != "" && s.Signature != s.Name {
			desc = s.Signature
		}

		fmt.Fprintf(&sb, "%s:%s [%s] %s", s.File, lineRange, s.Kind, desc)
	}

	if hasMore {
		fmt.Fprintf(&sb, "\n\n[Showing %d results (offset: %d). More results available: call with offset=%d]", len(symbols), offset, offset+len(symbols))
	} else if offset > 0 {
		fmt.Fprintf(&sb, "\n\n[Showing %d results (offset: %d). End of results]", len(symbols), offset)
	}

	return sb.String()
}

// FormatReferences compacta a lista de chamadores/referências com suporte a paginação.
func FormatReferences(symbolName string, refs []*storage.CallerInfo, asJSON bool, limit, offset int, hasMore bool) string {
	if len(refs) == 0 {
		if offset > 0 {
			return fmt.Sprintf("No more references found for symbol '%s' at offset %d.", symbolName, offset)
		}
		return fmt.Sprintf("No references found for symbol '%s'.", symbolName)
	}

	var nextOffset *int
	if hasMore {
		next := offset + len(refs)
		nextOffset = &next
	}

	if asJSON {
		var list []LeanCaller
		for _, r := range refs {
			list = append(list, LeanCaller{
				File: r.File,
				Line: r.Line,
				Text: strings.TrimSpace(r.Text),
			})
		}
		return ToCleanJSON(PagedReferencesResponse{
			References: list,
			Pagination: PaginationInfo{
				Limit:      limit,
				Offset:     offset,
				Count:      len(list),
				HasMore:    hasMore,
				NextOffset: nextOffset,
			},
		})
	}

	var sb strings.Builder
	for i, r := range refs {
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "%s:%d: %s", r.File, r.Line, strings.TrimSpace(r.Text))
	}

	if hasMore {
		fmt.Fprintf(&sb, "\n\n[Showing %d references (offset: %d). More references available: call with offset=%d]", len(refs), offset, offset+len(refs))
	} else if offset > 0 {
		fmt.Fprintf(&sb, "\n\n[Showing %d references (offset: %d). End of results]", len(refs), offset)
	}

	return sb.String()
}

// FormatGrepMatches compacta resultados de busca textual regex com suporte a paginação e context lines.
func FormatGrepMatches(pattern string, matches []storage.GrepMatch, asJSON bool, limit, offset int, hasMore bool) string {
	if len(matches) == 0 {
		if offset > 0 {
			return fmt.Sprintf("No more matches found for pattern '%s' at offset %d.", pattern, offset)
		}
		return fmt.Sprintf("No matches found for pattern '%s'.", pattern)
	}

	var nextOffset *int
	if hasMore {
		next := offset + len(matches)
		nextOffset = &next
	}

	if asJSON {
		return ToCleanJSON(PagedGrepResponse{
			Matches: matches,
			Pagination: PaginationInfo{
				Limit:      limit,
				Offset:     offset,
				Count:      len(matches),
				HasMore:    hasMore,
				NextOffset: nextOffset,
			},
		})
	}

	// Dedup no texto: contextos sobrepostos de matches próximos não devem
	// repetir (file,line) nem reimprimir uma linha que é main desta página.
	type lineKey struct {
		file string
		line int
	}
	mainSet := make(map[lineKey]struct{}, len(matches))
	for _, m := range matches {
		mainSet[lineKey{m.File, m.Line}] = struct{}{}
	}
	seen := make(map[lineKey]struct{}, len(matches)*3)
	var out []string

	emitContext := func(file string, lineNum int, content string) {
		k := lineKey{file, lineNum}
		if _, ok := seen[k]; ok {
			return
		}
		if _, isMain := mainSet[k]; isMain {
			return
		}
		seen[k] = struct{}{}
		out = append(out, fmt.Sprintf("%s:%d- %s", file, lineNum, content))
	}
	emitMain := func(file string, lineNum int, content string) {
		k := lineKey{file, lineNum}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		out = append(out, fmt.Sprintf("%s:%d: %s", file, lineNum, strings.TrimSpace(content)))
	}

	for _, m := range matches {
		for ci, ctx := range m.ContextBefore {
			ctxLine := m.Line - len(m.ContextBefore) + ci
			emitContext(m.File, ctxLine, ctx)
		}
		emitMain(m.File, m.Line, m.Content)
		for ci, ctx := range m.ContextAfter {
			emitContext(m.File, m.Line+ci+1, ctx)
		}
	}

	var sb strings.Builder
	sb.WriteString(strings.Join(out, "\n"))

	if hasMore {
		fmt.Fprintf(&sb, "\n\n[Showing %d matches (offset: %d). More matches available: call with offset=%d]", len(matches), offset, offset+len(matches))
	} else if offset > 0 {
		fmt.Fprintf(&sb, "\n\n[Showing %d matches (offset: %d). End of results]", len(matches), offset)
	}

	return sb.String()
}

// ToCleanJSON serializa estruturas sem escape de caracteres HTML (< e > não viram \u003c).
func ToCleanJSON(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		raw, _ := json.MarshalIndent(v, "", "  ")
		return string(raw)
	}
	return strings.TrimRight(buf.String(), "\n")
}


// LeanProject representa a projeção ultra-enxuta de um projeto registrado.
type LeanProject struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Language    string `json:"language"`
	Status      string `json:"status"`
	FileCount   int    `json:"file_count"`
	SymbolCount int    `json:"symbol_count"`
	Path        string `json:"path"`
}

// FormatProjects formata a lista de projetos em formato tabular alinhado ou JSON enxuto.
func FormatProjects(projects []*storage.Project, asJSON bool) string {
	if len(projects) == 0 {
		return "No registered projects found. Register a repository via the Web Dashboard at http://localhost:8080 or the REST API."
	}

	if asJSON {
		lean := make([]LeanProject, 0, len(projects))
		for _, p := range projects {
			lean = append(lean, LeanProject{
				ID:          p.ID,
				Name:        p.Name,
				Language:    p.Language,
				Status:      string(p.Status),
				FileCount:   p.FileCount,
				SymbolCount: p.SymbolCount,
				Path:        p.Path,
			})
		}
		return ToCleanJSON(lean)
	}

	maxNameLen := 4
	maxLangLen := 8
	maxStatusLen := 6

	type row struct {
		id      string
		name    string
		lang    string
		status  string
		files   string
		symbols string
		path    string
	}

	rows := make([]row, len(projects))
	for i, p := range projects {
		filesStr := fmt.Sprintf("%d", p.FileCount)
		symbolsStr := fmt.Sprintf("%d", p.SymbolCount)
		rows[i] = row{
			id:      p.ID,
			name:    p.Name,
			lang:    p.Language,
			status:  string(p.Status),
			files:   filesStr,
			symbols: symbolsStr,
			path:    p.Path,
		}
		if len(p.Name) > maxNameLen {
			maxNameLen = len(p.Name)
		}
		if len(p.Language) > maxLangLen {
			maxLangLen = len(p.Language)
		}
		if len(p.Status) > maxStatusLen {
			maxStatusLen = len(p.Status)
		}
	}

	var sb strings.Builder
	headerFmt := fmt.Sprintf("%%-36s  %%-%ds  %%-%ds  %%-%ds  %%-5s  %%-7s  %%s\n", maxNameLen, maxLangLen, maxStatusLen)
	fmt.Fprintf(&sb, headerFmt, "PROJECT_ID", "NAME", "LANGUAGE", "STATUS", "FILES", "SYMBOLS", "PATH")

	for _, rw := range rows {
		fmt.Fprintf(&sb, headerFmt, rw.id, rw.name, rw.lang, rw.status, rw.files, rw.symbols, rw.path)
	}

	return strings.TrimRight(sb.String(), "\n")
}

// FormatArchitectureGraph formata a árvore de dependências em visualização tree compacta ou JSON.
func FormatArchitectureGraph(root *service.ArchitectureGraphNode, direction string, maxDepth int, asJSON bool) string {
	if root == nil {
		return "No architecture graph available."
	}

	if asJSON {
		return ToCleanJSON(root)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Dependency Graph for %s (direction: %s, max_depth: %d):\n", root.Symbol, direction, maxDepth)

	loc := ""
	if root.File != "" {
		if root.Line > 0 {
			loc = fmt.Sprintf(" (%s:%d)", root.File, root.Line)
		} else {
			loc = fmt.Sprintf(" (%s)", root.File)
		}
	}
	fmt.Fprintf(&sb, "%s%s\n", root.Symbol, loc)

	formatGraphTreeNodes(&sb, root.Dependencies, "")

	return strings.TrimRight(sb.String(), "\n")
}

func formatGraphTreeNodes(sb *strings.Builder, children []*service.ArchitectureGraphNode, prefix string) {
	for i, child := range children {
		isLast := i == len(children)-1
		connector := "├── "
		subPrefix := "│   "
		if isLast {
			connector = "└── "
			subPrefix = "    "
		}

		rel := ""
		if child.RelType != "" {
			rel = fmt.Sprintf("[%s] ", child.RelType)
		}

		loc := ""
		if child.File != "" {
			if child.Line > 0 {
				loc = fmt.Sprintf(" (%s:%d)", child.File, child.Line)
			} else {
				loc = fmt.Sprintf(" (%s)", child.File)
			}
		}

		cycle := ""
		if child.IsCycle {
			cycle = " [cycle]"
		}

		fmt.Fprintf(sb, "%s%s%s%s%s%s\n", prefix, connector, rel, child.Symbol, loc, cycle)

		if len(child.Dependencies) > 0 {
			formatGraphTreeNodes(sb, child.Dependencies, prefix+subPrefix)
		}
	}
}

// FormatDataModel formata a definição do modelo/DTO em texto ultracompacto ou JSON limpo.
func FormatDataModel(model *storage.DataModel, asJSON bool) string {
	if model == nil {
		return "Data model not found."
	}

	if asJSON {
		return ToCleanJSON(model)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "[%s:%d] %s %s {\n", model.File, model.Line, model.Kind, model.Name)
	for _, f := range model.Fields {
		reqStr := "required"
		optMark := ""
		if !f.Required {
			reqStr = "optional"
			optMark = "?"
		}
		fmt.Fprintf(&sb, "  %s%s: %s (%s)\n", f.Name, optMark, f.Type, reqStr)
	}
	sb.WriteString("}")
	return sb.String()
}


// FormatStructureTree formata a árvore do projeto em texto hierárquico ou JSON.
func FormatStructureTree(treeText string, asJSON bool) string {
	if asJSON {
		return ToCleanJSON(map[string]any{
			"tree": treeText,
		})
	}
	return treeText
}

// FormatImplementation formata a extração cirúrgica de código AST em texto raw ou JSON.
func FormatImplementation(symbolName, filePath, code string, asJSON bool) string {
	if asJSON {
		return ToCleanJSON(map[string]string{
			"symbol_name": symbolName,
			"filepath":    filePath,
			"code":        code,
		})
	}
	return code
}

// FormatPeekFile formata a leitura cirúrgica de linhas com overlay de contexto AST em texto ou JSON.
func FormatPeekFile(filePath string, startLine, endLine int, anchorSymbol, content string, asJSON bool) string {
	if asJSON {
		res := map[string]any{
			"filepath":   filePath,
			"start_line": startLine,
			"content":    content,
		}
		if endLine > 0 {
			res["end_line"] = endLine
		}
		if anchorSymbol != "" {
			res["anchor_symbol"] = anchorSymbol
		}
		return ToCleanJSON(res)
	}
	return content
}

// FormatStructuredFileResult formata o resultado de consulta estruturada (JSON/YAML/CSV) em texto ou JSON.
func FormatStructuredFileResult(filePath, query, result string, asJSON bool) string {
	if asJSON {
		var parsed any
		if json.Unmarshal([]byte(result), &parsed) == nil {
			return ToCleanJSON(map[string]any{
				"filepath": filePath,
				"query":    query,
				"result":   parsed,
			})
		}
		return ToCleanJSON(map[string]any{
			"filepath": filePath,
			"query":    query,
			"result":   result,
		})
	}
	return result
}

// BundleResultJSON representa a projeção enxuta de um BundleResult para saída JSON.
type BundleResultJSON struct {
	Filepath   string `json:"filepath"`
	SymbolName string `json:"symbol_name"`
	Code       string `json:"code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// FormatImplementationBundle formata os resultados de uma consulta de implementação em lote.
// Texto: seções separadas por cabeçalho "### filepath :: symbol_name".
// JSON: array de {filepath, symbol_name, code, error?}.
func FormatImplementationBundle(results []service.BundleResult, asJSON bool) string {
	if len(results) == 0 {
		return "No symbols found in bundle."
	}
	if asJSON {
		lean := make([]BundleResultJSON, 0, len(results))
		for _, r := range results {
			lean = append(lean, BundleResultJSON{
				Filepath:   r.Filepath,
				SymbolName: r.SymbolName,
				Code:       r.Code,
				Error:      r.Error,
			})
		}
		return ToCleanJSON(map[string]any{"results": lean, "count": len(lean)})
	}

	var sb strings.Builder
	total := len(results)
	ok := 0
	for _, r := range results {
		if r.Error == "" {
			ok++
		}
	}
	sb.WriteString(fmt.Sprintf("[Bundle: %d/%d symbols retrieved]\n\n", ok, total))
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("### %s :: %s\n", r.Filepath, r.SymbolName))
		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("ERROR: %s\n\n", r.Error))
		} else {
			sb.WriteString(r.Code)
			sb.WriteString("\n\n")
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// FormatFileOutline formata o outline estrutural de declarações de um arquivo.
func FormatFileOutline(filePath string, symbols []*storage.Symbol, asJSON bool) string {
	if len(symbols) == 0 {
		return fmt.Sprintf("No symbols found in %s.", filePath)
	}
	if asJSON {
		var list []LeanSymbol
		for _, s := range symbols {
			list = append(list, LeanSymbol{
				File:      s.File,
				StartLine: s.StartLine,
				EndLine:   s.EndLine,
				Kind:      string(s.Kind),
				Name:      s.Name,
				Signature: s.Signature,
			})
		}
		return ToCleanJSON(map[string]any{"filepath": filePath, "symbols": list, "count": len(list)})
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Outline for %s (%d declarations):\n", filePath, len(symbols)))
	for _, s := range symbols {
		lines := fmt.Sprintf("L%d", s.StartLine)
		if s.EndLine > s.StartLine {
			lines = fmt.Sprintf("L%d-%d", s.StartLine, s.EndLine)
		}
		sb.WriteString(fmt.Sprintf("  %-8s %-12s %s\n", lines, s.Kind, s.Name))
		if s.Signature != "" && s.Signature != s.Name {
			sb.WriteString(fmt.Sprintf("           sig: %s\n", s.Signature))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}
