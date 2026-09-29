package languages

import (
	"astrix/pkg/storage"
	"path"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// CleanStringLiteral limpa aspas simples, duplas ou crases de literais de string extraídos da AST.
func CleanStringLiteral(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 {
		if (raw[0] == '"' && raw[len(raw)-1] == '"') ||
			(raw[0] == '\'' && raw[len(raw)-1] == '\'') ||
			(raw[0] == '`' && raw[len(raw)-1] == '`') {
			return raw[1 : len(raw)-1]
		}
	}
	return raw
}

// JoinURLPaths junta e normaliza prefixos e rotas HTTP (ex: "api/v1" + "users/:id" -> "/api/v1/users/:id").
func JoinURLPaths(prefix, routePath string) string {
	prefix = CleanStringLiteral(prefix)
	routePath = CleanStringLiteral(routePath)

	prefix = strings.TrimSpace(prefix)
	routePath = strings.TrimSpace(routePath)

	if prefix == "" && routePath == "" {
		return "/"
	}

	combined := path.Join("/", prefix, routePath)
	if !strings.HasPrefix(combined, "/") {
		combined = "/" + combined
	}
	return combined
}

// GetChildNodeByFieldOrType busca recursivamente ou diretamente um filho pelo campo ou tipo de nó.
func FindChildByType(node *sitter.Node, nodeType string) *sitter.Node {
	if node == nil {
		return nil
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}

var primitiveTypes = map[string]bool{
	"string": true, "int": true, "int32": true, "int64": true, "uint": true, "uint64": true,
	"byte": true, "rune": true, "float32": true, "float64": true, "bool": true, "boolean": true,
	"error": true, "number": true, "any": true, "unknown": true, "never": true, "void": true,
	"null": true, "undefined": true, "object": true, "symbol": true, "bigint": true,
	"str": true, "list": true, "dict": true, "set": true, "tuple": true, "self": true, "cls": true,
	"integer": true, "long": true, "double": true, "float": true, "short": true,
	"mixed": true, "callable": true, "iterable": true, "none": true, "context": true,
}

// IsPrimitiveType verifica se o tipo extraído é um tipo primitivo/básico da linguagem.
func IsPrimitiveType(name string) bool {
	name = strings.TrimPrefix(name, "*")
	name = strings.TrimPrefix(name, "[]")
	name = strings.TrimSpace(name)
	return primitiveTypes[strings.ToLower(name)]
}

// DeduplicateEdges remove duplicatas e autoreferências de arestas de dependência.
func DeduplicateEdges(edges []*storage.DependencyEdge) []*storage.DependencyEdge {
	type edgeKey struct {
		source string
		target string
		rel    string
	}
	seen := make(map[edgeKey]bool)
	var result []*storage.DependencyEdge
	for _, e := range edges {
		e.SourceSymbol = strings.TrimSpace(e.SourceSymbol)
		e.TargetSymbol = strings.TrimSpace(e.TargetSymbol)
		e.TargetSymbol = strings.TrimPrefix(e.TargetSymbol, "*")
		e.TargetSymbol = strings.TrimPrefix(e.TargetSymbol, "[]")

		if e.SourceSymbol == "" || e.TargetSymbol == "" || e.SourceSymbol == e.TargetSymbol {
			continue
		}
		if IsPrimitiveType(e.TargetSymbol) {
			continue
		}
		k := edgeKey{source: e.SourceSymbol, target: e.TargetSymbol, rel: e.RelationshipType}
		if !seen[k] {
			seen[k] = true
			result = append(result, e)
		}
	}
	return result
}

// extractImportSitesWithQuery executa uma S-expression Tree-sitter contra rootNode
// e retorna CallerInfo para cada captura @callee encontrada.
// Usada por todas as implementações de ImportSiteExtractor para evitar duplicação de código.
func extractImportSitesWithQuery(
	queryStr string,
	lang *sitter.Language,
	projectID, relPath string,
	content []byte,
	rootNode *sitter.Node,
) []*storage.CallerInfo {
	q, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil {
		return nil
	}
	defer q.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(q, rootNode)

	contentLines := strings.Split(string(content), "\n")
	var results []*storage.CallerInfo

	for {
		match, ok := cursor.NextMatch()
		if !ok || match == nil {
			break
		}
		for _, cap := range match.Captures {
			if q.CaptureNameForId(cap.Index) != "callee" {
				continue
			}
			name := cap.Node.Content(content)
			if name == "" || IsPrimitiveType(name) {
				continue
			}
			lineNum := int(cap.Node.StartPoint().Row) + 1
			var lineText string
			if lineNum-1 < len(contentLines) {
				lineText = strings.TrimSpace(contentLines[lineNum-1])
			}
			results = append(results, &storage.CallerInfo{
				ProjectID:  projectID,
				File:       relPath,
				Line:       lineNum,
				Text:       lineText,
				SymbolName: name,
			})
		}
	}
	return results
}
