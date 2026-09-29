package indexer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"
)

// QueryStructuredFile consulta nós/chaves específicas em arquivos estruturados (JSON, YAML, CSV)
// sem despejar o arquivo completo no contexto.
func QueryStructuredFile(rootDir, relPath, query string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("caminho do arquivo (filepath) é obrigatório")
	}
	if query == "" {
		return "", fmt.Errorf("expressão de busca (query) é obrigatória")
	}

	absPath := filepath.Join(rootDir, relPath)
	ext := strings.ToLower(filepath.Ext(relPath))

	switch ext {
	case ".json":
		return queryJSON(absPath, query)
	case ".yaml", ".yml":
		return queryYAML(absPath, query)
	case ".csv":
		return queryCSV(absPath, query)
	default:
		// Tenta detectar pelo conteúdo ou extensão
		data, err := os.ReadFile(absPath)
		if err != nil {
			return "", fmt.Errorf("falha ao ler arquivo '%s': %w", relPath, err)
		}
		trimmed := strings.TrimSpace(string(data))
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			return queryJSON(absPath, query)
		}
		return queryYAML(absPath, query)
	}
}

// queryJSON executa queries GJSON em arquivos .json.
func queryJSON(absPath, query string) (string, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("falha ao ler arquivo JSON: %w", err)
	}

	res := gjson.GetBytes(data, query)
	if !res.Exists() {
		return fmt.Sprintf("[Not Found] Chave ou caminho '%s' não encontrado no JSON", query), nil
	}

	// Se for primitivo (string, número, booleano, null)
	if res.Type == gjson.String || res.Type == gjson.Number || res.Type == gjson.True || res.Type == gjson.False || res.Type == gjson.Null {
		return res.String(), nil
	}

	// Se for objeto ou array, retorna JSON minificado
	var compactBuf bytes.Buffer
	if err := json.Compact(&compactBuf, []byte(res.Raw)); err == nil {
		return compactBuf.String(), nil
	}

	return res.Raw, nil
}

// queryYAML navega na AST do YAML usando gopkg.in/yaml.v3 e retorna o nó encontrado.
func queryYAML(absPath, query string) (string, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("falha ao ler arquivo YAML: %w", err)
	}

	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return "", fmt.Errorf("falha ao interpretar YAML: %w", err)
	}

	parts := strings.Split(query, ".")
	targetNode := findYAMLNode(&rootNode, parts)
	if targetNode == nil {
		return fmt.Sprintf("[Not Found] Chave ou caminho '%s' não encontrado no YAML", query), nil
	}

	if targetNode.Kind == yaml.ScalarNode {
		return targetNode.Value, nil
	}

	out, err := yaml.Marshal(targetNode)
	if err != nil {
		return "", fmt.Errorf("falha ao serializar resultado YAML: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// findYAMLNode navega recursivamente na árvore yaml.Node procurando o caminho delimitado por pontos.
func findYAMLNode(node *yaml.Node, parts []string) *yaml.Node {
	if node == nil || len(parts) == 0 {
		return node
	}

	// DocumentNode: desce para o nó raiz do documento
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return nil
		}
		return findYAMLNode(node.Content[0], parts)
	}

	currentKey := parts[0]
	remainingParts := parts[1:]

	switch node.Kind {
	case yaml.MappingNode:
		// Em MappingNode, node.Content é [key1, val1, key2, val2, ...]
		for i := 0; i < len(node.Content); i += 2 {
			kNode := node.Content[i]
			vNode := node.Content[i+1]
			if kNode.Value == currentKey {
				if len(remainingParts) == 0 {
					return vNode
				}
				return findYAMLNode(vNode, remainingParts)
			}
		}
		return nil

	case yaml.SequenceNode:
		// Em SequenceNode, target pode ser índice numérico
		if idx, err := strconv.Atoi(currentKey); err == nil && idx >= 0 && idx < len(node.Content) {
			if len(remainingParts) == 0 {
				return node.Content[idx]
			}
			return findYAMLNode(node.Content[idx], remainingParts)
		}
		return nil

	default:
		return nil
	}
}

// queryCSV processa CSVs via streaming suportando filtros de coluna ou seleção de campos.
func queryCSV(absPath, query string) (string, error) {
	file, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("falha ao abrir arquivo CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return "", fmt.Errorf("falha ao ler cabeçalho do CSV: %w", err)
	}

	maxRows := 20
	var sb strings.Builder

	// Caso 1: Filtro por valor de coluna -> filter:status=ACTIVE ou status=ACTIVE
	filterQuery := query
	if strings.HasPrefix(filterQuery, "filter:") {
		filterQuery = strings.TrimPrefix(filterQuery, "filter:")
	}
	if strings.Contains(filterQuery, "=") {
		parts := strings.SplitN(filterQuery, "=", 2)
		colName := strings.TrimSpace(parts[0])
		targetVal := strings.TrimSpace(parts[1])

		colIdx := -1
		for i, h := range header {
			if strings.EqualFold(strings.TrimSpace(h), colName) {
				colIdx = i
				break
			}
		}

		if colIdx == -1 {
			return fmt.Sprintf("[Not Found] Coluna '%s' não encontrada no CSV", colName), nil
		}

		sb.WriteString(strings.Join(header, ",") + "\n")
		matched := 0
		for {
			row, err := reader.Read()
			if err != nil {
				break
			}
			if colIdx < len(row) && strings.EqualFold(strings.TrimSpace(row[colIdx]), targetVal) {
				sb.WriteString(strings.Join(row, ",") + "\n")
				matched++
				if matched >= maxRows {
					fmt.Fprintf(&sb, "[... truncado em %d linhas]\n", maxRows)
					break
				}
			}
		}
		if matched == 0 {
			return fmt.Sprintf("[Not Found] Nenhum registro encontrado para %s=%s", colName, targetVal), nil
		}
		return sb.String(), nil
	}

	// Caso 2: Seleção de colunas -> columns:id,name ou id,name
	colsQuery := query
	if strings.HasPrefix(colsQuery, "columns:") {
		colsQuery = strings.TrimPrefix(colsQuery, "columns:")
	}
	if strings.Contains(colsQuery, ",") {
		requestedCols := strings.Split(colsQuery, ",")
		var colIndices []int
		var matchedHeaders []string

		for _, reqCol := range requestedCols {
			reqTrimmed := strings.TrimSpace(reqCol)
			for i, h := range header {
				if strings.EqualFold(strings.TrimSpace(h), reqTrimmed) {
					colIndices = append(colIndices, i)
					matchedHeaders = append(matchedHeaders, h)
					break
				}
			}
		}

		if len(colIndices) == 0 {
			return fmt.Sprintf("[Not Found] Nenhuma das colunas '%s' foi encontrada", colsQuery), nil
		}

		sb.WriteString(strings.Join(matchedHeaders, ",") + "\n")
		count := 0
		for {
			row, err := reader.Read()
			if err != nil {
				break
			}
			var rowSubset []string
			for _, idx := range colIndices {
				if idx < len(row) {
					rowSubset = append(rowSubset, row[idx])
				} else {
					rowSubset = append(rowSubset, "")
				}
			}
			sb.WriteString(strings.Join(rowSubset, ",") + "\n")
			count++
			if count >= maxRows {
				fmt.Fprintf(&sb, "[... truncado em %d linhas]\n", maxRows)
				break
			}
		}
		return sb.String(), nil
	}

	// Caso 3: Coluna única -> exibe valores daquela coluna
	colName := strings.TrimSpace(query)
	colIdx := -1
	for i, h := range header {
		if strings.EqualFold(strings.TrimSpace(h), colName) {
			colIdx = i
			break
		}
	}

	if colIdx != -1 {
		sb.WriteString(header[colIdx] + "\n")
		count := 0
		for {
			row, err := reader.Read()
			if err != nil {
				break
			}
			if colIdx < len(row) {
				sb.WriteString(row[colIdx] + "\n")
				count++
				if count >= maxRows {
					fmt.Fprintf(&sb, "[... truncado em %d linhas]\n", maxRows)
					break
				}
			}
		}
		return sb.String(), nil
	}

	// Caso 4: Busca por palavra-chave em qualquer coluna
	sb.WriteString(strings.Join(header, ",") + "\n")
	matched := 0
	for {
		row, err := reader.Read()
		if err != nil {
			break
		}
		rowStr := strings.Join(row, " ")
		if strings.Contains(strings.ToLower(rowStr), strings.ToLower(query)) {
			sb.WriteString(strings.Join(row, ",") + "\n")
			matched++
			if matched >= maxRows {
				fmt.Fprintf(&sb, "[... truncado em %d linhas]\n", maxRows)
				break
			}
		}
	}

	if matched == 0 {
		return fmt.Sprintf("[Not Found] Nenhum registro encontrado para '%s'", query), nil
	}

	return sb.String(), nil
}
