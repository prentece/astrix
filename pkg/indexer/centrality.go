package indexer

import (
	"astrix/pkg/storage"
	"strings"
)

// CalculateProjectCentrality calcula a pontuação de centralidade e relevância de todos os símbolos de um projeto.
// score = (arquivos_que_importam_este_arquivo * 2) + referencias_ao_simbolo
// Aplicando penalidades heurísticas (-80% para testes/mocks, -30% para utils) e bônus (+20% para core architecture).
func CalculateProjectCentrality(projectID string, symbolRepo storage.SymbolRepository) error {
	fileImports, err := symbolRepo.GetFileImportCounts(projectID)
	if err != nil {
		return err
	}

	symbolRefs, err := symbolRepo.GetSymbolReferenceCounts(projectID)
	if err != nil {
		return err
	}

	allSymbols, err := symbolRepo.GetAllSymbolsForRanking(projectID)
	if err != nil {
		return err
	}

	if len(allSymbols) == 0 {
		return nil
	}

	scores := make(map[int64]float64, len(allSymbols))
	for _, sym := range allSymbols {
		fileInDegree := fileImports[sym.File]
		refCount := symbolRefs[sym.Name]

		score := CalculateSymbolScore(sym.File, sym.Name, fileInDegree, refCount)
		scores[sym.ID] = score
	}

	return symbolRepo.UpdateRelevanceScores(projectID, scores)
}

// CalculateSymbolScore calcula o score individual de um símbolo combinando in-degree com multiplicadores heurísticos.
func CalculateSymbolScore(filePath, symbolName string, fileInDegree, symbolRefCount int) float64 {
	// Base: cada arquivo que importa este arquivo vale 2 pontos, e cada referência direta vale 1 ponto
	baseScore := float64(fileInDegree*2 + symbolRefCount)

	// Garante score mínimo base de 1.0 para que multiplicadores funcionem
	if baseScore <= 0 {
		baseScore = 1.0
	}

	lowerPath := strings.ToLower(filePath)
	lowerName := strings.ToLower(symbolName)

	multiplier := 1.0

	// 1. Penalidade de Testes e Mocks (-80%)
	if isTestOrMock(lowerPath, lowerName) {
		multiplier *= 0.2
	} else if isUtility(lowerPath) {
		// 2. Penalidade de Utilitários (-30%)
		multiplier *= 0.7
	} else if isCoreArchitecture(lowerPath, lowerName) {
		// 3. Bônus de Arquitetura Central (+20%) apenas se NÃO for teste ou utilitário
		multiplier *= 1.2
	}

	return baseScore * multiplier
}

// isTestOrMock verifica se o arquivo ou símbolo pertence a suíte de testes, mocks ou fixtures.
func isTestOrMock(lowerPath, lowerName string) bool {
	if strings.Contains(lowerPath, ".spec.") ||
		strings.Contains(lowerPath, ".test.") ||
		strings.HasSuffix(lowerPath, "_test.go") ||
		strings.HasSuffix(lowerPath, "test.go") ||
		strings.HasSuffix(lowerPath, "test.java") ||
		strings.Contains(lowerPath, "/tests/") ||
		strings.Contains(lowerPath, "/test/") ||
		strings.Contains(lowerPath, "/__tests__/") ||
		strings.Contains(lowerPath, "/mocks/") ||
		strings.Contains(lowerPath, "/mock/") ||
		strings.Contains(lowerPath, "/fixtures/") ||
		strings.Contains(lowerPath, "/stub/") ||
		strings.Contains(lowerPath, "/stubs/") {
		return true
	}

	if strings.HasSuffix(lowerName, "mock") ||
		strings.HasPrefix(lowerName, "mock") ||
		strings.HasSuffix(lowerName, "test") ||
		strings.HasPrefix(lowerName, "test") {
		return true
	}

	return false
}

// isUtility verifica se o caminho pertence a diretórios utilitários ou constantes.
func isUtility(lowerPath string) bool {
	return strings.Contains(lowerPath, "/utils/") ||
		strings.Contains(lowerPath, "/util/") ||
		strings.Contains(lowerPath, "/helpers/") ||
		strings.Contains(lowerPath, "/helper/") ||
		strings.Contains(lowerPath, "/constants/") ||
		strings.Contains(lowerPath, "/const/")
}

// isCoreArchitecture verifica se o caminho ou símbolo representa componentes centrais de domínio/arquitetura.
func isCoreArchitecture(lowerPath, lowerName string) bool {
	keywords := []string{
		"controller", "service", "module", "main", "app",
		"handler", "repository", "domain", "entity", "usecase",
		"model", "dao", "router", "gateway",
	}

	for _, kw := range keywords {
		if strings.Contains(lowerPath, kw) || strings.Contains(lowerName, kw) {
			return true
		}
	}

	return false
}
