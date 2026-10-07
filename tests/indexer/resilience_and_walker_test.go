package indexer_test

import (
	"astrix/pkg/indexer"
	_ "astrix/pkg/indexer/languages"
	"astrix/pkg/storage"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexer_SyntaxErrorResilience(t *testing.T) {
	tempDB := filepath.Join(t.TempDir(), "syntax_err_test.db")
	db, err := storage.NewDatabase(tempDB)
	require.NoError(t, err)
	defer db.Close()

	engine := indexer.NewEngine(
		storage.NewProjectRepo(db),
		storage.NewSymbolRepo(db),
		storage.NewDependencyGraphRepo(db),
		storage.NewDataModelRepo(db),
	)

	// Código Go com erro de sintaxe deliberado dentro de uma função entre duas funções válidas
	malformedGoCode := `package sample

func ValidFirst(x int) int {
	return x * 2
}

func BrokenFunc() {
	var = 123 // syntax error aqui
}

func ValidSecond() string {
	return "ok"
}
`
	goConfig, ok := indexer.GetConfigByFilePath("sample.go")
	require.True(t, ok)

	res, err := engine.ExtractASTDataAndDigest("proj-test", "sample.go", []byte(malformedGoCode), goConfig)
	require.NoError(t, err)
	require.NotNil(t, res)

	// A recuperação parcial de AST deve extrair com sucesso as funções válidas
	var symbolNames []string
	for _, s := range res.Symbols {
		symbolNames = append(symbolNames, s.Name)
	}

	assert.Contains(t, symbolNames, "ValidFirst")
	assert.Contains(t, symbolNames, "ValidSecond")
}

func TestWalker_BinaryDetection(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Arquivo de texto puro
	txtFile := filepath.Join(tempDir, "text.txt")
	err := os.WriteFile(txtFile, []byte("Hello world, this is a plain text file\n"), 0o644)
	require.NoError(t, err)
	assert.False(t, indexer.IsBinaryFile(txtFile))

	// 2. Arquivo binário com byte NUL
	binFile := filepath.Join(tempDir, "sample.bin")
	err = os.WriteFile(binFile, []byte("ELF\x00\x01\x02\x03\x04something"), 0o644)
	require.NoError(t, err)
	assert.True(t, indexer.IsBinaryFile(binFile))
}

func TestWalker_NestedGitIgnoreAndExclude(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Cria .gitignore na raiz ignorando *.log
	err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte("*.log\n"), 0o644)
	require.NoError(t, err)

	// 2. Cria .git/info/exclude ignorando local.secret
	err = os.MkdirAll(filepath.Join(tempDir, ".git", "info"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, ".git", "info", "exclude"), []byte("local.secret\n"), 0o644)
	require.NoError(t, err)

	// 3. Cria subdiretório com .gitignore aninhado ignorando temp.go
	subDir := filepath.Join(tempDir, "pkg", "sub")
	err = os.MkdirAll(subDir, 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, ".gitignore"), []byte("temp.go\n"), 0o644)
	require.NoError(t, err)

	// Arquivos de teste
	err = os.WriteFile(filepath.Join(tempDir, "app.log"), []byte("log data"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "local.secret"), []byte("secret"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "temp.go"), []byte("package sub\nfunc Temp() {}\n"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "real.go"), []byte("package sub\nfunc Real() {}\n"), 0o644)
	require.NoError(t, err)

	// Compila regras via LoadGitIgnore
	gi := indexer.LoadGitIgnore(tempDir)
	require.NotNil(t, gi)

	assert.True(t, gi.MatchesPath("app.log"))
	assert.True(t, gi.MatchesPath("local.secret"))
	assert.True(t, gi.MatchesPath("pkg/sub/temp.go"))
	assert.False(t, gi.MatchesPath("pkg/sub/real.go"))

	// Varredura via ScanRepository deve retornar apenas real.go
	files, err := indexer.ScanRepository(tempDir)
	require.NoError(t, err)

	var relPaths []string
	for _, f := range files {
		relPaths = append(relPaths, f.RelPath)
	}

	assert.Contains(t, relPaths, filepath.Join("pkg", "sub", "real.go"))
	assert.NotContains(t, relPaths, filepath.Join("pkg", "sub", "temp.go"))
	assert.NotContains(t, relPaths, "app.log")
	assert.NotContains(t, relPaths, "local.secret")
}

func TestIndexer_LanguageDetection_SingleSourceOfTruth(t *testing.T) {
	// 1. Linguagens com LanguageConfig registrado retornam a configuração correta
	cfgGo, ok := indexer.GetConfigByFilePath("main.go")
	require.True(t, ok)
	assert.Equal(t, "go", cfgGo.Name())

	cfgTS, ok := indexer.GetConfigByFilePath("app.ts")
	require.True(t, ok)
	assert.Equal(t, "typescript", cfgTS.Name())

	cfgPy, ok := indexer.GetConfigByFilePath("script.py")
	require.True(t, ok)
	assert.Equal(t, "python", cfgPy.Name())

	cfgJava, ok := indexer.GetConfigByFilePath("Service.java")
	require.True(t, ok)
	assert.Equal(t, "java", cfgJava.Name())

	cfgPhp, ok := indexer.GetConfigByFilePath("index.php")
	require.True(t, ok)
	assert.Equal(t, "php", cfgPhp.Name())

	// 2. Extensão desconhecida para AST retorna false no registry
	_, ok = indexer.GetConfigByFilePath("data.unknown")
	assert.False(t, ok)
}
