package indexer

import (
	"bytes"
	"astrix/pkg/storage"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DeltaEngine gerencia a detecção híbrida de alterações em repositórios (Git + Stat Cache).
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

// detectViaGit executa `git status --porcelain -uall` validando contra o cache de indexação.
func (d *DeltaEngine) detectViaGit(projectPath string, cachedStates map[string]*storage.ProjectFileState) (*storage.DeltaResult, error) {
	cmd := exec.Command("git", "status", "--porcelain", "-uall")
	cmd.Dir = projectPath

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git status falhou: %v (%s)", err, stderr.String())
	}

	result := &storage.DeltaResult{
		ModifiedFiles: []string{},
		AddedFiles:    []string{},
		DeletedFiles:  []string{},
		RenamedFiles:  make(map[string]string),
		Strategy:      "git",
	}

	lines := strings.Split(stdout.String(), "\n")
	gitModifiedSet := make(map[string]bool)

	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		statusCode := line[:2]
		rawPath := strings.TrimSpace(line[3:])

		// Tratar arquivos renomeados no git (ex: "R  old.go -> new.go" ou "R100 old.go -> new.go")
		if strings.HasPrefix(statusCode, "R") || strings.Contains(rawPath, " -> ") {
			parts := strings.Split(rawPath, " -> ")
			if len(parts) == 2 {
				oldPath := strings.Trim(strings.TrimSpace(parts[0]), "\"")
				newPath := strings.Trim(strings.TrimSpace(parts[1]), "\"")
				result.RenamedFiles[newPath] = oldPath
				result.ModifiedFiles = append(result.ModifiedFiles, newPath)
				result.DeletedFiles = append(result.DeletedFiles, oldPath)
				gitModifiedSet[oldPath] = true
				gitModifiedSet[newPath] = true
				continue
			}
		}

		filePath := strings.Trim(rawPath, "\"")
		filePath = filepath.ToSlash(filePath)
		gitModifiedSet[filePath] = true

		if strings.Contains(statusCode, "D") {
			if _, exists := cachedStates[filePath]; exists {
				result.DeletedFiles = append(result.DeletedFiles, filePath)
			}
		} else {
			absPath := filepath.Join(projectPath, filePath)
			info, err := os.Stat(absPath)
			if err != nil {
				if os.IsNotExist(err) {
					if _, exists := cachedStates[filePath]; exists {
						result.DeletedFiles = append(result.DeletedFiles, filePath)
					}
				}
				continue
			}

			mtime := info.ModTime().Unix()
			size := info.Size()
			state, exists := cachedStates[filePath]

			if !exists {
				result.AddedFiles = append(result.AddedFiles, filePath)
			} else if state.MTime != mtime || state.FileSize != size {
				result.ModifiedFiles = append(result.ModifiedFiles, filePath)
			} else {
				result.UnchangedCount++
			}
		}
	}

	// Arquivos no cache que não foram tocados pelo Git
	for path := range cachedStates {
		if !gitModifiedSet[path] {
			// Verifica se o arquivo ainda existe no disco
			absPath := filepath.Join(projectPath, path)
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				result.DeletedFiles = append(result.DeletedFiles, path)
			} else {
				result.UnchangedCount++
			}
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
