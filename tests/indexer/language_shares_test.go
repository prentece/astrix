package indexer_test

import (
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"testing"

	"github.com/stretchr/testify/assert"
)

func fileStates(sizes map[string]int64) map[string]*storage.ProjectFileState {
	out := make(map[string]*storage.ProjectFileState, len(sizes))
	for path, size := range sizes {
		out[path] = &storage.ProjectFileState{FilePath: path, FileSize: size}
	}
	return out
}

func TestComputeLanguageShares_NilAndEmpty(t *testing.T) {
	assert.Empty(t, indexer.ComputeLanguageShares(nil))
	assert.Empty(t, indexer.ComputeLanguageShares(map[string]*storage.ProjectFileState{}))
}

func TestComputeLanguageShares_NilEntryIsIgnored(t *testing.T) {
	states := fileStates(map[string]int64{"a.go": 100})
	states["ghost.go"] = nil

	shares := indexer.ComputeLanguageShares(states)

	assert.Equal(t, []storage.LanguageShare{{Name: "go", Percent: 100}}, shares)
}

func TestComputeLanguageShares_SingleLanguageAggregatesFiles(t *testing.T) {
	shares := indexer.ComputeLanguageShares(fileStates(map[string]int64{
		"a.go":     300,
		"pkg/b.go": 700,
	}))

	assert.Equal(t, []storage.LanguageShare{{Name: "go", Percent: 100}}, shares)
}

func TestComputeLanguageShares_MixedOrderedByShare(t *testing.T) {
	shares := indexer.ComputeLanguageShares(fileStates(map[string]int64{
		"a.go":       840,
		"scripts.py": 100,
		"Main.java":  60,
	}))

	assert.Equal(t, []storage.LanguageShare{
		{Name: "go", Percent: 84.0},
		{Name: "python", Percent: 10.0},
		{Name: "java", Percent: 6.0},
	}, shares)
}

func TestComputeLanguageShares_TieBreaksByName(t *testing.T) {
	shares := indexer.ComputeLanguageShares(fileStates(map[string]int64{
		"z.py": 50,
		"a.go": 50,
	}))

	assert.Equal(t, []storage.LanguageShare{
		{Name: "go", Percent: 50},
		{Name: "python", Percent: 50},
	}, shares)
}

func TestComputeLanguageShares_RoundsToOneDecimal(t *testing.T) {
	shares := indexer.ComputeLanguageShares(fileStates(map[string]int64{
		"a.go":   1,
		"b.py":   1,
		"C.java": 1,
	}))

	assert.Len(t, shares, 3)
	for _, s := range shares {
		assert.Equal(t, 33.3, s.Percent, s.Name)
	}
}

func TestComputeLanguageShares_IgnoresNonPositiveSizes(t *testing.T) {
	shares := indexer.ComputeLanguageShares(fileStates(map[string]int64{
		"a.go":   100,
		"b.py":   0,
		"C.java": -5,
	}))

	assert.Equal(t, []storage.LanguageShare{{Name: "go", Percent: 100}}, shares)
}

func TestComputeLanguageShares_AllNonPositiveYieldsEmpty(t *testing.T) {
	shares := indexer.ComputeLanguageShares(fileStates(map[string]int64{
		"a.go": 0,
		"b.py": -1,
	}))

	assert.Empty(t, shares)
}
