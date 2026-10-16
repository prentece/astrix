package indexer

import (
	"astrix/pkg/storage"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strconv"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// ExtractorVersion identifica a versão do algoritmo de extração de símbolos do engine.
// Incrementar este valor invalida automaticamente o cache incremental de todos os arquivos.
const ExtractorVersion = "v2"

// QueryVersioner é uma interface opcional para configurações de linguagem que desejem
// invalidar explicitamente o cache incremental além do hash das queries S-expression.
type QueryVersioner interface {
	QueryVersion() int
}

// TestPatternType define como reconhecer testes em uma linguagem.
type TestPatternType string

const (
	TestPatternPrefix         TestPatternType = "prefix"
	TestPatternCallExpression TestPatternType = "call_expression"
	TestPatternAttribute      TestPatternType = "attribute"
)

// TestPattern define a regra para identificar funções de teste.
type TestPattern struct {
	Type  TestPatternType
	Value string
}

// DependencyExtractor define a capacidade de uma linguagem de extrair dependências e injeções de dependência via AST.
type DependencyExtractor interface {
	ExtractDependencies(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DependencyEdge
}

// DataModelExtractor define a capacidade de uma linguagem de extrair Schemas, DTOs e Entidades via AST.
type DataModelExtractor interface {
	ExtractDataModels(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.DataModel
}

// ImportSiteExtractor define a capacidade de uma linguagem de capturar import-sites e
// type-reference sites de símbolos de classe/tipo via AST.
// Captura referências que não são call-sites: import statements, declarações de variável
// com tipo explícito, anotações de tipo, herança e implementação de interfaces.
// Implementado opcionalmente; linguagens sem ele simplesmente não emitem import-sites.
type ImportSiteExtractor interface {
	ExtractImportSites(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.CallerInfo
}

// ASTDigestExtractor define a capacidade de gerar um esqueleto estruturado ultracompacto (< 500 tokens)
// contendo apenas assinaturas, structs/classes e imports/dependências para envio à LLM.
type ASTDigestExtractor interface {
	ExtractDigest(relPath string, content []byte, rootNode *sitter.Node) string
}

// LanguageConfig define o contrato e os metadados necessários para suportar uma linguagem no Tree-sitter.
type LanguageConfig interface {
	// Name retorna o identificador da linguagem (ex: "go", "python", "typescript", "javascript").
	Name() string
	// GetLanguage retorna o ponteiro C do Tree-sitter Language.
	GetLanguage() *sitter.Language
	// SymbolsQuery retorna a S-expression para extração de símbolos e definições.
	SymbolsQuery() string
	// CallersQuery retorna a S-expression para extração de chamadas de funções/métodos.
	CallersQuery() string
	// VariablesQuery retorna a S-expression para extração de variáveis locais.
	VariablesQuery() string
	// TestPatterns retorna os padrões para identificação de testes.
	TestPatterns() []TestPattern
	// Extensions retorna as extensões de arquivo associadas (ex: []string{".go"}).
	Extensions() []string
}

// Registry mantém as configurações de linguagem registradas.
var (
	registry = make(map[string]LanguageConfig)
	extMap   = make(map[string]LanguageConfig)
)

// RegisterLanguage registra uma nova configuração de linguagem no sistema.
func RegisterLanguage(config LanguageConfig) {
	registry[config.Name()] = config
	for _, ext := range config.Extensions() {
		extMap[strings.ToLower(ext)] = config
	}
}

// GetConfigByName busca a configuração pelo nome da linguagem.
func GetConfigByName(name string) (LanguageConfig, bool) {
	cfg, ok := registry[strings.ToLower(name)]
	return cfg, ok
}

// GetConfigByFilePath detecta a linguagem a partir da extensão do arquivo.
func GetConfigByFilePath(filePath string) (LanguageConfig, bool) {
	ext := strings.ToLower(filepath.Ext(filePath))
	cfg, ok := extMap[ext]
	return cfg, ok
}

// SupportedExtensions retorna a lista de extensões que possuem suporte ao Tree-sitter.
func SupportedExtensions() []string {
	exts := make([]string, 0, len(extMap))
	for ext := range extMap {
		exts = append(exts, ext)
	}
	return exts
}

// QuerySignature retorna uma assinatura curta (12 hex chars) que representa a versão do extrator
// e o texto das queries Tree-sitter da linguagem. Qualquer mudança nas queries altera a assinatura.
func QuerySignature(cfg LanguageConfig) string {
	if cfg == nil {
		return ""
	}
	h := sha256.New()
	_, _ = h.Write([]byte(ExtractorVersion))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(cfg.Name()))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(cfg.SymbolsQuery()))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(cfg.CallersQuery()))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(cfg.VariablesQuery()))
	if qv, ok := cfg.(QueryVersioner); ok {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(strconv.Itoa(qv.QueryVersion())))
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:6])
}

// CalculateContentHash combina o SHA-256 do conteúdo do arquivo com a QuerySignature da linguagem.
func CalculateContentHash(content []byte, cfg LanguageConfig) string {
	base := calculateHash(content)
	sig := QuerySignature(cfg)
	if sig == "" {
		return base
	}
	return base + ":" + sig
}

// IsContentHashQueryStale informa se um ContentHash em cache foi gerado com uma versão de query/extrator
// diferente da atual para a linguagem informada.
func IsContentHashQueryStale(cachedHash string, cfg LanguageConfig) bool {
	if cfg == nil || cachedHash == "" {
		return false
	}
	if idx := strings.IndexByte(cachedHash, ':'); idx >= 0 {
		return cachedHash[idx+1:] != QuerySignature(cfg)
	}
	// Hashes SHA-256 reais de versões anteriores do Astrix têm 64 caracteres hex sem sufixo ":sig".
	return len(cachedHash) == 64
}
