package languages_test

import (
	"astrix/pkg/indexer"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/stretchr/testify/require"
)

// Ferramenta de auditoria de cobertura de símbolos (Fase 2 de evolucao-linguagens).
//
// Não altera nenhum comportamento: percorre a AST de arquivos reais e de fixtures, e lista os
// tipos de nó "declarativos" que NENHUMA query de símbolos captura de forma efetiva, isto é,
// que o engine realmente transformaria em símbolo.
//
// Uso (sob demanda; sem a variável o teste é ignorado):
//
//	ASTRIX_AUDIT=1 ASTRIX_AUDIT_OUT=/tmp/audit.md go test -tags sqlite_foreign_keys ./tests/indexer/languages -run TestASTAudit -v
//
// Corpus extra (opcional), no formato "linguagem=diretório" separados por vírgula:
//
//	ASTRIX_AUDIT_CORPUS="python=/path/a,java=/path/b"
//
// Sempre são auditados as fixtures em testdata/audit e, para Go, o próprio repositório.

// declarativeNode seleciona tipos de nó que declaram algo potencialmente indexável.
var declarativeNode = regexp.MustCompile(
	`(_declaration|_definition|_declarator|_spec|_signature|_element|_elem|_case|_constant|_alias|_item)$` +
		`|^(field_declaration|public_field_definition|property_signature|enum_assignment|method_signature|abstract_method_signature|constructor_declaration|annotation_type_declaration|assignment)$`,
)

var auditSkipDirs = map[string]bool{
	"node_modules": true, ".git": true, "vendor": true, "dist": true, "build": true,
	"__pycache__": true, "site-packages": true, ".venv": true, "venv": true,
}

type auditLang struct {
	name string // nome da LanguageConfig
	exts []string
}

var auditLangs = []auditLang{
	{"go", []string{".go"}},
	{"java", []string{".java"}},
	{"php", []string{".php"}},
	{"python", []string{".py"}},
	{"typescript", []string{".ts"}}, // .tsx usa outra gramática; fora do escopo da auditoria
	{"javascript", []string{".js", ".mjs", ".cjs"}},
}

type nodeKey struct {
	start, end uint32
	typ        string
}

func keyOf(n *sitter.Node) nodeKey {
	return nodeKey{start: n.StartByte(), end: n.EndByte(), typ: n.Type()}
}

type nodeStat struct {
	total, covered, ignored int
	examples                []string
}

type auditResult struct {
	label  string
	files  int
	stats  map[string]*nodeStat
	errors []string
}

func newAuditResult(label string) *auditResult {
	return &auditResult{label: label, stats: map[string]*nodeStat{}}
}

func TestASTAudit(t *testing.T) {
	if os.Getenv("ASTRIX_AUDIT") == "" {
		t.Skip("auditoria sob demanda: defina ASTRIX_AUDIT=1")
	}

	extra := parseAuditCorpus(os.Getenv("ASTRIX_AUDIT_CORPUS"))
	var report strings.Builder
	report.WriteString("# Auditoria de cobertura de símbolos\n\n")
	report.WriteString("`covered` = o engine gera símbolo (captura reconhecida). `ignored` = há captura na query, mas com nome que o engine descarta. `uncovered` = nenhuma captura.\n\n")

	for _, al := range auditLangs {
		cfg, ok := indexer.GetConfigByName(al.name)
		require.True(t, ok, "linguagem %s não registrada", al.name)

		fmt.Fprintf(&report, "## %s\n\n", al.name)
		writeQueryHealth(&report, cfg)

		fixtures := newAuditResult("fixtures")
		auditDir(t, cfg, al, filepath.Join("testdata", "audit", al.name), fixtures, 0)
		writeAuditTable(&report, fixtures)

		real := newAuditResult("corpus real")
		if al.name == "go" {
			auditDir(t, cfg, al, filepath.Join("..", "..", ".."), real, 400)
		}
		for _, dir := range extra[al.name] {
			auditDir(t, cfg, al, dir, real, 400)
		}
		if real.files > 0 {
			writeAuditTable(&report, real)
		} else {
			report.WriteString("_Sem corpus real configurado para esta linguagem._\n\n")
		}
	}

	if out := os.Getenv("ASTRIX_AUDIT_OUT"); out != "" {
		require.NoError(t, os.WriteFile(out, []byte(report.String()), 0o644))
		t.Logf("relatório gravado em %s", out)
	} else {
		t.Log("\n" + report.String())
	}
}

func parseAuditCorpus(raw string) map[string][]string {
	out := map[string][]string{}
	for _, part := range strings.Split(raw, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && kv[0] != "" && kv[1] != "" {
			out[kv[0]] = append(out[kv[0]], kv[1])
		}
	}
	return out
}

// writeQueryHealth reporta se as queries compilam e quais nomes de captura o engine descarta.
func writeQueryHealth(sb *strings.Builder, cfg indexer.LanguageConfig) {
	lang := cfg.GetLanguage()
	type namedQuery struct{ label, src string }
	queries := []namedQuery{{"symbols", cfg.SymbolsQuery()}, {"callers", cfg.CallersQuery()}}
	if v, ok := cfg.(interface{ VariablesQuery() string }); ok {
		queries = append(queries, namedQuery{"variables", v.VariablesQuery()})
	}

	for _, nq := range queries {
		if _, err := sitter.NewQuery([]byte(nq.src), lang); err != nil {
			fmt.Fprintf(sb, "- **query `%s` NÃO compila**: %v\n", nq.label, err)
		}
	}

	if q, err := sitter.NewQuery([]byte(cfg.SymbolsQuery()), lang); err == nil {
		seen := map[string]bool{}
		var dropped []string
		for i := uint32(0); i < q.CaptureCount(); i++ {
			name := q.CaptureNameForId(i)
			base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(name, ".name"), ".def"), ".parent")
			_, known := indexer.SymbolCaptureKind(base)
			if !known && !seen[name] {
				seen[name] = true
				dropped = append(dropped, "`@"+name+"`")
			}
		}
		if len(dropped) > 0 {
			sort.Strings(dropped)
			fmt.Fprintf(sb, "- capturas da SymbolsQuery **descartadas pelo engine**: %s\n", strings.Join(dropped, ", "))
		}
	}
	sb.WriteString("\n")
}

func auditDir(t *testing.T, cfg indexer.LanguageConfig, al auditLang, root string, res *auditResult, maxFiles int) {
	t.Helper()
	if _, err := os.Stat(root); err != nil {
		res.errors = append(res.errors, fmt.Sprintf("diretório indisponível: %s", root))
		return
	}

	symbols, err := sitter.NewQuery([]byte(cfg.SymbolsQuery()), cfg.GetLanguage())
	if err != nil {
		res.errors = append(res.errors, fmt.Sprintf("SymbolsQuery inválida: %v", err))
		return
	}

	count := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if auditSkipDirs[d.Name()] || (d.Name() == "testdata" && maxFiles > 0) {
				return filepath.SkipDir
			}
			return nil
		}
		if maxFiles > 0 && count >= maxFiles {
			return filepath.SkipAll
		}
		if !hasAnyExt(path, al.exts) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 400*1024 {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		auditFile(cfg, symbols, path, content, res)
		count++
		return nil
	})
}

func hasAnyExt(path string, exts []string) bool {
	lower := strings.ToLower(path)
	for _, e := range exts {
		if strings.HasSuffix(lower, e) {
			return true
		}
	}
	return false
}

func auditFile(cfg indexer.LanguageConfig, symbols *sitter.Query, path string, content []byte, res *auditResult) {
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(cfg.GetLanguage())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil || tree == nil {
		res.errors = append(res.errors, fmt.Sprintf("parse falhou: %s", path))
		return
	}
	defer tree.Close()
	root := tree.RootNode()
	res.files++

	covered := map[nodeKey]bool{} // capturas que o engine reconhece
	ignored := map[nodeKey]bool{} // capturas que o engine descarta

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(symbols, root)
	for {
		match, ok := cursor.NextMatch()
		if !ok || match == nil {
			break
		}
		for _, c := range match.Captures {
			name := symbols.CaptureNameForId(c.Index)
			base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(name, ".name"), ".def"), ".parent")
			target := ignored
			if _, ok := indexer.SymbolCaptureKind(base); ok {
				target = covered
			}
			switch {
			case strings.HasSuffix(name, ".def"):
				target[keyOf(c.Node)] = true
			case strings.HasSuffix(name, ".name"):
				if p := c.Node.Parent(); p != nil {
					target[keyOf(p)] = true
				}
			}
		}
	}

	walkAudit(root, func(n *sitter.Node) {
		if !n.IsNamed() || !declarativeNode.MatchString(n.Type()) {
			return
		}
		st := res.stats[n.Type()]
		if st == nil {
			st = &nodeStat{}
			res.stats[n.Type()] = st
		}
		st.total++
		k := keyOf(n)
		switch {
		case covered[k]:
			st.covered++
		case ignored[k]:
			st.ignored++
		default:
			if len(st.examples) < 2 {
				st.examples = append(st.examples, fmt.Sprintf("%s:%d", filepath.Base(path), n.StartPoint().Row+1))
			}
		}
	})
}

func walkAudit(n *sitter.Node, visit func(*sitter.Node)) {
	visit(n)
	for i := 0; i < int(n.ChildCount()); i++ {
		walkAudit(n.Child(i), visit)
	}
}

func writeAuditTable(sb *strings.Builder, res *auditResult) {
	fmt.Fprintf(sb, "### %s (%d arquivos)\n\n", res.label, res.files)
	for _, e := range res.errors {
		fmt.Fprintf(sb, "- aviso: %s\n", e)
	}
	if res.files == 0 {
		sb.WriteString("_Nenhum arquivo auditado._\n\n")
		return
	}

	types := make([]string, 0, len(res.stats))
	for typ := range res.stats {
		types = append(types, typ)
	}
	sort.Slice(types, func(i, j int) bool {
		a, b := res.stats[types[i]], res.stats[types[j]]
		ua, ub := a.total-a.covered, b.total-b.covered
		if ua != ub {
			return ua > ub
		}
		return types[i] < types[j]
	})

	sb.WriteString("| tipo de nó | total | covered | ignored | uncovered | exemplos |\n|---|---:|---:|---:|---:|---|\n")
	for _, typ := range types {
		st := res.stats[typ]
		uncovered := st.total - st.covered - st.ignored
		fmt.Fprintf(sb, "| `%s` | %d | %d | %d | %d | %s |\n", typ, st.total, st.covered, st.ignored, uncovered, strings.Join(st.examples, ", "))
	}
	sb.WriteString("\n")
}
