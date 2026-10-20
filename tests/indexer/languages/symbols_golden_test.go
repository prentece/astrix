package languages_test

import (
	"astrix/pkg/indexer"
	_ "astrix/pkg/indexer/languages"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Golden de símbolos (Fase 2): cada fixture em testdata/audit/<lang>/ tem a lista EXATA de
// símbolos esperados no formato "kind nome parent". O parent é o container que envolve o símbolo
// (classe, struct, interface, enum, trait, namespace) ou, em Go, o tipo do receiver.
var symbolGolden = map[string][]string{
	"go": {
		"constant Version -", "constant ModeA -", "constant ModeB -",
		"variable DefaultName -", "variable handler -",
		"type Alias -", "type Weekday -",
		"interface Shape -", "method Area Shape", "method Perimeter Shape",
		"struct Point -", "field X Point", "field Y Point", "field Label Point",
		"struct Named -", "field Tags Named",
		"function New -", "method String Point",
		"function Generic -",
		"struct Stack -", "field items Stack", "method Push Stack",
	},
	"java": {
		"interface Marker -", "method value Marker",
		"enum Color -", "constant RED Color", "constant GREEN Color", "method label Color",
		"class Pair -", "method Pair Pair", "method sum Pair",
		"interface Repo -", "method find Repo", "method touch Repo",
		"class Base -", "field LIMIT Base", "field name Base", "field count Base",
		"method Base Base", "method run Base",
		"class Inner Base", "method go Inner",
	},
	"php": {
		"namespace App\\Sample -", "import Repo -",
		"constant TOP_LEVEL -", "function helper -",
		"trait Greets -", "method hello Greets",
		"enum Status -", "constant Active Status", "constant Off Status",
		"interface Named -", "constant PREFIX Named", "method name Named",
		"class Model -", "import Greets Model", "constant TABLE Model",
		"field title Model", "field count Model", "field id Model",
		"method __construct Model", "method save Model", "method name Model",
	},
	"python": {
		"import Optional -",
		"constant MAX_RETRIES -", "constant _cache -", "constant DEBUG -",
		"function top_level -", "function fetch -",
		"class Outer -", "constant LIMIT Outer", "constant name Outer",
		"class Nested Outer", "method inner Nested",
		"method __init__ Outer", "method doubled Outer", "method make Outer", "method build Outer",
		"class Decorated -", "constant square -",
	},
	"typescript": {
		"enum Direction -", "constant Up Direction", "constant Down Direction",
		"enum Flags -", "constant A Flags",
		"function ambient -",
		"namespace Util -", "function helper Util",
		"namespace ext -",
		"class Base -", "field count Base", "field id Base", "field cache Base", "field name Base",
		"method constructor Base", "method run Base", "method label Base", "field handle Base",
		"interface Repo -", "method find Repo", "field size Repo", "field name Repo",
		"type Id -",
		"constant arrow -", "function fnExpr -", "constant CONSTANT -",
		"class Main -",
	},
	"javascript": {
		"function top -", "constant arrow -", "constant CONSTANT -", "function fnExpr -",
		"class Base -", "field count Base", "field name Base", "field #secret Base",
		"method constructor Base", "method label Base", "method make Base", "field handle Base",
		"function helper -", "function other -",
	},
	"rust": {
		"namespace net -", "function connect net",
		"constant MAX_CONN -", "constant VERSION -",
		"type Result -",
		"enum Status -", "constant Active Status", "constant Inactive Status",
		"trait Runner -", "method run Runner",
		"struct Server -", "field port Server", "field name Server",
		"method new Server", "method start Server",
		"function bootstrap -",
	},
	"csharp": {
		"namespace App.Core -",
		"enum Role App.Core", "constant Admin Role", "constant Guest Role",
		"interface IRepository App.Core", "method Find IRepository",
		"struct Coordinate App.Core", "field X Coordinate", "field Y Coordinate",
		"class UserDto App.Core",
		"class UserService App.Core", "field _prefix UserService", "field Count UserService",
		"method UserService UserService", "method Find UserService", "method Format UserService",
	},
	"kotlin": {
		"enum Mode -", "constant FAST Mode", "constant SAFE Mode",
		"interface Worker -", "method execute Worker",
		"class Config -", "field version Config", "method load Config",
		"class Service -", "field name Service", "field retries Service", "method execute Service",
		"function helper -",
	},
	"ruby": {
		"namespace App -", "constant DEFAULT_TIMEOUT App",
		"namespace Helpers App", "method format Helpers",
		"class Client App", "constant MAX_RETRIES Client",
		"method initialize Client", "method fetch Client", "method create Client",
		"function top_helper -",
	},
}

func extractFixtureSymbols(t *testing.T, lang string) []string {
	t.Helper()
	cfg, ok := indexer.GetConfigByName(lang)
	require.True(t, ok)

	matches, err := filepath.Glob(filepath.Join("testdata", "audit", lang, "*"))
	require.NoError(t, err)
	require.Len(t, matches, 1, "esperada exatamente 1 fixture para %s", lang)

	content, err := os.ReadFile(matches[0])
	require.NoError(t, err)

	engine := indexer.NewEngine(nil, nil, nil, nil)
	res, err := engine.ExtractASTDataAndDigest("p", filepath.Base(matches[0]), content, cfg)
	require.NoError(t, err)

	out := make([]string, 0, len(res.Symbols))
	for _, s := range res.Symbols {
		parent := s.Parent
		if parent == "" {
			parent = "-"
		}
		out = append(out, fmt.Sprintf("%s %s %s", s.Kind, s.Name, parent))
	}
	sort.Strings(out)
	return out
}

func TestSymbolsGolden(t *testing.T) {
	for _, al := range auditLangs {
		t.Run(al.name, func(t *testing.T) {
			want := append([]string(nil), symbolGolden[al.name]...)
			sort.Strings(want)
			assert.Equal(t, want, extractFixtureSymbols(t, al.name))
		})
	}
}

func TestNewLanguagesCallersAndImportSites(t *testing.T) {
	expectedCallers := map[string][]string{
		"rust":   {"connect", "new", "start"},
		"csharp": {"Format"},
		"kotlin": {"trim", "helper", "isNotEmpty"},
		"ruby":   {"to_s", "format", "new"},
	}

	engine := indexer.NewEngine(nil, nil, nil, nil)
	for lang, wantCallees := range expectedCallers {
		t.Run(lang, func(t *testing.T) {
			cfg, ok := indexer.GetConfigByName(lang)
			require.True(t, ok)

			matches, err := filepath.Glob(filepath.Join("testdata", "audit", lang, "*"))
			require.NoError(t, err)
			require.Len(t, matches, 1)

			content, err := os.ReadFile(matches[0])
			require.NoError(t, err)

			res, err := engine.ExtractASTDataAndDigest("p", filepath.Base(matches[0]), content, cfg)
			require.NoError(t, err)

			seen := map[string]bool{}
			for _, c := range res.Callers {
				seen[c.SymbolName] = true
			}
			for _, callee := range wantCallees {
				assert.True(t, seen[callee], "esperava chamada/referência a %q em %s, obteve %v", callee, lang, seen)
			}
		})
	}
}

