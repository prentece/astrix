package languages

import (
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// TypeScriptSymbolsQuery é a S-expression de símbolos para TypeScript.
const TypeScriptSymbolsQuery = `
(function_declaration
  name: (identifier) @function.name) @function.def

(class_declaration
  name: (type_identifier) @class.name) @class.def

(method_definition
  name: (property_identifier) @method.name) @method.def

(lexical_declaration
  (variable_declarator
    name: (identifier) @const.name
    value: (arrow_function))) @const.def

(lexical_declaration
  (variable_declarator
    name: (identifier) @function.name
    value: (function_expression))) @function.def

(interface_declaration
  name: (type_identifier) @interface.name) @interface.def

(type_alias_declaration
  name: (type_identifier) @type.name) @type.def

(enum_declaration
  name: (identifier) @enum.name) @enum.def
` // TypeScriptCallersQuery é a S-expression para detecção de chamadas em TypeScript.

const TypeScriptCallersQuery = `
(call_expression
  function: (identifier) @callee)

(call_expression
  function: (member_expression
    property: (property_identifier) @callee))
`

// TypeScriptImportSitesQuery captura import-sites e type-reference sites de símbolos de classe/tipo em TypeScript.
// Cobre: import nomeado, new expression, type annotations, extends e implements.
const TypeScriptImportSitesQuery = `
(import_statement
  (import_clause
    (named_imports
      (import_specifier
        name: (identifier) @callee))))

(new_expression
  constructor: (identifier) @callee)

(new_expression
  constructor: (member_expression
    property: (property_identifier) @callee))

(type_annotation
  (type_identifier) @callee)

(extends_clause
  (identifier) @callee)

(implements_clause
  (type_identifier) @callee)
`

// TypeScriptVariablesQuery é a S-expression para variáveis locais em TypeScript.
const TypeScriptVariablesQuery = `
(variable_declarator
  name: (identifier) @var.name)

(variable_declarator
  name: (object_pattern
    (shorthand_property_identifier_pattern) @var.name))

(variable_declarator
  name: (array_pattern
    (identifier) @var.name))

(for_in_statement
  left: (identifier) @var.name)

(for_in_statement
  left: (lexical_declaration
    (variable_declarator
      name: (identifier) @var.name))))

(required_parameter
  pattern: (identifier) @var.name)

(optional_parameter
  pattern: (identifier) @var.name)
`

// TypeScriptLanguageConfig implementa LanguageConfig para TypeScript.
type TypeScriptLanguageConfig struct{}

func (c *TypeScriptLanguageConfig) Name() string {
	return "typescript"
}

func (c *TypeScriptLanguageConfig) GetLanguage() *sitter.Language {
	return typescript.GetLanguage()
}

func (c *TypeScriptLanguageConfig) SymbolsQuery() string {
	return TypeScriptSymbolsQuery
}

func (c *TypeScriptLanguageConfig) CallersQuery() string {
	return TypeScriptCallersQuery
}

func (c *TypeScriptLanguageConfig) ExtractImportSites(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.CallerInfo {
	return extractImportSitesWithQuery(TypeScriptImportSitesQuery, typescript.GetLanguage(), projectID, relPath, content, rootNode)
}

func (c *TypeScriptLanguageConfig) VariablesQuery() string {
	return TypeScriptVariablesQuery
}

func (c *TypeScriptLanguageConfig) TestPatterns() []indexer.TestPattern {
	return []indexer.TestPattern{
		{Type: indexer.TestPatternCallExpression, Value: "it"},
		{Type: indexer.TestPatternCallExpression, Value: "test"},
		{Type: indexer.TestPatternCallExpression, Value: "describe"},
	}
}

func (c *TypeScriptLanguageConfig) Extensions() []string {
	return []string{".ts", ".tsx"}
}

func (c *TypeScriptLanguageConfig) ExtractDataModels(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DataModel {
	return extractJSTSDataModels(projectID, relPath, content, rootNode)
}

func (c *JavaScriptLanguageConfig) ExtractDataModels(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DataModel {
	return extractJSTSDataModels(projectID, relPath, content, rootNode)
}

// JavaScriptSymbolsQuery é a S-expression de símbolos para JavaScript.
const JavaScriptSymbolsQuery = `
(function_declaration
  name: (identifier) @function.name) @function.def

(class_declaration
  name: (identifier) @class.name) @class.def

(method_definition
  name: (property_identifier) @method.name) @method.def

(lexical_declaration
  (variable_declarator
    name: (identifier) @const.name
    value: (arrow_function))) @const.def

(lexical_declaration
  (variable_declarator
    name: (identifier) @function.name
    value: (function_expression))) @function.def

(variable_declaration
  (variable_declarator
    name: (identifier) @const.name
    value: (arrow_function))) @const.def

(variable_declaration
  (variable_declarator
    name: (identifier) @function.name
    value: (function_expression))) @function.def

(assignment_expression
  left: (member_expression
    property: (property_identifier) @function.name)
  right: (function_expression)) @function.def

(assignment_expression
  left: (member_expression
    property: (property_identifier) @function.name)
  right: (arrow_function)) @function.def
`

// JavaScriptCallersQuery é a S-expression para chamadas em JavaScript.
const JavaScriptCallersQuery = `
(call_expression
  function: (identifier) @callee)

(call_expression
  function: (member_expression
    property: (property_identifier) @callee))
`

// JavaScriptImportSitesQuery captura import-sites e type-reference sites em JavaScript.
// Cobre: import nomeado, require() e new expression.
const JavaScriptImportSitesQuery = `
(import_statement
  (import_clause
    (named_imports
      (import_specifier
        name: (identifier) @callee))))

(new_expression
  constructor: (identifier) @callee)

(new_expression
  constructor: (member_expression
    property: (property_identifier) @callee))
`

// JavaScriptVariablesQuery é a S-expression para variáveis locais em JavaScript.
const JavaScriptVariablesQuery = `
(variable_declarator
  name: (identifier) @var.name)

(variable_declarator
  name: (object_pattern
    (shorthand_property_identifier_pattern) @var.name))

(variable_declarator
  name: (array_pattern
    (identifier) @var.name))

(for_in_statement
  left: (identifier) @var.name)

(formal_parameters
  (identifier) @var.name)
`

// JavaScriptLanguageConfig implementa LanguageConfig para JavaScript.
type JavaScriptLanguageConfig struct{}

func (c *JavaScriptLanguageConfig) Name() string {
	return "javascript"
}

func (c *JavaScriptLanguageConfig) GetLanguage() *sitter.Language {
	return javascript.GetLanguage()
}

func (c *JavaScriptLanguageConfig) SymbolsQuery() string {
	return JavaScriptSymbolsQuery
}

func (c *JavaScriptLanguageConfig) CallersQuery() string {
	return JavaScriptCallersQuery
}

func (c *JavaScriptLanguageConfig) VariablesQuery() string {
	return JavaScriptVariablesQuery
}

func (c *JavaScriptLanguageConfig) TestPatterns() []indexer.TestPattern {
	return []indexer.TestPattern{
		{Type: indexer.TestPatternCallExpression, Value: "it"},
		{Type: indexer.TestPatternCallExpression, Value: "test"},
		{Type: indexer.TestPatternCallExpression, Value: "describe"},
	}
}

func (c *JavaScriptLanguageConfig) Extensions() []string {
	return []string{".js", ".jsx", ".mjs", ".cjs"}
}

func (c *JavaScriptLanguageConfig) ExtractImportSites(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.CallerInfo {
	return extractImportSitesWithQuery(JavaScriptImportSitesQuery, javascript.GetLanguage(), projectID, relPath, content, rootNode)
}

func (c *TypeScriptLanguageConfig) ExtractDependencies(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DependencyEdge {
	return extractJSTSDependencies(projectID, relPath, content, rootNode)
}

func (c *JavaScriptLanguageConfig) ExtractDependencies(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DependencyEdge {
	return extractJSTSDependencies(projectID, relPath, content, rootNode)
}

func extractJSTSDependencies(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DependencyEdge {
	var edges []*storage.DependencyEdge
	if rootNode == nil {
		return edges
	}

	var walk func(node *sitter.Node, currentClass string)
	walk = func(node *sitter.Node, currentClass string) {
		if node == nil {
			return
		}

		if node.Type() == "class_declaration" {
			className := ""
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "type_identifier" || child.Type() == "identifier" {
					className = child.Content(content)
					break
				}
			}

			if className != "" {
				currentClass = className

				// 1. Heritage (extends / implements)
				for i := 0; i < int(node.ChildCount()); i++ {
					child := node.Child(i)
					if child.Type() == "class_heritage" || child.Type() == "extends_clause" || child.Type() == "implements_clause" {
						extractJSTSHeritage(projectID, relPath, currentClass, child, content, &edges)
					}
				}
			}
		}

		// 2. Construtores e Injeção de Dependências
		if node.Type() == "method_definition" {
			methodName := ""
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "property_identifier" || child.Type() == "identifier" {
					methodName = child.Content(content)
					break
				}
			}

			if methodName == "constructor" && currentClass != "" {
				extractJSTSConstructorParams(projectID, relPath, currentClass, node, content, &edges)
			}
		}

		// 3. Propriedades anotadas com @Inject
		if node.Type() == "public_field_definition" || node.Type() == "property_definition" {
			if currentClass != "" {
				extractJSTSPropertyInject(projectID, relPath, currentClass, node, content, &edges)
			}
		}

		// 4. Instanciações diretas: new Target(...)
		if node.Type() == "new_expression" && currentClass != "" {
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "identifier" || child.Type() == "type_identifier" {
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

func extractJSTSHeritage(projectID, relPath, currentClass string, heritageNode *sitter.Node, content []byte, edges *[]*storage.DependencyEdge) {
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "type_identifier" || n.Type() == "identifier" {
			target := n.Content(content)
			if target != currentClass && !IsPrimitiveType(target) {
				*edges = append(*edges, &storage.DependencyEdge{
					ProjectID:        projectID,
					SourceSymbol:     currentClass,
					TargetSymbol:     target,
					SourceFile:       relPath,
					RelationshipType: "uses",
				})
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(heritageNode)
}

func extractJSTSConstructorParams(projectID, relPath, currentClass string, constructorNode *sitter.Node, content []byte, edges *[]*storage.DependencyEdge) {
	paramsNode := FindChildByType(constructorNode, "formal_parameters")
	if paramsNode == nil {
		return
	}

	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		param := paramsNode.Child(i)
		if param == nil {
			continue
		}

		// Procura decorator @Inject('Token')
		hasDecoratorInject := false
		for j := 0; j < int(param.ChildCount()); j++ {
			child := param.Child(j)
			if child.Type() == "decorator" && strings.Contains(child.Content(content), "Inject") {
				if token := extractFirstStringArg(child, content); token != "" {
					hasDecoratorInject = true
					*edges = append(*edges, &storage.DependencyEdge{
						ProjectID:        projectID,
						SourceSymbol:     currentClass,
						TargetSymbol:     token,
						SourceFile:       relPath,
						RelationshipType: "injects",
					})
				}
			}
		}

		// Procura tipo anotado (type_annotation -> type_identifier)
		var findType func(n *sitter.Node)
		findType = func(n *sitter.Node) {
			if n == nil || hasDecoratorInject {
				return
			}
			if n.Type() == "type_identifier" {
				typeName := n.Content(content)
				if !IsPrimitiveType(typeName) {
					*edges = append(*edges, &storage.DependencyEdge{
						ProjectID:        projectID,
						SourceSymbol:     currentClass,
						TargetSymbol:     typeName,
						SourceFile:       relPath,
						RelationshipType: "injects",
					})
				}
				return
			}
			for k := 0; k < int(n.ChildCount()); k++ {
				findType(n.Child(k))
			}
		}

		for j := 0; j < int(param.ChildCount()); j++ {
			child := param.Child(j)
			if child.Type() == "type_annotation" {
				findType(child)
			}
		}
	}
}

func extractJSTSPropertyInject(projectID, relPath, currentClass string, propNode *sitter.Node, content []byte, edges *[]*storage.DependencyEdge) {
	propText := propNode.Content(content)
	if !strings.Contains(propText, "@Inject") && !strings.Contains(propText, "@Autowired") {
		return
	}

	typeAnn := FindChildByType(propNode, "type_annotation")
	if typeAnn != nil {
		for i := 0; i < int(typeAnn.ChildCount()); i++ {
			child := typeAnn.Child(i)
			if child.Type() == "type_identifier" {
				target := child.Content(content)
				if !IsPrimitiveType(target) {
					*edges = append(*edges, &storage.DependencyEdge{
						ProjectID:        projectID,
						SourceSymbol:     currentClass,
						TargetSymbol:     target,
						SourceFile:       relPath,
						RelationshipType: "injects",
					})
				}
			}
		}
	}
}

func extractFirstStringArg(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	var res string
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil || res != "" {
			return
		}
		if n.Type() == "string" || n.Type() == "string_fragment" || n.Type() == "string_literal" {
			res = CleanStringLiteral(n.Content(content))
			return
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
	return res
}

func extractJSTSDataModels(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DataModel {
	var modelsList []*storage.DataModel

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		switch n.Type() {
		case "class_declaration":
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				className := nameNode.Content(content)
				lineNum := int(nameNode.StartPoint().Row) + 1
				fields := extractTSClassFields(n, content)
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

		case "interface_declaration":
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				ifaceName := nameNode.Content(content)
				lineNum := int(nameNode.StartPoint().Row) + 1
				fields := extractTSInterfaceFields(n, content)
				if len(fields) > 0 {
					modelsList = append(modelsList, &storage.DataModel{
						ProjectID: projectID,
						Name:      ifaceName,
						File:      relPath,
						Kind:      "interface",
						Line:      lineNum,
						Fields:    fields,
					})
				}
			}

		case "type_alias_declaration":
			nameNode := n.ChildByFieldName("name")
			valueNode := n.ChildByFieldName("value")
			if nameNode != nil && valueNode != nil && valueNode.Type() == "object_type" {
				typeName := nameNode.Content(content)
				lineNum := int(nameNode.StartPoint().Row) + 1
				fields := extractTSObjectTypeFields(valueNode, content)
				if len(fields) > 0 {
					modelsList = append(modelsList, &storage.DataModel{
						ProjectID: projectID,
						Name:      typeName,
						File:      relPath,
						Kind:      "type",
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

func extractTSClassFields(classNode *sitter.Node, content []byte) []*storage.ModelField {
	var fields []*storage.ModelField
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return fields
	}

	for i := 0; i < int(body.ChildCount()); i++ {
		member := body.Child(i)
		if member == nil {
			continue
		}

		mType := member.Type()
		if mType == "field_definition" || mType == "public_field_definition" || mType == "property_definition" {
			nameNode := member.ChildByFieldName("name")
			if nameNode != nil {
				fieldName := nameNode.Content(content)
				// Ignora campos estáticos ou métodos
				typeAnn := FindChildByType(member, "type_annotation")
				typeStr := "any"
				if typeAnn != nil {
					typeStr = cleanTSType(typeAnn.Content(content))
				}

				memberText := member.Content(content)
				isOptional := strings.Contains(memberText, "?") || strings.Contains(memberText, "@IsOptional")

				fields = append(fields, &storage.ModelField{
					Name:     fieldName,
					Type:     typeStr,
					Required: !isOptional,
				})
			}
		} else if mType == "method_definition" {
			nameNode := member.ChildByFieldName("name")
			if nameNode != nil && nameNode.Content(content) == "constructor" {
				// Parâmetros de construtor tipados
				params := FindChildByType(member, "formal_parameters")
				if params != nil {
					for j := 0; j < int(params.ChildCount()); j++ {
						p := params.Child(j)
						if p == nil {
							continue
						}
						pText := p.Content(content)
						if strings.Contains(pText, "public") || strings.Contains(pText, "private") || strings.Contains(pText, "protected") || strings.Contains(pText, "readonly") {
							pNameNode := FindChildByType(p, "identifier")
							if pNameNode == nil {
								pNameNode = FindChildByType(p, "property_identifier")
							}
							if pNameNode != nil {
								pName := pNameNode.Content(content)
								typeAnn := FindChildByType(p, "type_annotation")
								typeStr := "any"
								if typeAnn != nil {
									typeStr = cleanTSType(typeAnn.Content(content))
								}
								isOptional := strings.Contains(pText, "?")
								fields = append(fields, &storage.ModelField{
									Name:     pName,
									Type:     typeStr,
									Required: !isOptional,
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

func extractTSInterfaceFields(ifaceNode *sitter.Node, content []byte) []*storage.ModelField {
	var fields []*storage.ModelField
	body := ifaceNode.ChildByFieldName("body")
	if body == nil {
		body = FindChildByType(ifaceNode, "object_type")
	}
	if body == nil {
		return fields
	}

	for i := 0; i < int(body.ChildCount()); i++ {
		member := body.Child(i)
		if member == nil {
			continue
		}
		if member.Type() == "property_signature" {
			nameNode := member.ChildByFieldName("name")
			if nameNode != nil {
				fieldName := nameNode.Content(content)
				typeAnn := FindChildByType(member, "type_annotation")
				typeStr := "any"
				if typeAnn != nil {
					typeStr = cleanTSType(typeAnn.Content(content))
				}
				isOptional := strings.Contains(member.Content(content), "?")
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

func extractTSObjectTypeFields(objNode *sitter.Node, content []byte) []*storage.ModelField {
	var fields []*storage.ModelField
	for i := 0; i < int(objNode.ChildCount()); i++ {
		member := objNode.Child(i)
		if member != nil && member.Type() == "property_signature" {
			nameNode := member.ChildByFieldName("name")
			if nameNode != nil {
				fieldName := nameNode.Content(content)
				typeAnn := FindChildByType(member, "type_annotation")
				typeStr := "any"
				if typeAnn != nil {
					typeStr = cleanTSType(typeAnn.Content(content))
				}
				isOptional := strings.Contains(member.Content(content), "?")
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

func cleanTSType(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, ":")
	raw = strings.TrimSpace(raw)
	return raw
}

func extractJSTSDigest(relPath string, content []byte, rootNode *sitter.Node) string {
	if rootNode == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("// File: " + relPath + "\n")

	for i := 0; i < int(rootNode.ChildCount()); i++ {
		child := rootNode.Child(i)
		if child == nil {
			continue
		}

		target := child
		isExport := false
		if child.Type() == "export_statement" {
			isExport = true
			decl := child.ChildByFieldName("declaration")
			if decl != nil {
				target = decl
			} else {
				sb.WriteString(child.Content(content) + "\n\n")
				continue
			}
		}

		prefix := ""
		if isExport {
			prefix = "export "
		}

		switch target.Type() {
		case "import_statement":
			sb.WriteString(target.Content(content) + "\n\n")
		case "interface_declaration", "type_alias_declaration":
			sb.WriteString(prefix + target.Content(content) + "\n\n")
		case "class_declaration":
			nameNode := target.ChildByFieldName("name")
			heritage := FindChildByType(target, "class_heritage")
			className := ""
			if nameNode != nil {
				className = nameNode.Content(content)
			}
			classSig := prefix + "class " + className
			if heritage != nil {
				classSig += " " + heritage.Content(content)
			}
			classSig += " {\n"
			sb.WriteString(classSig)

			body := target.ChildByFieldName("body")
			if body != nil {
				for j := 0; j < int(body.ChildCount()); j++ {
					m := body.Child(j)
					if m == nil {
						continue
					}
					switch m.Type() {
					case "method_definition":
						mName := m.ChildByFieldName("name")
						mParams := m.ChildByFieldName("parameters")
						mRet := m.ChildByFieldName("return_type")
						if mName != nil {
							mSig := "  " + mName.Content(content)
							if mParams != nil {
								mSig += mParams.Content(content)
							} else {
								mSig += "()"
							}
							if mRet != nil {
								retContent := strings.TrimSpace(mRet.Content(content))
								if strings.HasPrefix(retContent, ":") {
									mSig += ": " + strings.TrimSpace(strings.TrimPrefix(retContent, ":"))
								} else {
									mSig += ": " + retContent
								}
							}
							mSig += " { ... }\n"
							sb.WriteString(mSig)
						}
					case "public_field_definition", "field_definition":
						sb.WriteString("  " + m.Content(content) + "\n")
					}
				}
			}
			sb.WriteString("}\n\n")
		case "function_declaration":
			nameNode := target.ChildByFieldName("name")
			paramsNode := target.ChildByFieldName("parameters")
			retNode := target.ChildByFieldName("return_type")
			funcSig := prefix + "function "
			if nameNode != nil {
				funcSig += nameNode.Content(content)
			}
			if paramsNode != nil {
				funcSig += paramsNode.Content(content)
			} else {
				funcSig += "()"
			}
			if retNode != nil {
				retContent := strings.TrimSpace(retNode.Content(content))
				if strings.HasPrefix(retContent, ":") {
					funcSig += ": " + strings.TrimSpace(strings.TrimPrefix(retContent, ":"))
				} else {
					funcSig += ": " + retContent
				}
			}
			funcSig += " { ... }\n\n"
			sb.WriteString(funcSig)
		}
	}

	return strings.TrimSpace(sb.String())
}

// ExtractDigest gera o digest AST do arquivo TypeScript.
func (c *TypeScriptLanguageConfig) ExtractDigest(relPath string, content []byte, rootNode *sitter.Node) string {
	return extractJSTSDigest(relPath, content, rootNode)
}

// ExtractDigest gera o digest AST do arquivo JavaScript.
func (c *JavaScriptLanguageConfig) ExtractDigest(relPath string, content []byte, rootNode *sitter.Node) string {
	return extractJSTSDigest(relPath, content, rootNode)
}

func init() {
	indexer.RegisterLanguage(&TypeScriptLanguageConfig{})
	indexer.RegisterLanguage(&JavaScriptLanguageConfig{})
}
