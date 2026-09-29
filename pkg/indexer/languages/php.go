package languages

import (
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/php"
)

// PhpSymbolsQuery é a S-expression de símbolos para PHP adaptada da referência CoderLM.
const PhpSymbolsQuery = `
(function_definition
  name: (name) @function.name) @function.def

(method_declaration
  name: (name) @method.name) @method.def

(class_declaration
  name: (name) @class.name) @class.def

(interface_declaration
  name: (name) @interface.name) @interface.def

(trait_declaration
  name: (name) @class.name) @class.def

(enum_declaration
  name: (name) @enum.name) @enum.def

(enum_case
  name: (name) @const.name) @const.def

(const_declaration
  (const_element
    (name) @const.name) @const.def)

(namespace_definition
  name: (namespace_name) @mod.name) @mod.def

(namespace_use_clause
  (qualified_name (name) @import.name) @import.def)

(namespace_use_clause
  (name) @import.name) @import.def
`

// PhpCallersQuery é a S-expression para detecção de chamadas em PHP.
const PhpCallersQuery = `
(function_call_expression
  function: (name) @callee)

(function_call_expression
  function: (qualified_name (name) @callee))

(member_call_expression
  name: (name) @callee)

(scoped_call_expression
  name: (name) @callee)

(nullsafe_member_call_expression
  name: (name) @callee)

(object_creation_expression
  (name) @callee)

(object_creation_expression
  (qualified_name (name) @callee))
`

// PhpImportSitesQuery captura import-sites e type-reference sites em PHP.
// `new ClassName()` já é capturado pelo PhpCallersQuery; aqui cobrimos use statements
// e type hints em parâmetros, propriedades e herança/implementação.
const PhpImportSitesQuery = `
(namespace_use_clause
  (qualified_name
    (name) @callee))

(namespace_use_clause
  (name) @callee)

(simple_parameter
  type: (named_type
    (name) @callee))

(property_promotion_parameter
  type: (named_type
    (name) @callee))

(property_declaration
  type: (named_type
    (name) @callee))

(class_declaration
  base_clause: (qualified_name
    (name) @callee))

(class_implements_clause
  (qualified_name
    (name) @callee))
`

// PhpVariablesQuery é a S-expression para declarações de variáveis locais em PHP.
const PhpVariablesQuery = `
(simple_parameter
  name: (variable_name (name) @var.name))

(property_promotion_parameter
  name: (variable_name (name) @var.name))

(variadic_parameter
  name: (variable_name (name) @var.name))

(assignment_expression
  left: (variable_name (name) @var.name))

(static_variable_declaration
  name: (variable_name (name) @var.name))

(catch_clause
  name: (variable_name (name) @var.name))

(foreach_statement
  (pair
    (variable_name (name) @var.name)
    (variable_name (name) @var.name)))

(foreach_statement
  (variable_name (name) @var.name))
`

// PhpLanguageConfig implementa LanguageConfig para PHP.
type PhpLanguageConfig struct{}

func (c *PhpLanguageConfig) Name() string {
	return "php"
}

func (c *PhpLanguageConfig) GetLanguage() *sitter.Language {
	return php.GetLanguage()
}

func (c *PhpLanguageConfig) SymbolsQuery() string {
	return PhpSymbolsQuery
}

func (c *PhpLanguageConfig) CallersQuery() string {
	return PhpCallersQuery
}

func (c *PhpLanguageConfig) VariablesQuery() string {
	return PhpVariablesQuery
}

func (c *PhpLanguageConfig) TestPatterns() []indexer.TestPattern {
	return []indexer.TestPattern{
		{Type: indexer.TestPatternPrefix, Value: "test"},
		{Type: indexer.TestPatternAttribute, Value: "Test"},
	}
}

func (c *PhpLanguageConfig) Extensions() []string {
	return []string{".php", ".phtml", ".php8"}
}

func (c *PhpLanguageConfig) ExtractDataModels(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DataModel {
	return extractPhpDataModels(projectID, relPath, content, rootNode)
}

func (c *PhpLanguageConfig) ExtractDependencies(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DependencyEdge {
	var edges []*storage.DependencyEdge
	if rootNode == nil {
		return edges
	}

	var walk func(node *sitter.Node, currentClass string)
	walk = func(node *sitter.Node, currentClass string) {
		if node == nil {
			return
		}

		if node.Type() == "class_declaration" || node.Type() == "trait_declaration" {
			nameChild := node.ChildByFieldName("name")
			if nameChild != nil {
				currentClass = nameChild.Content(content)
			}

			// 1. Heritage (extends / implements)
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "base_clause" || child.Type() == "class_interface_clause" {
					extractPhpTypes(projectID, relPath, currentClass, child, "uses", content, &edges)
				}
			}
		}

		// 2. Construtores com Injeção de Dependência
		if node.Type() == "method_declaration" && currentClass != "" {
			nameChild := node.ChildByFieldName("name")
			if nameChild != nil && nameChild.Content(content) == "__construct" {
				paramsNode := node.ChildByFieldName("parameters")
				if paramsNode != nil {
					for i := 0; i < int(paramsNode.ChildCount()); i++ {
						param := paramsNode.Child(i)
						if param.Type() == "simple_parameter" || param.Type() == "property_promotion_parameter" {
							typeChild := param.ChildByFieldName("type")
							if typeChild != nil {
								extractPhpTypes(projectID, relPath, currentClass, typeChild, "injects", content, &edges)
							}
						}
					}
				}
			}
		}

		// 3. Instanciações diretas: new SomeClass(...)
		if node.Type() == "object_creation_expression" && currentClass != "" {
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "name" || child.Type() == "qualified_name" {
					target := child.Content(content)
					if !IsPrimitiveType(target) {
						edges = append(edges, &storage.DependencyEdge{
							ProjectID:        projectID,
							SourceSymbol:     currentClass,
							TargetSymbol:     target,
							SourceFile:       relPath,
							RelationshipType: "instantiates",
						})
					}
					break
				}
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i), currentClass)
		}
	}

	walk(rootNode, "")
	return DeduplicateEdges(edges)
}

func extractPhpTypes(projectID, relPath, currentClass string, node *sitter.Node, relType string, content []byte, edges *[]*storage.DependencyEdge) {
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "name" || n.Type() == "qualified_name" {
			target := n.Content(content)
			if target != currentClass && !IsPrimitiveType(target) {
				*edges = append(*edges, &storage.DependencyEdge{
					ProjectID:        projectID,
					SourceSymbol:     currentClass,
					TargetSymbol:     target,
					SourceFile:       relPath,
					RelationshipType: relType,
				})
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
}

func extractPhpDataModels(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DataModel {
	var modelsList []*storage.DataModel

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		if n.Type() == "class_declaration" {
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				className := nameNode.Content(content)
				lineNum := int(nameNode.StartPoint().Row) + 1
				fields := extractPhpClassFields(n, content)
				if len(fields) > 0 {
					modelsList = append(modelsList, &storage.DataModel{
						ProjectID: projectID,
						Name:      className,
						File:      relPath,
						Kind:      "class",
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

func extractPhpClassFields(classNode *sitter.Node, content []byte) []*storage.ModelField {
	var fields []*storage.ModelField
	body := classNode.ChildByFieldName("body")
	if body == nil {
		body = FindChildByType(classNode, "declaration_list")
	}
	if body == nil {
		return fields
	}

	for i := 0; i < int(body.ChildCount()); i++ {
		member := body.Child(i)
		if member == nil {
			continue
		}

		if member.Type() == "property_declaration" {
			// Procura type e property_element
			typeNode := member.ChildByFieldName("type")
			typeStr := "mixed"
			if typeNode != nil {
				typeStr = cleanPhpType(typeNode.Content(content))
			}
			elem := FindChildByType(member, "property_element")
			if elem != nil {
				nameNode := elem.ChildByFieldName("name")
				if nameNode == nil {
					nameNode = FindChildByType(elem, "variable_name")
				}
				if nameNode != nil {
					fName := strings.TrimPrefix(nameNode.Content(content), "$")
					isOptional := strings.HasPrefix(typeStr, "?") || strings.Contains(elem.Content(content), "null")
					fields = append(fields, &storage.ModelField{
						Name:     fName,
						Type:     typeStr,
						Required: !isOptional,
					})
				}
			}
		} else if member.Type() == "method_declaration" {
			nameNode := member.ChildByFieldName("name")
			if nameNode != nil && nameNode.Content(content) == "__construct" {
				paramsNode := member.ChildByFieldName("parameters")
				if paramsNode == nil {
					paramsNode = FindChildByType(member, "formal_parameters")
				}
				if paramsNode != nil {
					for j := 0; j < int(paramsNode.ChildCount()); j++ {
						param := paramsNode.Child(j)
						if param == nil {
							continue
						}
						pText := param.Content(content)
						if strings.Contains(pText, "public") || strings.Contains(pText, "private") || strings.Contains(pText, "protected") || strings.Contains(pText, "readonly") {
							pTypeNode := param.ChildByFieldName("type")
							pNameNode := param.ChildByFieldName("name")
							if pNameNode == nil {
								pNameNode = FindChildByType(param, "variable_name")
							}
							if pNameNode != nil {
								fName := strings.TrimPrefix(pNameNode.Content(content), "$")
								tStr := "mixed"
								if pTypeNode != nil {
									tStr = cleanPhpType(pTypeNode.Content(content))
								}
								isOpt := strings.HasPrefix(tStr, "?") || strings.Contains(pText, "null")
								fields = append(fields, &storage.ModelField{
									Name:     fName,
									Type:     tStr,
									Required: !isOpt,
								})
							}
						}
					}
				}
			}
		}
	}

	return fields
}

func cleanPhpType(raw string) string {
	raw = strings.TrimSpace(raw)
	return raw
}

// ExtractDigest gera um esqueleto enxuto do arquivo PHP omitindo corpos de métodos.
func (c *PhpLanguageConfig) ExtractDigest(relPath string, content []byte, rootNode *sitter.Node) string {
	if rootNode == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<?php\n// File: " + relPath + "\n")

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		switch n.Type() {
		case "namespace_definition", "use_declaration":
			sb.WriteString(n.Content(content) + "\n\n")
		case "class_declaration", "interface_declaration", "trait_declaration", "enum_declaration":
			nameNode := n.ChildByFieldName("name")
			className := ""
			if nameNode != nil {
				className = nameNode.Content(content)
			}
			kind := "class"
			if n.Type() == "interface_declaration" {
				kind = "interface"
			} else if n.Type() == "trait_declaration" {
				kind = "trait"
			} else if n.Type() == "enum_declaration" {
				kind = "enum"
			}

			sb.WriteString(kind + " " + className + " {\n")

			body := n.ChildByFieldName("body")
			if body == nil {
				body = FindChildByType(n, "declaration_list")
			}
			if body != nil {
				for j := 0; j < int(body.ChildCount()); j++ {
					m := body.Child(j)
					if m == nil {
						continue
					}
					switch m.Type() {
					case "method_declaration":
						mName := m.ChildByFieldName("name")
						mParams := m.ChildByFieldName("parameters")
						if mParams == nil {
							mParams = FindChildByType(m, "formal_parameters")
						}
						mRet := m.ChildByFieldName("return_type")
						if mName != nil {
							sig := "    public function " + mName.Content(content)
							if mParams != nil {
								sig += mParams.Content(content)
							} else {
								sig += "()"
							}
							if mRet != nil {
								sig += ": " + mRet.Content(content)
							}
							sig += " { ... }\n"
							sb.WriteString(sig)
						}
					case "property_declaration":
						sb.WriteString("    " + m.Content(content) + "\n")
					}
				}
			}
			sb.WriteString("}\n\n")
		case "function_definition":
			nameNode := n.ChildByFieldName("name")
			paramsNode := n.ChildByFieldName("parameters")
			if paramsNode == nil {
				paramsNode = FindChildByType(n, "formal_parameters")
			}
			retNode := n.ChildByFieldName("return_type")
			if nameNode != nil {
				sig := "function " + nameNode.Content(content)
				if paramsNode != nil {
					sig += paramsNode.Content(content)
				} else {
					sig += "()"
				}
				if retNode != nil {
					sig += ": " + retNode.Content(content)
				}
				sig += " { ... }\n\n"
				sb.WriteString(sig)
			}
		}

		if n.Type() == "program" {
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i))
			}
		}
	}

	walk(rootNode)
	return strings.TrimSpace(sb.String())
}

func init() {
	indexer.RegisterLanguage(&PhpLanguageConfig{})
}
