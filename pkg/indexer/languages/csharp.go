package languages

import (
	"astrix/pkg/indexer"

	"github.com/smacker/go-tree-sitter/csharp"
)

const CSharpSymbolsQuery = `
(namespace_declaration
  name: [
    (identifier) @namespace.name
    (qualified_name) @namespace.name
  ]) @namespace.def

(class_declaration
  name: (identifier) @class.name) @class.def

(struct_declaration
  name: (identifier) @struct.name) @struct.def

(interface_declaration
  name: (identifier) @interface.name) @interface.def

(enum_declaration
  name: (identifier) @enum.name) @enum.def

(enum_member_declaration
  name: (identifier) @const.name) @const.def

(record_declaration
  name: (identifier) @record.name) @record.def

(method_declaration
  name: (identifier) @method.name) @method.def

(constructor_declaration
  name: (identifier) @constructor.name) @constructor.def

(property_declaration
  name: (identifier) @field.name) @field.def

(field_declaration
  (variable_declaration
    (variable_declarator
      (identifier) @field.name))) @field.def
`

const CSharpCallersQuery = `
(invocation_expression
  function: (identifier) @callee)

(invocation_expression
  function: (member_access_expression
    name: (identifier) @callee))

(object_creation_expression
  type: (identifier) @callee)
`

const CSharpVariablesQuery = `
(local_declaration_statement
  (variable_declaration
    (variable_declarator
      (identifier) @var.name)))

(parameter
  name: (identifier) @var.name)
`

const CSharpImportSitesQuery = `
(using_directive
  (identifier) @callee)

(using_directive
  (qualified_name
    (identifier) @callee))

(base_list
  (identifier) @callee)
`

func init() {
	indexer.RegisterLanguage(&BaseLanguageConfig{
		LangName:    "csharp",
		Language:    csharp.GetLanguage(),
		Exts:        []string{".cs"},
		Symbols:     CSharpSymbolsQuery,
		Callers:     CSharpCallersQuery,
		Variables:   CSharpVariablesQuery,
		ImportSites: CSharpImportSitesQuery,
		Tests: []indexer.TestPattern{
			{Type: indexer.TestPatternAttribute, Value: "Fact"},
			{Type: indexer.TestPatternAttribute, Value: "Theory"},
			{Type: indexer.TestPatternAttribute, Value: "Test"},
		},
	})
}
