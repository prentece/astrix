package languages

import (
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
)

// JavaSymbolsQuery é a S-expression de símbolos para Java adaptada da referência CoderLM.
const JavaSymbolsQuery = `
(class_declaration
  name: (identifier) @class.name) @class.def

(interface_declaration
  name: (identifier) @interface.name) @interface.def

(enum_declaration
  name: (identifier) @enum.name) @enum.def

(record_declaration
  name: (identifier) @record.name) @record.def

(method_declaration
  name: (identifier) @method.name) @method.def

(constructor_declaration
  name: (identifier) @constructor.name) @constructor.def
`

// JavaCallersQuery é a S-expression para detecção de chamadas em Java.
const JavaCallersQuery = `
(method_invocation
  name: (identifier) @callee)

(object_creation_expression
  type: (type_identifier) @callee)
`

// JavaImportSitesQuery captura import-sites e type-reference sites em Java.
// `new ClassName()` já é capturado pelo JavaCallersQuery; aqui cobrimos import declarations
// e referências de tipo em campos, parâmetros, variáveis locais e herança/implementação.
const JavaImportSitesQuery = `
(import_declaration
  (scoped_identifier
    name: (identifier) @callee))

(field_declaration
  type: (type_identifier) @callee)

(formal_parameter
  type: (type_identifier) @callee)

(local_variable_declaration
  type: (type_identifier) @callee)

(class_declaration
  superclass: (type_identifier) @callee)

(class_declaration
  interfaces: (super_interfaces
    (type_list
      (type_identifier) @callee)))
`

// JavaVariablesQuery é a S-expression para declarações de variáveis locais em Java.
const JavaVariablesQuery = `
(local_variable_declaration
  declarator: (variable_declarator
    name: (identifier) @var.name))

(enhanced_for_statement
  name: (identifier) @var.name)

(formal_parameter
  name: (identifier) @var.name)
`

// JavaLanguageConfig implementa LanguageConfig para Java.
type JavaLanguageConfig struct{}

func (c *JavaLanguageConfig) Name() string {
	return "java"
}

func (c *JavaLanguageConfig) GetLanguage() *sitter.Language {
	return java.GetLanguage()
}

func (c *JavaLanguageConfig) SymbolsQuery() string {
	return JavaSymbolsQuery
}

func (c *JavaLanguageConfig) CallersQuery() string {
	return JavaCallersQuery
}

func (c *JavaLanguageConfig) VariablesQuery() string {
	return JavaVariablesQuery
}

func (c *JavaLanguageConfig) TestPatterns() []indexer.TestPattern {
	return []indexer.TestPattern{
		{Type: indexer.TestPatternAttribute, Value: "Test"},
		{Type: indexer.TestPatternAttribute, Value: "org.junit"},
	}
}

func (c *JavaLanguageConfig) Extensions() []string {
	return []string{".java"}
}

func (c *JavaLanguageConfig) ExtractDataModels(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DataModel {
	return extractJavaDataModels(projectID, relPath, content, rootNode)
}

func (c *JavaLanguageConfig) ExtractDependencies(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DependencyEdge {
	var edges []*storage.DependencyEdge
	if rootNode == nil {
		return edges
	}

	var walk func(node *sitter.Node, currentClass string)
	walk = func(node *sitter.Node, currentClass string) {
		if node == nil {
			return
		}

		if node.Type() == "class_declaration" || node.Type() == "interface_declaration" {
			nameChild := node.ChildByFieldName("name")
			if nameChild != nil {
				currentClass = nameChild.Content(content)
			}

			// 1. Superclass e SuperInterfaces (extends / implements)
			superClass := node.ChildByFieldName("superclass")
			if superClass != nil {
				extractJavaTypes(projectID, relPath, currentClass, superClass, "uses", content, &edges)
			}
			interfaces := node.ChildByFieldName("interfaces")
			if interfaces != nil {
				extractJavaTypes(projectID, relPath, currentClass, interfaces, "uses", content, &edges)
			}
		}

		// 2. Injeção de Campos (@Autowired / @Inject)
		if node.Type() == "field_declaration" && currentClass != "" {
			isAutowired := false
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "modifiers" {
					modText := child.Content(content)
					if strings.Contains(modText, "@Autowired") || strings.Contains(modText, "@Inject") {
						isAutowired = true
						break
					}
				}
			}

			if isAutowired {
				typeChild := node.ChildByFieldName("type")
				if typeChild != nil {
					extractJavaTypes(projectID, relPath, currentClass, typeChild, "injects", content, &edges)
				}
			}
		}

		// 3. Parâmetros do Construtor
		if node.Type() == "constructor_declaration" && currentClass != "" {
			paramsNode := node.ChildByFieldName("parameters")
			if paramsNode != nil {
				for i := 0; i < int(paramsNode.ChildCount()); i++ {
					param := paramsNode.Child(i)
					if param.Type() == "formal_parameter" {
						typeChild := param.ChildByFieldName("type")
						if typeChild != nil {
							extractJavaTypes(projectID, relPath, currentClass, typeChild, "injects", content, &edges)
						}
					}
				}
			}
		}

		// 4. Instanciação direta: new Target(...)
		if node.Type() == "object_creation_expression" && currentClass != "" {
			typeChild := node.ChildByFieldName("type")
			if typeChild != nil {
				target := typeChild.Content(content)
				if !IsPrimitiveType(target) {
					edges = append(edges, &storage.DependencyEdge{
						ProjectID:        projectID,
						SourceSymbol:     currentClass,
						TargetSymbol:     target,
						SourceFile:       relPath,
						RelationshipType: "instantiates",
					})
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

func extractJavaTypes(projectID, relPath, currentClass string, node *sitter.Node, relType string, content []byte, edges *[]*storage.DependencyEdge) {
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "type_identifier" {
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

func extractJavaDataModels(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DataModel {
	var modelsList []*storage.DataModel

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		if n.Type() == "record_declaration" {
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				recName := nameNode.Content(content)
				lineNum := int(nameNode.StartPoint().Row) + 1
				fields := extractJavaRecordFields(n, content)
				if len(fields) > 0 {
					modelsList = append(modelsList, &storage.DataModel{
						ProjectID: projectID,
						Name:      recName,
						File:      relPath,
						Kind:      "record",
						Line:      lineNum,
						Fields:    fields,
					})
				}
			}
		} else if n.Type() == "class_declaration" {
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				className := nameNode.Content(content)
				lineNum := int(nameNode.StartPoint().Row) + 1
				fields := extractJavaClassFields(n, content)
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

func extractJavaRecordFields(recNode *sitter.Node, content []byte) []*storage.ModelField {
	var fields []*storage.ModelField
	paramsNode := recNode.ChildByFieldName("parameters")
	if paramsNode == nil {
		paramsNode = FindChildByType(recNode, "formal_parameters")
	}
	if paramsNode == nil {
		return fields
	}

	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		param := paramsNode.Child(i)
		if param != nil && param.Type() == "formal_parameter" {
			typeNode := param.ChildByFieldName("type")
			nameNode := param.ChildByFieldName("name")
			if nameNode != nil {
				fieldName := nameNode.Content(content)
				typeStr := "Object"
				if typeNode != nil {
					typeStr = cleanJavaType(typeNode.Content(content))
				}
				isOptional := strings.Contains(param.Content(content), "@Nullable") || strings.Contains(typeStr, "Optional<")
				fields = append(fields, &storage.ModelField{
					Name:     fieldName,
					Type:     typeStr,
					Required: !isOptional,
				})
			}
		}
	}
	return fields
}

func extractJavaClassFields(classNode *sitter.Node, content []byte) []*storage.ModelField {
	var fields []*storage.ModelField
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return fields
	}

	for i := 0; i < int(body.ChildCount()); i++ {
		member := body.Child(i)
		if member != nil && member.Type() == "field_declaration" {
			typeNode := member.ChildByFieldName("type")
			declarator := FindChildByType(member, "variable_declarator")
			if declarator != nil {
				nameNode := declarator.ChildByFieldName("name")
				if nameNode != nil {
					fieldName := nameNode.Content(content)
					typeStr := "Object"
					if typeNode != nil {
						typeStr = cleanJavaType(typeNode.Content(content))
					}
					isOptional := strings.Contains(member.Content(content), "@Nullable") || strings.Contains(typeStr, "Optional<")
					fields = append(fields, &storage.ModelField{
						Name:     fieldName,
						Type:     typeStr,
						Required: !isOptional,
					})
				}
			}
		}
	}
	return fields
}

func cleanJavaType(raw string) string {
	raw = strings.TrimSpace(raw)
	return raw
}

// ExtractDigest gera um esqueleto enxuto do arquivo Java omitindo corpos de métodos.
func (c *JavaLanguageConfig) ExtractDigest(relPath string, content []byte, rootNode *sitter.Node) string {
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
		case "package_declaration", "import_declaration":
			sb.WriteString(child.Content(content))
			sb.WriteString("\n\n")
		case "class_declaration", "interface_declaration", "record_declaration", "enum_declaration":
			nameNode := child.ChildByFieldName("name")
			className := ""
			if nameNode != nil {
				className = nameNode.Content(content)
			}
			kind := "class"
			if child.Type() == "interface_declaration" {
				kind = "interface"
			} else if child.Type() == "record_declaration" {
				kind = "record"
			} else if child.Type() == "enum_declaration" {
				kind = "enum"
			}

			sb.WriteString("public ")
			sb.WriteString(kind)
			sb.WriteString(" ")
			sb.WriteString(className)
			sb.WriteString(" {\n")

			body := child.ChildByFieldName("body")
			if body != nil {
				for j := 0; j < int(body.ChildCount()); j++ {
					m := body.Child(j)
					if m == nil {
						continue
					}
					switch m.Type() {
					case "method_declaration", "constructor_declaration":
						mName := m.ChildByFieldName("name")
						mParams := m.ChildByFieldName("parameters")
						mType := m.ChildByFieldName("type")
						if mName != nil {
							sig := "    "
							if mType != nil {
								sig += mType.Content(content) + " "
							}
							sig += mName.Content(content)
							if mParams != nil {
								sig += mParams.Content(content)
							} else {
								sig += "()"
							}
							sig += " { ... }\n"
							sb.WriteString(sig)
						}
					case "field_declaration":
						sb.WriteString("    ")
						sb.WriteString(m.Content(content))
						sb.WriteString("\n")
					}
				}
			}
			sb.WriteString("}\n\n")
		}
	}

	return strings.TrimSpace(sb.String())
}

func init() {
	indexer.RegisterLanguage(&JavaLanguageConfig{})
}
