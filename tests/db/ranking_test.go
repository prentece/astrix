package db_test

import (
	"astrix/pkg/storage"
	"path/filepath"
	"testing"
	"time"
)

func TestSymbolRanking_FindSymbol_OrderedByRelevance(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ranking_test.db")

	database, err := storage.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer database.Close()

	projRepo := storage.NewProjectRepo(database)
	symbolRepo := storage.NewSymbolRepo(database)

	project := &storage.Project{
		ID:        "proj_ranking_test",
		Name:      "Ranking Test",
		Path:      "/tmp/ranking_test",
		Language:  "typescript",
		Status:    storage.StatusReady,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := projRepo.Create(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Insere símbolos com scores de relevância distintos
	symbols := []*storage.Symbol{
		{
			ProjectID:      project.ID,
			File:           "src/users/user.spec.ts",
			Name:           "UserService",
			Kind:           storage.KindClass,
			Signature:      "class UserService",
			Language:       "typescript",
			StartLine:      10,
			EndLine:        20,
			StartByte:      100,
			EndByte:        200,
			RelevanceScore: 2.0, // Baixo score devido ao teste
		},
		{
			ProjectID:      project.ID,
			File:           "src/users/user.service.ts",
			Name:           "UserService",
			Kind:           storage.KindClass,
			Signature:      "class UserService",
			Language:       "typescript",
			StartLine:      5,
			EndLine:        50,
			StartByte:      50,
			EndByte:        500,
			RelevanceScore: 25.0, // Alto score de relevância
		},
		{
			ProjectID:      project.ID,
			File:           "src/mocks/user_mock.ts",
			Name:           "UserServiceMock",
			Kind:           storage.KindClass,
			Signature:      "class UserServiceMock",
			Language:       "typescript",
			StartLine:      1,
			EndLine:        30,
			StartByte:      0,
			EndByte:        300,
			RelevanceScore: 0.5,
		},
	}

	if err := symbolRepo.SaveSymbols(project.ID, symbols); err != nil {
		t.Fatalf("failed to save symbols: %v", err)
	}

	// Busca por "UserService"
	results, _, err := symbolRepo.FindSymbol(project.ID, "UserService", 10, 0)
	if err != nil {
		t.Fatalf("failed to find symbols: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// 1º deve ser src/users/user.service.ts (score 25.0)
	if results[0].File != "src/users/user.service.ts" {
		t.Errorf("1st result expected 'src/users/user.service.ts', got '%s'", results[0].File)
	}

	// 2º deve ser src/users/user.spec.ts (score 2.0)
	if results[1].File != "src/users/user.spec.ts" {
		t.Errorf("2nd result expected 'src/users/user.spec.ts', got '%s'", results[1].File)
	}

	// 3º deve ser src/mocks/user_mock.ts (score 0.5)
	if results[2].File != "src/mocks/user_mock.ts" {
		t.Errorf("3rd result expected 'src/mocks/user_mock.ts', got '%s'", results[2].File)
	}
}
