package languages_test

import (
	"astrix/pkg/indexer"
	"context"
	"testing"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/stretchr/testify/require"
)

func parseAST(t *testing.T, cfg indexer.LanguageConfig, code string) *sitter.Node {
	t.Helper()
	parser := sitter.NewParser()
	parser.SetLanguage(cfg.GetLanguage())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tree, err := parser.ParseCtx(ctx, nil, []byte(code))
	require.NoError(t, err)
	require.NotNil(t, tree)
	return tree.RootNode()
}
