package mcp_test

import (
	"astrix/internal/mcp"
	"astrix/pkg/storage"
	"testing"

	"github.com/stretchr/testify/assert"
)

func projectWithLanguages(langs ...storage.LanguageShare) *storage.Project {
	return &storage.Project{
		ID:        "p1",
		Name:      "Astrix",
		Path:      "/tmp/astrix",
		Language:  "legacy-lang",
		Status:    storage.StatusReady,
		FileCount: 3,
		Languages: langs,
	}
}

func TestFormatProjects_JSON_WithLanguageShares(t *testing.T) {
	p := projectWithLanguages(
		storage.LanguageShare{Name: "go", Percent: 84},
		storage.LanguageShare{Name: "python", Percent: 16},
	)

	out := mcp.FormatProjects([]*storage.Project{p}, true)

	assert.Contains(t, out, `"primary_language": "go"`)
	assert.Contains(t, out, `"language": "go"`, "language é mantido, com o valor de primary_language")
	assert.NotContains(t, out, "legacy-lang")
	assert.Contains(t, out, `"languages"`)
	assert.Contains(t, out, `"name": "python"`)
	assert.Contains(t, out, `"percent": 16`)
}

func TestFormatProjects_JSON_FallbackToLegacyLanguage(t *testing.T) {
	p := projectWithLanguages()

	out := mcp.FormatProjects([]*storage.Project{p}, true)

	assert.Contains(t, out, `"language": "legacy-lang"`)
	assert.Contains(t, out, `"primary_language": "legacy-lang"`)
	assert.NotContains(t, out, `"languages"`)
}

func TestFormatProjects_Text_TopThreeLanguages(t *testing.T) {
	p := projectWithLanguages(
		storage.LanguageShare{Name: "go", Percent: 84},
		storage.LanguageShare{Name: "python", Percent: 10},
		storage.LanguageShare{Name: "java", Percent: 6},
	)

	out := mcp.FormatProjects([]*storage.Project{p}, false)

	assert.Contains(t, out, "go 84.0%, python 10.0%, java 6.0%")
	assert.NotContains(t, out, "other")
	assert.NotContains(t, out, "legacy-lang")
}

func TestFormatProjects_Text_GroupsRemainderAsOther(t *testing.T) {
	p := projectWithLanguages(
		storage.LanguageShare{Name: "go", Percent: 50},
		storage.LanguageShare{Name: "python", Percent: 20},
		storage.LanguageShare{Name: "java", Percent: 15},
		storage.LanguageShare{Name: "php", Percent: 10},
		storage.LanguageShare{Name: "typescript", Percent: 5},
	)

	out := mcp.FormatProjects([]*storage.Project{p}, false)

	assert.Contains(t, out, "go 50.0%, python 20.0%, java 15.0%, other 15.0%")
	assert.NotContains(t, out, "php")
}

func TestFormatProjects_Text_FallbackToLegacyLanguage(t *testing.T) {
	p := projectWithLanguages()

	out := mcp.FormatProjects([]*storage.Project{p}, false)

	assert.Contains(t, out, "legacy-lang")
	assert.NotContains(t, out, "%")
}
