package languages

import (
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

// PythonSymbolsQuery é a S-expression de símbolos para Python adaptada da referência CoderLM.
const PythonSymbolsQuery = `
(function_definition
  name: (identifier) @function.name) @function.def

(decorated_definition
  definition: (function_definition
    name: (identifier) @function.name)) @function.def

(class_definition
  name: (identifier) @class.name) @class.def

(decorated_definition
  definition: (class_definition
    name: (identifier) @class.name)) @class.def

(expression_statement
  (assignment
    left: (identifier) @const.name)) @const.def

(import_from_statement
  name: (dotted_name) @import.name) @import.def
`

// PythonCallersQuery é a S-expression para detecção de chamadas em Python.
const PythonCallersQuery = `
(call
  function: (identifier) @callee)

(call
  function: (attribute
    attribute: (identifier) @callee))
`

// PythonImportSitesQuery captura import-sites e type-reference sites em Python.
// Cobre: from x import Cls, import x, herança de classes e type hints.
const PythonImportSitesQuery = `
(import_from_statement
  name: (dotted_name) @callee)

(import_statement
  name: (dotted_name) @callee)

(class_definition
  superclasses: (argument_list
    (identifier) @callee))

(typed_parameter
  type: (type) @callee)

(typed_default_parameter
  type: (type) @callee)

(function_definition
  return_type: (type) @callee)
`

// PythonVariablesQuery é a S-expression para declarações de variáveis locais em Python.
const PythonVariablesQuery = `
(assignment
  left: (identifier) @var.name)

(assignment
  left: (pattern_list
    (identifier) @var.name))

(assignment
  left: (tuple_pattern
    (identifier) @var.name))

(for_statement
  left: (identifier) @var.name)

(for_statement
  left: (tuple_pattern
    (identifier) @var.name))

(with_item
  (as_pattern
    alias: (as_pattern_target
      (identifier) @var.name)))

(parameters
  (identifier) @var.name)

(parameters
  (default_parameter
    name: (identifier) @var.name))

(parameters
  (typed_parameter
    (identifier) @var.name))

(parameters
  (typed_default_parameter
    name: (identifier) @var.name))
`

// PythonLanguageConfig implementa LanguageConfig para Python.
type PythonLanguageConfig struct{}

func (c *PythonLanguageConfig) Name() string {
	return "python"
}

func (c *PythonLanguageConfig) GetLanguage() *sitter.Language {
	return python.GetLanguage()
}

func (c *PythonLanguageConfig) SymbolsQuery() string {
	return PythonSymbolsQuery
}

func (c *PythonLanguageConfig) CallersQuery() string {
	return PythonCallersQuery
}

func (c *PythonLanguageConfig) VariablesQuery() string {
	return PythonVariablesQuery
}

func (c *PythonLanguageConfig) TestPatterns() []indexer.TestPattern {
	return []indexer.TestPattern{
		{Type: indexer.TestPatternPrefix, Value: "test_"},
	}
}

func (c *PythonLanguageConfig) Extensions() []string {
	return []string{".py", ".pyi"}
}

func (c *PythonLanguageConfig) ExtractImportSites(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.CallerInfo {
	return extractImportSitesWithQuery(PythonImportSitesQuery, python.GetLanguage(), projectID, relPath, content, rootNode)
}

func (c *PythonLanguageConfig) ExtractDataModels(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DataModel {
	return extractPythonDataModels(projectID, relPath, content, rootNode)
}

func (c *PythonLanguageConfig) ExtractDependencies(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DependencyEdge {
	var edges []*storage.DependencyEdge
	if rootNode == nil {
		return edges
	}

	var walk func(node *sitter.Node, currentClass, currentFunc string)
	walk = func(node *sitter.Node, currentClass, currentFunc string) {
		if node == nil {
			return
		}

		if node.Type() == "class_definition" {
			nameChild := node.ChildByFieldName("name")
			if nameChild != nil {
				currentClass = nameChild.Content(content)
			}

			// 1. Superclasses (heritage)
			argsNode := node.ChildByFieldName("superclasses")
			if argsNode != nil {
				for i := 0; i < int(argsNode.ChildCount()); i++ {
					arg := argsNode.Child(i)
					if arg.Type() == "identifier" || arg.Type() == "attribute" {
						target := arg.Content(content)
						if target != currentClass && !IsPrimitiveType(target) {
							edges = append(edges, &storage.DependencyEdge{
								ProjectID:        projectID,
								SourceSymbol:     currentClass,
								TargetSymbol:     target,
								SourceFile:       relPath,
								RelationshipType: "uses",
							})
						}
					}
				}
			}
		}

		if node.Type() == "function_definition" {
			nameChild := node.ChildByFieldName("name")
			if nameChild != nil {
				currentFunc = nameChild.Content(content)
			}

			// 2. Parâmetros com Type Hints ou Depends (FastAPI / DI)
			paramsNode := node.ChildByFieldName("parameters")
			if paramsNode != nil {
				source := currentClass
				if source == "" || currentFunc != "__init__" {
					if currentFunc != "" {
						source = currentFunc
					}
				}

				if source != "" {
					extractPythonParams(projectID, relPath, source, paramsNode, content, &edges)
				}
			}
		}

		// 3. Instanciação direta / chamadas de classes (PascalCase)
		if node.Type() == "call" && currentClass != "" {
			fnChild := node.ChildByFieldName("function")
			if fnChild != nil && (fnChild.Type() == "identifier" || fnChild.Type() == "attribute") {
				fnName := fnChild.Content(content)
				// Se começa com Maiúscula e não é primitivo
				if len(fnName) > 0 && fnName[0] >= 'A' && fnName[0] <= 'Z' && !IsPrimitiveType(fnName) && fnName != currentClass {
					edges = append(edges, &storage.DependencyEdge{
						ProjectID:        projectID,
						SourceSymbol:     currentClass,
						TargetSymbol:     fnName,
						SourceFile:       relPath,
						RelationshipType: "instantiates",
					})
				}
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i), currentClass, currentFunc)
		}
	}

	walk(rootNode, "", "")
	return DeduplicateEdges(edges)
}

func extractPythonParams(projectID, relPath, sourceSymbol string, paramsNode *sitter.Node, content []byte, edges *[]*storage.DependencyEdge) {
	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		param := paramsNode.Child(i)
		paramText := param.Content(content)
		if paramText == "self" || paramText == "cls" {
			continue
		}

		// FastApi Depends(Target)
		if strings.Contains(paramText, "Depends(") {
			idx := strings.Index(paramText, "Depends(")
			sub := paramText[idx+8:]
			endIdx := strings.Index(sub, ")")
			if endIdx != -1 {
				depTarget := strings.TrimSpace(sub[:endIdx])
				if depTarget != "" && !IsPrimitiveType(depTarget) && depTarget != sourceSymbol {
					*edges = append(*edges, &storage.DependencyEdge{
						ProjectID:        projectID,
						SourceSymbol:     sourceSymbol,
						TargetSymbol:     depTarget,
						SourceFile:       relPath,
						RelationshipType: "injects",
					})
				}
			}
		}

		// Type annotation
		if param.Type() == "typed_parameter" || param.Type() == "typed_default_parameter" {
			typeChild := param.ChildByFieldName("type")
			if typeChild != nil {
				target := typeChild.Content(content)
				target = strings.TrimPrefix(target, "Optional[")
				target = strings.TrimSuffix(target, "]")
				target = strings.TrimSpace(target)
				if target != "" && !IsPrimitiveType(target) && target != sourceSymbol {
					*edges = append(*edges, &storage.DependencyEdge{
						ProjectID:        projectID,
						SourceSymbol:     sourceSymbol,
						TargetSymbol:     target,
						SourceFile:       relPath,
						RelationshipType: "injects",
					})
				}
			}
		}
	}
}

func extractPythonDataModels(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DataModel {
	var modelsList []*storage.DataModel

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		if n.Type() == "class_definition" {
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				className := nameNode.Content(content)
				lineNum := int(nameNode.StartPoint().Row) + 1
				fields := extractPythonClassFields(n, content)
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

func extractPythonClassFields(classNode *sitter.Node, content []byte) []*storage.ModelField {
	var fields []*storage.ModelField
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return fields
	}

	for i := 0; i < int(body.ChildCount()); i++ {
		stmt := body.Child(i)
		if stmt == nil {
			continue
		}

		// Expressões de atribuição / tipo no nível da classe (Pydantic / Dataclasses)
		if stmt.Type() == "expression_statement" {
			for j := 0; j < int(stmt.ChildCount()); j++ {
				child := stmt.Child(j)
				if child.Type() == "assignment" {
					left := child.ChildByFieldName("left")
					typeNode := child.ChildByFieldName("type")
					right := child.ChildByFieldName("right")

					if left != nil {
						fieldName := left.Content(content)
						if !strings.HasPrefix(fieldName, "_") {
							typeStr := "Any"
							if typeNode != nil {
								typeStr = cleanPythonType(typeNode.Content(content))
							}
							isOptional := right != nil || strings.Contains(typeStr, "Optional[") || strings.Contains(typeStr, "None")
							fields = append(fields, &storage.ModelField{
								Name:     fieldName,
								Type:     typeStr,
								Required: !isOptional,
							})
						}
					}
				} else if child.Type() == "type" {
					// Campo tipado sem valor default: name: str
					text := child.Content(content)
					parts := strings.SplitN(text, ":", 2)
					if len(parts) == 2 {
						fName := strings.TrimSpace(parts[0])
						fType := cleanPythonType(parts[1])
						if !strings.HasPrefix(fName, "_") {
							isOptional := strings.Contains(fType, "Optional[") || strings.Contains(fType, "None")
							fields = append(fields, &storage.ModelField{
								Name:     fName,
								Type:     fType,
								Required: !isOptional,
							})
						}
					}
				}
			}
		} else if stmt.Type() == "function_definition" {
			nameNode := stmt.ChildByFieldName("name")
			if nameNode != nil && nameNode.Content(content) == "__init__" && len(fields) == 0 {
				params := stmt.ChildByFieldName("parameters")
				if params != nil {
					for j := 0; j < int(params.ChildCount()); j++ {
						p := params.Child(j)
						if p == nil {
							continue
						}
						pType := p.Type()
						if pType == "typed_parameter" || pType == "typed_default_parameter" {
							pNameNode := p.ChildByFieldName("name")
							pTypeNode := p.ChildByFieldName("type")
							if pNameNode != nil {
								pName := pNameNode.Content(content)
								if pName != "self" && pName != "cls" {
									tStr := "Any"
									if pTypeNode != nil {
										tStr = cleanPythonType(pTypeNode.Content(content))
									}
									isOpt := pType == "typed_default_parameter" || strings.Contains(tStr, "Optional[")
									fields = append(fields, &storage.ModelField{
										Name:     pName,
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
	}

	return fields
}

func cleanPythonType(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, ":")
	raw = strings.TrimSpace(raw)
	return raw
}

// ExtractDigest gera um esqueleto enxuto do arquivo Python omitindo corpos de funções.
func (c *PythonLanguageConfig) ExtractDigest(relPath string, content []byte, rootNode *sitter.Node) string {
	if rootNode == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# File: " + relPath + "\n")

	for i := 0; i < int(rootNode.ChildCount()); i++ {
		child := rootNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "import_statement", "import_from_statement":
			sb.WriteString(child.Content(content) + "\n\n")
		case "class_definition":
			nameNode := child.ChildByFieldName("name")
			superNode := child.ChildByFieldName("superclasses")
			className := ""
			if nameNode != nil {
				className = nameNode.Content(content)
			}
			classSig := "class " + className
			if superNode != nil {
				classSig += superNode.Content(content)
			}
			classSig += ":\n"
			sb.WriteString(classSig)

			body := child.ChildByFieldName("body")
			if body != nil {
				for j := 0; j < int(body.ChildCount()); j++ {
					stmt := body.Child(j)
					if stmt == nil {
						continue
					}
					if stmt.Type() == "function_definition" {
						fnName := stmt.ChildByFieldName("name")
						fnParams := stmt.ChildByFieldName("parameters")
						fnRet := stmt.ChildByFieldName("return_type")
						if fnName != nil {
							sig := "    def " + fnName.Content(content)
							if fnParams != nil {
								sig += fnParams.Content(content)
							} else {
								sig += "()"
							}
							if fnRet != nil {
								sig += " -> " + fnRet.Content(content)
							}
							sig += ": ...\n"
							sb.WriteString(sig)
						}
					}
				}
			}
			sb.WriteString("\n")
		case "function_definition":
			nameNode := child.ChildByFieldName("name")
			paramsNode := child.ChildByFieldName("parameters")
			retNode := child.ChildByFieldName("return_type")
			if nameNode != nil {
				sig := "def " + nameNode.Content(content)
				if paramsNode != nil {
					sig += paramsNode.Content(content)
				} else {
					sig += "()"
				}
				if retNode != nil {
					sig += " -> " + retNode.Content(content)
				}
				sig += ": ...\n\n"
				sb.WriteString(sig)
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

func init() {
	indexer.RegisterLanguage(&PythonLanguageConfig{})
}
