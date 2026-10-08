package languages

import (
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// GoSymbolsQuery é a S-expression de símbolos para Go adaptada da referência CoderLM.
const GoSymbolsQuery = `
(function_declaration
  name: (identifier) @function.name) @function.def

(method_declaration
  name: (field_identifier) @method.name) @method.def

(type_declaration
  (type_spec
    name: (type_identifier) @struct.name
    type: (struct_type))) @struct.def

(type_declaration
  (type_spec
    name: (type_identifier) @interface.name
    type: (interface_type))) @interface.def

(type_declaration
  (type_spec
    name: (type_identifier) @type.name)) @type.def

(const_declaration
  (const_spec
    name: (identifier) @const.name)) @const.def

(var_declaration
  (var_spec
    name: (identifier) @var.name)) @var.def
`

// GoCallersQuery é a S-expression para detecção de chamadas de funções/métodos em Go.
const GoCallersQuery = `
(call_expression
  function: (identifier) @callee)

(call_expression
  function: (selector_expression
    field: (field_identifier) @callee))
`

// GoImportSitesQuery captura import-sites e type-reference sites em Go.
// Cobre: declarações de variável com tipo, composite literals, parâmetros e campos de struct.
const GoImportSitesQuery = `
(var_declaration
  (var_spec
    type: (type_identifier) @callee))

(composite_literal
  type: (type_identifier) @callee)

(parameter_declaration
  type: (type_identifier) @callee)

(parameter_declaration
  type: (pointer_type
    (type_identifier) @callee))

(field_declaration
  type: (type_identifier) @callee)

(field_declaration
  type: (pointer_type
    (type_identifier) @callee))
`

// GoVariablesQuery é a S-expression para declarações de variáveis locais em Go.
const GoVariablesQuery = `
(short_var_declaration
  left: (expression_list
    (identifier) @var.name))

(var_declaration
  (var_spec
    name: (identifier) @var.name))

(range_clause
  left: (expression_list
    (identifier) @var.name))

(parameter_declaration
  name: (identifier) @var.name)
`

// GoLanguageConfig implementa LanguageConfig para Go.
type GoLanguageConfig struct{}

func (c *GoLanguageConfig) Name() string {
	return "go"
}

func (c *GoLanguageConfig) GetLanguage() *sitter.Language {
	return golang.GetLanguage()
}

func (c *GoLanguageConfig) SymbolsQuery() string {
	return GoSymbolsQuery
}

func (c *GoLanguageConfig) CallersQuery() string {
	return GoCallersQuery
}

func (c *GoLanguageConfig) VariablesQuery() string {
	return GoVariablesQuery
}

func (c *GoLanguageConfig) TestPatterns() []indexer.TestPattern {
	return []indexer.TestPattern{
		{Type: indexer.TestPatternPrefix, Value: "Test"},
	}
}

func (c *GoLanguageConfig) Extensions() []string {
	return []string{".go"}
}

func (c *GoLanguageConfig) ExtractImportSites(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.CallerInfo {
	return extractImportSitesWithQuery(GoImportSitesQuery, golang.GetLanguage(), projectID, relPath, content, rootNode)
}

func (c *GoLanguageConfig) ExtractDataModels(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DataModel {
	return extractGoDataModels(projectID, relPath, content, rootNode)
}

func (c *GoLanguageConfig) ExtractDependencies(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DependencyEdge {
	var edges []*storage.DependencyEdge
	if rootNode == nil {
		return edges
	}

	var walk func(node *sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		// 1. Structs e campos (struct embedding / dependency fields)
		if node.Type() == "type_spec" {
			nameChild := node.ChildByFieldName("name")
			typeChild := node.ChildByFieldName("type")

			if nameChild != nil && typeChild != nil && typeChild.Type() == "struct_type" {
				structName := nameChild.Content(content)
				for i := 0; i < int(typeChild.ChildCount()); i++ {
					fList := typeChild.Child(i)
					if fList.Type() == "field_declaration_list" {
						for j := 0; j < int(fList.ChildCount()); j++ {
							field := fList.Child(j)
							if field.Type() == "field_declaration" {
								fType := field.ChildByFieldName("type")
								if fType != nil {
									targetType := extractGoTypeName(fType, content)
									if targetType != "" && targetType != structName && !IsPrimitiveType(targetType) {
										edges = append(edges, &storage.DependencyEdge{
											ProjectID:        projectID,
											SourceSymbol:     structName,
											TargetSymbol:     targetType,
											SourceFile:       relPath,
											RelationshipType: "injects",
										})
									}
								} else {
									// Embedded field
									targetType := extractGoTypeName(field, content)
									if targetType != "" && targetType != structName && !IsPrimitiveType(targetType) {
										edges = append(edges, &storage.DependencyEdge{
											ProjectID:        projectID,
											SourceSymbol:     structName,
											TargetSymbol:     targetType,
											SourceFile:       relPath,
											RelationshipType: "uses",
										})
									}
								}
							}
						}
					}
				}
			}
		}

		// 2. Factory Constructors: func NewService(repo Repository) *Service
		if node.Type() == "function_declaration" {
			nameChild := node.ChildByFieldName("name")
			if nameChild != nil {
				funcName := nameChild.Content(content)
				if strings.HasPrefix(funcName, "New") {
					sourceSymbol := strings.TrimPrefix(funcName, "New")
					if sourceSymbol == "" {
						sourceSymbol = funcName
					}

					// Se retorna explicitamente um tipo de struct
					resultNode := node.ChildByFieldName("result")
					if resultNode != nil {
						retType := extractGoTypeName(resultNode, content)
						if retType != "" && !IsPrimitiveType(retType) {
							sourceSymbol = retType
						}
					}

					paramsNode := node.ChildByFieldName("parameters")
					if paramsNode != nil {
						for i := 0; i < int(paramsNode.ChildCount()); i++ {
							param := paramsNode.Child(i)
							if param.Type() == "parameter_declaration" {
								pType := param.ChildByFieldName("type")
								if pType != nil {
									targetType := extractGoTypeName(pType, content)
									if targetType != "" && targetType != sourceSymbol && !IsPrimitiveType(targetType) {
										edges = append(edges, &storage.DependencyEdge{
											ProjectID:        projectID,
											SourceSymbol:     sourceSymbol,
											TargetSymbol:     targetType,
											SourceFile:       relPath,
											RelationshipType: "injects",
										})
									}
								}
							}
						}
					}
				}
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(rootNode)
	return DeduplicateEdges(edges)
}

func extractGoTypeName(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	var res string
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil || res != "" {
			return
		}
		if n.Type() == "type_identifier" {
			res = n.Content(content)
			return
		}
		if n.Type() == "field_identifier" {
			res = n.Content(content)
			return
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
	return res
}

func extractGoDataModels(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DataModel {
	var modelsList []*storage.DataModel

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		if n.Type() == "type_spec" {
			nameNode := n.ChildByFieldName("name")
			typeNode := n.ChildByFieldName("type")

			if nameNode != nil && typeNode != nil && typeNode.Type() == "struct_type" {
				structName := nameNode.Content(content)
				lineNum := int(nameNode.StartPoint().Row) + 1
				fields := extractGoStructFields(typeNode, content)
				if len(fields) > 0 {
					modelsList = append(modelsList, &storage.DataModel{
						ProjectID: projectID,
						Name:      structName,
						File:      relPath,
						Kind:      "struct",
						Line:      lineNum,
						Fields:    fields,
					})
				}
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(rootNode)
	return modelsList
}

func extractGoStructFields(structNode *sitter.Node, content []byte) []*storage.ModelField {
	var fields []*storage.ModelField
	fieldList := FindChildByType(structNode, "field_declaration_list")
	if fieldList == nil {
		return fields
	}

	for i := 0; i < int(fieldList.ChildCount()); i++ {
		fieldDecl := fieldList.Child(i)
		if fieldDecl == nil || fieldDecl.Type() != "field_declaration" {
			continue
		}

		nameNode := fieldDecl.ChildByFieldName("name")
		typeNode := fieldDecl.ChildByFieldName("type")
		tagNode := fieldDecl.ChildByFieldName("tag")

		tagStr := ""
		if tagNode != nil {
			tagStr = strings.Trim(tagNode.Content(content), "`\"")
		}

		typeStr := "interface{}"
		if typeNode != nil {
			typeStr = typeNode.Content(content)
		}

		fieldName := ""
		if nameNode != nil {
			fieldName = nameNode.Content(content)
		} else if typeNode != nil {
			// Embedded struct field
			fieldName = typeNode.Content(content)
		}

		if fieldName != "" {
			isOptional := strings.HasPrefix(typeStr, "*") || strings.Contains(tagStr, "omitempty")
			fields = append(fields, &storage.ModelField{
				Name:     fieldName,
				Type:     typeStr,
				Required: !isOptional,
				Tag:      tagStr,
			})
		}
	}

	return fields
}

// ExtractDigest gera um esqueleto enxuto do arquivo Go omitindo corpos de funções.
func (c *GoLanguageConfig) ExtractDigest(relPath string, content []byte, rootNode *sitter.Node) string {
	if rootNode == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("// File: ")
	sb.WriteString(relPath)
	sb.WriteString("\n")

	for i := 0; i < int(rootNode.ChildCount()); i++ {
		child := rootNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "package_clause":
			sb.WriteString(child.Content(content))
			sb.WriteString("\n\n")
		case "import_declaration":
			sb.WriteString(child.Content(content))
			sb.WriteString("\n\n")
		case "type_declaration":
			sb.WriteString(child.Content(content))
			sb.WriteString("\n\n")
		case "function_declaration":
			nameNode := child.ChildByFieldName("name")
			paramsNode := child.ChildByFieldName("parameters")
			resultNode := child.ChildByFieldName("result")

			sig := "func "
			if nameNode != nil {
				sig += nameNode.Content(content)
			}
			if paramsNode != nil {
				sig += paramsNode.Content(content)
			} else {
				sig += "()"
			}
			if resultNode != nil {
				sig += " " + resultNode.Content(content)
			}
			sig += " { ... }\n"
			sb.WriteString(sig)
		case "method_declaration":
			recvNode := child.ChildByFieldName("receiver")
			nameNode := child.ChildByFieldName("name")
			paramsNode := child.ChildByFieldName("parameters")
			resultNode := child.ChildByFieldName("result")

			sig := "func "
			if recvNode != nil {
				sig += recvNode.Content(content) + " "
			}
			if nameNode != nil {
				sig += nameNode.Content(content)
			}
			if paramsNode != nil {
				sig += paramsNode.Content(content)
			} else {
				sig += "()"
			}
			if resultNode != nil {
				sig += " " + resultNode.Content(content)
			}
			sig += " { ... }\n"
			sb.WriteString(sig)
		}
	}

	return strings.TrimSpace(sb.String())
}

func init() {
	indexer.RegisterLanguage(&GoLanguageConfig{})
}
