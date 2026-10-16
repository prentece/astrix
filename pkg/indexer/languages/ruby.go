package languages

import (
	"astrix/pkg/indexer"

	"github.com/smacker/go-tree-sitter/ruby"
)

const RubySymbolsQuery = `
(module
  name: [
    (constant) @namespace.name
    (scope_resolution) @namespace.name
  ]) @namespace.def

(class
  name: [
    (constant) @class.name
    (scope_resolution) @class.name
  ]) @class.def

(program
  (method
    name: (identifier) @function.name) @function.def)

(body_statement
  (method
    name: (identifier) @method.name) @method.def)

(singleton_method
  name: (identifier) @method.name) @method.def

(assignment
  left: (constant) @const.name) @const.def
`

const RubyCallersQuery = `
(call
  method: (identifier) @callee)
`

const RubyVariablesQuery = `
(assignment
  left: (identifier) @var.name)

(method_parameters
  (identifier) @var.name)
`

const RubyImportSitesQuery = `
(call
  method: (identifier) @callee)
`

func init() {
	indexer.RegisterLanguage(&BaseLanguageConfig{
		LangName:    "ruby",
		Language:    ruby.GetLanguage(),
		Exts:        []string{".rb"},
		Symbols:     RubySymbolsQuery,
		Callers:     RubyCallersQuery,
		Variables:   RubyVariablesQuery,
		ImportSites: RubyImportSitesQuery,
		Tests: []indexer.TestPattern{
			{Type: indexer.TestPatternPrefix, Value: "test_"},
		},
	})
}
