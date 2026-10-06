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

func TestEngine_QueryCacheAndSinglePassExtraction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "astrix_cache_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := storage.NewDatabase(dbPath)
	require.NoError(t, err)
	defer database.Close()

	projectRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)
	depRepo := storage.NewDependencyGraphRepo(database)
	dataModelRepo := storage.NewDataModelRepo(database)

	engine := indexer.NewEngine(projectRepo, symbolRepo, depRepo, dataModelRepo)
	defer engine.Close()

	code := []byte(`package sample

// Service defines a test service
type Service struct {
	Name string
}

func (s *Service) Execute() string {
	return s.Name
}
`)

	config, ok := indexer.GetConfigByFilePath("service.go")
	require.True(t, ok)

	// Primeira extração: compila e armazena queries no cache
	res1, err := engine.ExtractASTDataAndDigest("proj-test", "service.go", code, config)
	require.NoError(t, err)
	require.NotNil(t, res1)
	assert.NotEmpty(t, res1.Symbols)
	assert.NotEmpty(t, res1.Digest)

	// Segunda extração: deve reutilizar queries compiladas do cache sem erros
	res2, err := engine.ExtractASTDataAndDigest("proj-test", "service.go", code, config)
	require.NoError(t, err)
	require.NotNil(t, res2)
	assert.Equal(t, len(res1.Symbols), len(res2.Symbols))
	assert.Equal(t, res1.Digest, res2.Digest)

	// Testa compatibilidade com ExtractASTData e ExtractASTDigest
	syms, refs, deps, models, err := engine.ExtractASTData("proj-test", "service.go", code, config)
	require.NoError(t, err)
	assert.Equal(t, len(res1.Symbols), len(syms))
	_ = refs
	_ = deps
	_ = models

	digest := engine.ExtractASTDigest("service.go", code, config)
	assert.Equal(t, res1.Digest, digest)

	// Fecha o engine para verificar liberação de recursos do cache
	err = engine.Close()
	assert.NoError(t, err)
}
