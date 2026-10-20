package languages_test

import (
	"astrix/pkg/indexer"
	"astrix/pkg/indexer/languages"
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/stretchr/testify/require"
)

// Toda query de toda linguagem registrada SHALL compilar contra a sua gramática
// e toda captura de SymbolsQuery SHALL ser reconhecida pelo engine.
func TestAllLanguageQueriesCompile(t *testing.T) {
	for _, al := range auditLangs {
		t.Run(al.name, func(t *testing.T) {
			cfg, ok := indexer.GetConfigByName(al.name)
			require.True(t, ok)
			require.NotEmpty(t, cfg.Extensions(), "linguagem %s sem extensões registradas", al.name)
			lang := cfg.GetLanguage()
			require.NotNil(t, lang)

			queries := map[string]string{
				"symbols":   cfg.SymbolsQuery(),
				"callers":   cfg.CallersQuery(),
				"variables": cfg.VariablesQuery(),
			}
			for label, src := range queries {
				q, err := sitter.NewQuery([]byte(src), lang)
				require.NoError(t, err, "query %s de %s não compila", label, al.name)
				if label == "symbols" {
					for i := uint32(0); i < q.CaptureCount(); i++ {
						capName := q.CaptureNameForId(i)
						base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(capName, ".name"), ".def"), ".parent")
						_, known := indexer.SymbolCaptureKind(base)
						require.True(t, known, "captura @%s de %s seria descartada pelo engine", capName, al.name)
					}
				}
			}
		})
	}
}

func TestBaseLanguageConfigContract(t *testing.T) {
	goCfg, ok := indexer.GetConfigByName("go")
	require.True(t, ok)

	base := &languages.BaseLanguageConfig{
		LangName:    "custom-go",
		Language:    goCfg.GetLanguage(),
		Exts:        []string{".cgo"},
		Symbols:     goCfg.SymbolsQuery(),
		Callers:     goCfg.CallersQuery(),
		Variables:   goCfg.VariablesQuery(),
		ImportSites: `(import_spec path: (interpreted_string_literal) @callee)`,
		Tests:       []indexer.TestPattern{{Type: indexer.TestPatternPrefix, Value: "Test"}},
	}

	var cfg indexer.LanguageConfig = base
	require.Equal(t, "custom-go", cfg.Name())
	require.Equal(t, []string{".cgo"}, cfg.Extensions())
	require.NotNil(t, cfg.GetLanguage())
	require.NotEmpty(t, cfg.SymbolsQuery())
	require.NotEmpty(t, cfg.CallersQuery())
	require.NotEmpty(t, cfg.VariablesQuery())
	require.Len(t, cfg.TestPatterns(), 1)

	var impExtractor indexer.ImportSiteExtractor = base
	require.NotNil(t, impExtractor)
}

