package languages

import (
	"astrix/pkg/indexer"

	"github.com/smacker/go-tree-sitter/kotlin"
)

const KotlinSymbolsQuery = `
(class_declaration
  (type_identifier) @class.name) @class.def

(class_declaration
  "interface"
  (type_identifier) @interface.name) @interface.def

(class_declaration
  (type_identifier) @enum.name
  (enum_class_body)) @enum.def

(object_declaration
  (type_identifier) @class.name) @class.def

(enum_entry
  (simple_identifier) @const.name) @const.def

(function_declaration
  (simple_identifier) @function.name) @function.def

(class_body
  (function_declaration
    (simple_identifier) @method.name) @method.def)

(class_parameter
  (simple_identifier) @field.name) @field.def

(class_body
  (property_declaration
    (variable_declaration
      (simple_identifier) @field.name)) @field.def)

(source_file
  (property_declaration
    (variable_declaration
      (simple_identifier) @const.name)) @const.def)
`

const KotlinCallersQuery = `
(call_expression
  (simple_identifier) @callee)

(call_expression
  (navigation_expression
    (navigation_suffix
      (simple_identifier) @callee)))
`

const KotlinVariablesQuery = `
(property_declaration
  (variable_declaration
    (simple_identifier) @var.name))

(parameter
  (simple_identifier) @var.name)
`

const KotlinImportSitesQuery = `
(import_header
  (identifier
    (simple_identifier) @callee))
`

func init() {
	indexer.RegisterLanguage(&BaseLanguageConfig{
		LangName:    "kotlin",
		Language:    kotlin.GetLanguage(),
		Exts:        []string{".kt", ".kts"},
		Symbols:     KotlinSymbolsQuery,
		Callers:     KotlinCallersQuery,
		Variables:   KotlinVariablesQuery,
		ImportSites: KotlinImportSitesQuery,
		Tests: []indexer.TestPattern{
			{Type: indexer.TestPatternAttribute, Value: "Test"},
		},
	})
}
