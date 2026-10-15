package indexer

import (
	"astrix/pkg/storage"
	"math"
	"sort"
)

// ComputeLanguageShares agrega o tamanho (bytes) dos arquivos indexados por linguagem
// e devolve a participação percentual de cada uma, com 1 casa decimal.
//
// A linguagem de cada arquivo é inferida pela extensão (detectLanguageByExtension).
// Entradas nil e arquivos com FileSize <= 0 são ignorados. A ordenação é determinística:
// maior participação primeiro, empate resolvido pelo nome.
func ComputeLanguageShares(states map[string]*storage.ProjectFileState) []storage.LanguageShare {
	bytesByLang := make(map[string]int64)
	var total int64
	for path, st := range states {
		if st == nil || st.FileSize <= 0 {
			continue
		}
		filePath := st.FilePath
		if filePath == "" {
			filePath = path
		}
		bytesByLang[detectLanguageByExtension(filePath)] += st.FileSize
		total += st.FileSize
	}
	if total == 0 {
		return nil
	}

	type entry struct {
		name  string
		bytes int64
	}
	entries := make([]entry, 0, len(bytesByLang))
	for name, b := range bytesByLang {
		entries = append(entries, entry{name: name, bytes: b})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].bytes != entries[j].bytes {
			return entries[i].bytes > entries[j].bytes
		}
		return entries[i].name < entries[j].name
	})

	shares := make([]storage.LanguageShare, 0, len(entries))
	for _, e := range entries {
		pct := math.Round(float64(e.bytes)/float64(total)*1000) / 10
		shares = append(shares, storage.LanguageShare{Name: e.name, Percent: pct})
	}
	return shares
}

// ProjectLanguageShares calcula a distribuição por linguagem de um projeto a partir dos
// estados de arquivo persistidos. É seguro chamar com receiver nil ou sem repositório de
// estados configurado: nesses casos devolve (nil, nil), sem distribuição.
func (e *Engine) ProjectLanguageShares(projectID string) ([]storage.LanguageShare, error) {
	if e == nil || e.fileStateRepo == nil {
		return nil, nil
	}
	states, err := e.fileStateRepo.ListByProject(projectID)
	if err != nil {
		return nil, err
	}
	return ComputeLanguageShares(states), nil
}
