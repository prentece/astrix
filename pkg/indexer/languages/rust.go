package languages

import (
	"astrix/pkg/indexer"

	"github.com/smacker/go-tree-sitter/rust"
)

const RustSymbolsQuery = `
(source_file
  (function_item
    name: (identifier) @function.name) @function.def)

(mod_item
  name: (identifier) @namespace.name) @namespace.def

(mod_item
  body: (declaration_list
    (function_item
      name: (identifier) @function.name) @function.def))

(impl_item
  type: [
    (type_identifier) @method.parent
    (generic_type
      type: (type_identifier) @method.parent)
  ]
  body: (declaration_list
    (function_item
      name: (identifier) @method.name) @method.def))

(trait_item
  name: (type_identifier) @trait.name) @trait.def

(trait_item
  body: (declaration_list
    (function_item
      name: (identifier) @method.name) @method.def))

(function_signature_item
  name: (identifier) @method.name) @method.def

(struct_item
  name: (type_identifier) @struct.name) @struct.def

(field_declaration
  name: (field_identifier) @field.name) @field.def

(enum_item
  name: (type_identifier) @enum.name) @enum.def

(enum_variant
  name: (identifier) @const.name) @const.def

(type_item
  name: (type_identifier) @type.name) @type.def

(const_item
  name: (identifier) @const.name) @const.def

(static_item
  name: (identifier) @const.name) @const.def
`

const RustCallersQuery = `
(call_expression
  function: (identifier) @callee)

(call_expression
  function: (field_expression
    field: (field_identifier) @callee))

(call_expression
  function: (scoped_identifier
    name: (identifier) @callee))
`

const RustVariablesQuery = `
(let_declaration
  pattern: (identifier) @var.name)

(parameter
  pattern: (identifier) @var.name)
`

const RustImportSitesQuery = `
(use_declaration
  argument: (scoped_identifier
    name: (identifier) @callee))

(struct_expression
  name: (type_identifier) @callee)
`

func init() {
	indexer.RegisterLanguage(&BaseLanguageConfig{
		LangName:    "rust",
		Language:    rust.GetLanguage(),
		Exts:        []string{".rs"},
		Symbols:     RustSymbolsQuery,
		Callers:     RustCallersQuery,
		Variables:   RustVariablesQuery,
		ImportSites: RustImportSitesQuery,
		Tests: []indexer.TestPattern{
			{Type: indexer.TestPatternAttribute, Value: "test"},
			{Type: indexer.TestPatternPrefix, Value: "test_"},
		},
	})
}
