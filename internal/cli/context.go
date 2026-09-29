package cli

import (
	"astrix/pkg/storage"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const AstrixDir = ".astrix"

// ProjectConfig representa a configuração local persistida em .astrix/config.json
type ProjectConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Language  string `json:"language,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// ProjectContext representa o estado e metadados do diretório atual.
type ProjectContext struct {
	RootDir      string
	IsProject    bool
	IsRegistered bool
	ProjectID    string
	Name         string
	DetectedLang string
}

// CommonProjectMarkers lista arquivos que identificam uma pasta como projeto válido.
var CommonProjectMarkers = []string{
	".git",
	"go.mod",
	"package.json",
	"pom.xml",
	"build.gradle",
	"Cargo.toml",
	"composer.json",
	"pyproject.toml",
	"requirements.txt",
	"Gemfile",
	"Makefile",
}

// DetectContext inspeciona o diretório atual para resolver contexto e cadastro.
func DetectContext(dir string, projectRepo ...storage.ProjectRepository) (*ProjectContext, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("falha ao resolver caminho absoluto: %w", err)
	}

	ctx := &ProjectContext{
		RootDir:      absDir,
		Name:         filepath.Base(absDir),
		DetectedLang: detectProjectLanguage(absDir),
	}

	// 1. Verifica se é um diretório de projeto válido
	for _, marker := range CommonProjectMarkers {
		if _, err := os.Stat(filepath.Join(absDir, marker)); err == nil {
			ctx.IsProject = true
			break
		}
	}

	// 2. Verifica se possui a pasta .astrix cadastrada localmente
	astrixPath := filepath.Join(absDir, AstrixDir)
	if info, err := os.Stat(astrixPath); err == nil && info.IsDir() {
		configFile := filepath.Join(astrixPath, "config.json")
		if data, err := os.ReadFile(configFile); err == nil {
			var cfg ProjectConfig
			if err := json.Unmarshal(data, &cfg); err == nil && cfg.ID != "" {
				ctx.IsRegistered = true
				ctx.ProjectID = cfg.ID
				if cfg.Name != "" {
					ctx.Name = cfg.Name
				}
				return ctx, nil
			}
		}

		// Fallback para arquivo simples .astrix/id
		idFile := filepath.Join(astrixPath, "id")
		if data, err := os.ReadFile(idFile); err == nil {
			id := strings.TrimSpace(string(data))
			if id != "" {
				ctx.IsRegistered = true
				ctx.ProjectID = id
				return ctx, nil
			}
		}
	}

	// 3. Fallback: Consulta o banco de dados global pelo caminho absoluto (auto-heal)
	if len(projectRepo) > 0 && projectRepo[0] != nil {
		all, _ := projectRepo[0].ListAll()
		for _, proj := range all {
			if proj != nil && proj.Path == absDir {
				ctx.IsProject = true
				ctx.IsRegistered = true
				ctx.ProjectID = proj.ID
				ctx.Name = proj.Name
				// Restaura automaticamente o arquivo .astrix/config.json local
				_ = SaveProjectConfig(absDir, proj.ID, proj.Name, proj.Language)
				return ctx, nil
			}
		}
	}

	return ctx, nil
}

// SaveProjectConfig grava os metadados do projeto na pasta .astrix/config.json
func SaveProjectConfig(dir, projectID, name, lang string) error {
	astrixPath := filepath.Join(dir, AstrixDir)
	if err := os.MkdirAll(astrixPath, 0755); err != nil {
		return fmt.Errorf("falha ao criar pasta %s: %w", AstrixDir, err)
	}

	cfg := ProjectConfig{
		ID:       projectID,
		Name:     name,
		Language: lang,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	configFile := filepath.Join(astrixPath, "config.json")
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("falha ao gravar config.json: %w", err)
	}

	return nil
}

// RemoveProjectConfig remove a pasta .astrix do diretório
func RemoveProjectConfig(dir string) error {
	astrixPath := filepath.Join(dir, AstrixDir)
	return os.RemoveAll(astrixPath)
}

func detectProjectLanguage(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
		return "typescript"
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return "javascript"
	}
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil ||
		fileExists(filepath.Join(dir, "requirements.txt")) {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(dir, "pom.xml")); err == nil ||
		fileExists(filepath.Join(dir, "build.gradle")) {
		return "java"
	}
	if _, err := os.Stat(filepath.Join(dir, "composer.json")); err == nil {
		return "php"
	}
	return "generic"
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
