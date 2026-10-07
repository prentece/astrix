package indexer

import (
	"astrix/pkg/storage"
	"fmt"
	"os"
	"path/filepath"
)

// DeltaEngine gerencia a detecção de alterações em repositórios (Stat Cache + Event Stat).
type DeltaEngine struct {
	fileStateRepo storage.FileStateRepository
}

// NewDeltaEngine cria uma nova instância de DeltaEngine.
func NewDeltaEngine(fileStateRepo storage.FileStateRepository) *DeltaEngine {
	return &DeltaEngine{
		fileStateRepo: fileStateRepo,
	}
}

// DetectDelta identifica alterações de arquivos em um projeto comparando o estado no disco com o banco do Astrix.
func (d *DeltaEngine) DetectDelta(projectID, projectPath string) (*storage.DeltaResult, error) {
	cachedStates, err := d.fileStateRepo.ListByProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("erro ao recuperar estados em cache do projeto: %w", err)
	}

	// Detecção baseada no cache do Astrix (compara mtime e file_size do disco com o banco de dados).
	// O status do Git rastreia commits locais, o que NÃO deve ser critério de desync do Astrix.
	return d.detectViaStatCache(projectPath, cachedStates)
}

// DetectDeltaForFiles verifica apenas os arquivos candidatos informados contra o cache de estados,
// evitando escanear todo o repositório com filepath.Walk quando os caminhos alterados já são conhecidos.
// Se candidateRelPaths estiver vazio, realiza o scan completo tradicional (fallback).
func (d *DeltaEngine) DetectDeltaForFiles(projectID, projectPath string, candidateRelPaths []string) (*storage.DeltaResult, error) {
	if len(candidateRelPaths) == 0 {
		return d.DetectDelta(projectID, projectPath)
	}

	cachedStates, err := d.fileStateRepo.ListByProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("erro ao recuperar estados em cache do projeto: %w", err)
	}

	result := &storage.DeltaResult{
		ModifiedFiles: []string{},
		AddedFiles:    []string{},
		DeletedFiles:  []string{},
		RenamedFiles:  make(map[string]string),
		Strategy:      "event_stat",
	}

	seen := make(map[string]bool)
	for _, relPath := range candidateRelPaths {
		relPath = filepath.ToSlash(filepath.Clean(relPath))
		if seen[relPath] || relPath == "." || relPath == "" {
			continue
		}
		seen[relPath] = true

		absPath, err := SafeJoin(projectPath, relPath)
		if err != nil {
			continue
		}

		info, err := os.Stat(absPath)
		state, exists := cachedStates[relPath]

		if err != nil {
			if os.IsNotExist(err) {
				if exists {
					result.DeletedFiles = append(result.DeletedFiles, relPath)
				}
			}
			continue
		}

		if info.IsDir() {
			continue
		}

		// Checa se o arquivo é suportado pelas linguagens cadastradas
		if _, ok := GetConfigByFilePath(relPath); !ok {
			continue
		}

		mtime := info.ModTime().Unix()
		size := info.Size()

		if !exists {
			result.AddedFiles = append(result.AddedFiles, relPath)
		} else if state.MTime != mtime || state.FileSize != size {
			result.ModifiedFiles = append(result.ModifiedFiles, relPath)
		} else {
			result.UnchangedCount++
		}
	}

	return result, nil
}

// detectViaStatCache itera sobre os arquivos do projeto comparando mtime e file_size sem ler conteúdo.
func (d *DeltaEngine) detectViaStatCache(projectPath string, cachedStates map[string]*storage.ProjectFileState) (*storage.DeltaResult, error) {
	files, err := ScanRepository(projectPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao escanear diretório do projeto: %w", err)
	}

	result := &storage.DeltaResult{
		ModifiedFiles: []string{},
		AddedFiles:    []string{},
		DeletedFiles:  []string{},
		RenamedFiles:  make(map[string]string),
		Strategy:      "stat",
	}

	diskFilesMap := make(map[string]bool)

	for _, f := range files {
		diskFilesMap[f.RelPath] = true

		state, exists := cachedStates[f.RelPath]
		if !exists {
			// Arquivo novo não presente no banco
			result.AddedFiles = append(result.AddedFiles, f.RelPath)
			continue
		}

		info, err := os.Stat(f.AbsPath)
		if err != nil {
			continue
		}

		mtime := info.ModTime().Unix()
		size := info.Size()

		// Se mtime e size são idênticos, pula sem ler bytes do disco
		if state.MTime == mtime && state.FileSize == size {
			result.UnchangedCount++
			continue
		}

		// Arquivo modificado no disco
		result.ModifiedFiles = append(result.ModifiedFiles, f.RelPath)
	}

	// Identificar arquivos que foram excluídos do disco
	for cachedPath := range cachedStates {
		if !diskFilesMap[cachedPath] {
			result.DeletedFiles = append(result.DeletedFiles, cachedPath)
		}
	}

	return result, nil
}
