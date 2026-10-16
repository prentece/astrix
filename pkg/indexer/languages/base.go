package languages

import (
	"astrix/pkg/indexer"
	"astrix/pkg/storage"

	sitter "github.com/smacker/go-tree-sitter"
)

// BaseLanguageConfig é uma implementação declarativa de indexer.LanguageConfig
// e indexer.ImportSiteExtractor para registrar novas linguagens com mínimo boilerplate.
type BaseLanguageConfig struct {
	LangName    string
	Language    *sitter.Language
	Exts        []string
	Symbols     string
	Callers     string
	Variables   string
	ImportSites string
	Tests       []indexer.TestPattern
}

func (c *BaseLanguageConfig) Name() string {
	return c.LangName
}

func (c *BaseLanguageConfig) GetLanguage() *sitter.Language {
	return c.Language
}

func (c *BaseLanguageConfig) SymbolsQuery() string {
	return c.Symbols
}

func (c *BaseLanguageConfig) CallersQuery() string {
	return c.Callers
}

func (c *BaseLanguageConfig) VariablesQuery() string {
	return c.Variables
}

func (c *BaseLanguageConfig) TestPatterns() []indexer.TestPattern {
	return c.Tests
}

func (c *BaseLanguageConfig) Extensions() []string {
	return c.Exts
}

func (c *BaseLanguageConfig) ExtractImportSites(projectID, relPath string, content []byte, rootNode *sitter.Node) []*storage.CallerInfo {
	if c.ImportSites == "" || c.Language == nil {
		return nil
	}
	return extractImportSitesWithQuery(c.ImportSites, c.Language, projectID, relPath, content, rootNode)
}
