package indexer_test

import (
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"testing"
)

func TestCalculateSymbolScore_Heuristics(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		symbolName   string
		fileInDegree int
		symbolRefs   int
		expectedMin  float64
		expectedMax  float64
	}{
		{
			name:         "Core service with high in-degree and refs",
			filePath:     "src/api/user.service.ts",
			symbolName:   "UserService",
			fileInDegree: 5,
			symbolRefs:   10,
			// Base: (5*2 + 10) = 20. Bonus 1.2 => 24.0
			expectedMin: 23.9,
			expectedMax: 24.1,
		},
		{
			name:         "Controller with moderate in-degree",
			filePath:     "internal/handlers/order_controller.go",
			symbolName:   "OrderController",
			fileInDegree: 2,
			symbolRefs:   4,
			// Base: (2*2 + 4) = 8. Bonus 1.2 => 9.6
			expectedMin: 9.5,
			expectedMax: 9.7,
		},
		{
			name:         "Test file with same base refs receives 80% penalty",
			filePath:     "src/api/user.spec.ts",
			symbolName:   "UserServiceTest",
			fileInDegree: 5,
			symbolRefs:   10,
			// Base: (5*2 + 10) = 20. Penalty 0.2 => 4.0
			expectedMin: 3.9,
			expectedMax: 4.1,
		},
		{
			name:         "Go test file receives penalty",
			filePath:     "pkg/auth/auth_test.go",
			symbolName:   "TestAuthenticate",
			fileInDegree: 1,
			symbolRefs:   0,
			// Base: (1*2 + 0) = 2. Penalty 0.2 => 0.4
			expectedMin: 0.39,
			expectedMax: 0.41,
		},
		{
			name:         "Mock receives penalty",
			filePath:     "tests/mocks/user_mock.go",
			symbolName:   "MockUserRepository",
			fileInDegree: 3,
			symbolRefs:   5,
			// Base: (3*2 + 5) = 11. Penalty 0.2 => 2.2
			expectedMin: 2.19,
			expectedMax: 2.21,
		},
		{
			name:         "Utility file receives 30% penalty",
			filePath:     "src/utils/date_helpers.ts",
			symbolName:   "formatDate",
			fileInDegree: 4,
			symbolRefs:   2,
			// Base: (4*2 + 2) = 10. Penalty 0.7 => 7.0
			expectedMin: 6.9,
			expectedMax: 7.1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := indexer.CalculateSymbolScore(tc.filePath, tc.symbolName, tc.fileInDegree, tc.symbolRefs)
			if score < tc.expectedMin || score > tc.expectedMax {
				t.Fatalf("score for %s:%s got %f, expected between %f and %f",
					tc.filePath, tc.symbolName, score, tc.expectedMin, tc.expectedMax)
			}
		})
	}
}

// MockSymbolRepo para testes unitários de cálculo de centralidade de projeto
type mockSymbolRepo struct {
	fileImports   map[string]int
	symbolRefs    map[string]int
	symbols       []*storage.Symbol
	updatedScores map[int64]float64
}

func (m *mockSymbolRepo) ClearProjectData(projectID string) error                      { return nil }
func (m *mockSymbolRepo) DeleteByFile(projectID, file string) error                    { return nil }
func (m *mockSymbolRepo) SaveSymbols(projectID string, symbols []*storage.Symbol) error { return nil }

func (m *mockSymbolRepo) SaveReferences(projectID string, references []*storage.CallerInfo) error {
	return nil
}

func (m *mockSymbolRepo) FindSymbol(projectID, name string, limit, offset int) ([]*storage.Symbol, bool, error) {
	return nil, false, nil
}

func (m *mockSymbolRepo) GetSymbolByFileAndName(projectID, file, symbolName string) (*storage.Symbol, error) {
	return nil, nil
}

func (m *mockSymbolRepo) FindReferences(projectID, symbolName string, limit, offset int) ([]*storage.CallerInfo, bool, error) {
	return nil, false, nil
}

func (m *mockSymbolRepo) GetFileImportCounts(projectID string) (map[string]int, error) {
	return m.fileImports, nil
}

func (m *mockSymbolRepo) GetSymbolReferenceCounts(projectID string) (map[string]int, error) {
	return m.symbolRefs, nil
}

func (m *mockSymbolRepo) GetAllSymbolsForRanking(projectID string) ([]*storage.Symbol, error) {
	return m.symbols, nil
}

func (m *mockSymbolRepo) UpdateRelevanceScores(projectID string, symbolScores map[int64]float64) error {
	m.updatedScores = symbolScores
	return nil
}

func (m *mockSymbolRepo) GetSymbolsByFileAndLineRange(projectID, file string, startLine, endLine int) ([]*storage.Symbol, error) {
	return nil, nil
}

func (m *mockSymbolRepo) GetSymbolCountsByFile(projectID string) ([]*storage.FileSymbolStats, error) {
	return nil, nil
}

func TestCalculateProjectCentrality(t *testing.T) {
	mockRepo := &mockSymbolRepo{
		fileImports: map[string]int{
			"src/services/user.service.ts": 8,
			"src/services/user.spec.ts":    0,
			"src/utils/helpers.ts":         2,
		},
		symbolRefs: map[string]int{
			"UserService":     15,
			"UserServiceTest": 1,
			"formatString":    3,
		},
		symbols: []*storage.Symbol{
			{ID: 1, File: "src/services/user.service.ts", Name: "UserService"},
			{ID: 2, File: "src/services/user.spec.ts", Name: "UserServiceTest"},
			{ID: 3, File: "src/utils/helpers.ts", Name: "formatString"},
		},
	}

	err := indexer.CalculateProjectCentrality("proj-1", mockRepo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mockRepo.updatedScores) != 3 {
		t.Fatalf("expected 3 updated scores, got %d", len(mockRepo.updatedScores))
	}

	userServiceScore := mockRepo.updatedScores[1]
	userTestScore := mockRepo.updatedScores[2]
	helperScore := mockRepo.updatedScores[3]

	// UserService (core + imported) MUST be highest
	if userServiceScore <= helperScore {
		t.Fatalf("UserService score (%f) must be greater than helper score (%f)", userServiceScore, helperScore)
	}
	if helperScore <= userTestScore {
		t.Fatalf("Helper score (%f) must be greater than test score (%f)", helperScore, userTestScore)
	}
}
