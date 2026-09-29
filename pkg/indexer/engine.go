package indexer

import (
	"bufio"
	"astrix/pkg/storage"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	ignore "github.com/sabhiram/go-gitignore"
	sitter "github.com/smacker/go-tree-sitter"
)

// Engine coordena a análise sintática de código via Tree-sitter, busca textual e a persistência desacoplada de dados.
type Engine struct {
	projectRepo   storage.ProjectRepository
	symbolRepo    storage.SymbolRepository
	depRepo       storage.DependencyGraphRepository
	dataModelRepo storage.DataModelRepository
	fileStateRepo storage.FileStateRepository
	indexingMu    sync.Mutex
	indexingMap   map[string]bool
	progressMap   map[string]*storage.IndexingProgress
}

// NewEngine inicializa o motor de indexação e operações utilizando interfaces de repositório.
func NewEngine(
	projectRepo storage.ProjectRepository,
	symbolRepo storage.SymbolRepository,
	depRepo storage.DependencyGraphRepository,
	dataModelRepo storage.DataModelRepository,
) *Engine {
	return &Engine{
		projectRepo:   projectRepo,
		symbolRepo:    symbolRepo,
		depRepo:       depRepo,
		dataModelRepo: dataModelRepo,
		indexingMap:   make(map[string]bool),
		progressMap:   make(map[string]*storage.IndexingProgress),
	}
}

// GetIndexingProgress retorna uma cópia thread-safe do progresso atual de indexação de um projeto.
func (e *Engine) GetIndexingProgress(projectID string) *storage.IndexingProgress {
	e.indexingMu.Lock()
	defer e.indexingMu.Unlock()
	if p, ok := e.progressMap[projectID]; ok && p != nil {
		copy := *p
		return &copy
	}
	return nil
}

// SetFileStateRepo injeta o repositório de estados de arquivo.
func (e *Engine) SetFileStateRepo(repo storage.FileStateRepository) {
	e.fileStateRepo = repo
}

// detectLanguageByExtension detecta a linguagem de programação com base na extensão do arquivo.
func detectLanguageByExtension(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	langMap := map[string]string{
		".go":    "go",
		".ts":    "typescript",
		".tsx":   "typescript",
		".js":    "javascript",
		".jsx":   "javascript",
		".py":    "python",
		".java":  "java",
		".kt":    "kotlin",
		".rs":    "rust",
		".rb":    "ruby",
		".php":   "php",
		".cs":    "csharp",
		".cpp":   "cpp",
		".c":     "c",
		".h":     "c",
		".hpp":   "cpp",
		".swift": "swift",
		".sql":   "sql",
		".html":  "html",
		".css":   "css",
		".scss":  "scss",
		".yaml":  "yaml",
		".yml":   "yaml",
		".json":  "json",
		".xml":   "xml",
		".md":    "markdown",
		".sh":    "shell",
		".bash":  "shell",
		".lua":   "lua",
		".dart":  "dart",
		".ex":    "elixir",
		".exs":   "elixir",
	}
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	if ext != "" {
		return ext[1:] // retorna extensão sem o ponto como fallback
	}
	return "text"
}

// GetFileState retorna o estado de arquivo em cache para o projeto e caminho relativo.
func (e *Engine) GetFileState(projectID, filepath string) (*storage.ProjectFileState, error) {
	if e.fileStateRepo == nil {
		return nil, nil
	}
	return e.fileStateRepo.Get(projectID, filepath)
}

// IndexProjectAsync dispara a indexação de um projeto em segundo plano.
func (e *Engine) IndexProjectAsync(projectID string) {
	go func() {
		if err := e.IndexProject(projectID); err != nil {
			log.Printf("[INDEXER ERROR] Falha ao indexar projeto %s: %v\n", projectID, err)
		}
	}()
}

// IndexProject executa o processo completo de indexação de um projeto.
func (e *Engine) IndexProject(projectID string) error {
	e.indexingMu.Lock()
	if e.indexingMap[projectID] {
		e.indexingMu.Unlock()
		return fmt.Errorf("projeto %s já está sendo indexado", projectID)
	}
	e.indexingMap[projectID] = true
	e.indexingMu.Unlock()

	defer func() {
		e.indexingMu.Lock()
		delete(e.indexingMap, projectID)
		e.indexingMu.Unlock()
	}()

	return e.indexProjectInternal(projectID)
}

// indexProjectInternal executa a lógica de indexação completa sem adquirir indexingMu novamente.
func (e *Engine) indexProjectInternal(projectID string) error {
	defer func() {
		e.indexingMu.Lock()
		delete(e.progressMap, projectID)
		e.indexingMu.Unlock()
	}()

	proj, err := e.projectRepo.GetByID(projectID)
	if err != nil {
		return err
	}
	if proj == nil {
		return fmt.Errorf("projeto %s não encontrado", projectID)
	}

	log.Printf("[INDEXER] Iniciando indexação do projeto '%s' (ID: %s, Caminho: %s)\n", proj.Name, proj.ID, proj.Path)
	_ = e.projectRepo.UpdateStatus(proj.ID, storage.StatusIndexing, "", 0, 0)

	startTime := time.Now()

	// 1. Escaneia todos os arquivos do repositório respeitando .gitignore
	files, err := ScanRepository(proj.Path)
	if err != nil {
		errMsg := fmt.Sprintf("falha ao escanear repositório: %v", err)
		_ = e.projectRepo.UpdateStatus(proj.ID, storage.StatusError, errMsg, 0, 0)
		return err
	}

	log.Printf("[INDEXER] Encontrados %d arquivos suportados em '%s'\n", len(files), proj.Name)

	e.indexingMu.Lock()
	e.progressMap[proj.ID] = &storage.IndexingProgress{
		ProjectID:      proj.ID,
		IsIndexing:     true,
		ProcessedFiles: 0,
		TotalFiles:     len(files),
		Percent:        0,
	}
	e.indexingMu.Unlock()

	// 2. Limpa dados de indexações anteriores deste projeto
	if err := e.symbolRepo.ClearProjectData(proj.ID); err != nil {
		errMsg := fmt.Sprintf("falha ao limpar índice antigo: %v", err)
		_ = e.projectRepo.UpdateStatus(proj.ID, storage.StatusError, errMsg, 0, 0)
		return err
	}
	if e.depRepo != nil {
		_ = e.depRepo.ClearProjectDependencies(proj.ID)
	}
	if e.dataModelRepo != nil {
		_ = e.dataModelRepo.ClearProjectDataModels(proj.ID)
	}

	var allSymbols []*storage.Symbol
	var allRefs []*storage.CallerInfo
	var allDeps []*storage.DependencyEdge
	var allModels []*storage.DataModel
	var allFileStates []*storage.ProjectFileState

	// 3. Processa arquivos em paralelo via Worker Pool aproveitando múltiplos núcleos de CPU
	numWorkers := runtime.NumCPU()
	if numWorkers > 16 {
		numWorkers = 16
	}
	if numWorkers > len(files) {
		numWorkers = len(files)
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	type indexJob struct {
		idx  int
		file FileInfo
	}

	type indexResult struct {
		idx        int
		file       FileInfo
		symbols    []*storage.Symbol
		refs       []*storage.CallerInfo
		deps       []*storage.DependencyEdge
		modelsList []*storage.DataModel
		fileState  *storage.ProjectFileState
		err        error
	}

	jobCh := make(chan indexJob, len(files))
	resultCh := make(chan indexResult, numWorkers*2)

	for i, f := range files {
		jobCh <- indexJob{idx: i, file: f}
	}
	close(jobCh)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				content, err := os.ReadFile(job.file.AbsPath)
				if err != nil {
					log.Printf("[INDEXER WARN] Erro ao ler %s: %v\n", job.file.RelPath, err)
					resultCh <- indexResult{idx: job.idx, file: job.file, err: err}
					continue
				}

				symbols, refs, deps, modelsList, err := e.ExtractASTData(proj.ID, job.file.RelPath, content, job.file.Config)
				if err != nil {
					log.Printf("[INDEXER WARN] Erro ao processar AST de %s: %v\n", job.file.RelPath, err)
					resultCh <- indexResult{idx: job.idx, file: job.file, err: err}
					continue
				}

				var state *storage.ProjectFileState
				if e.fileStateRepo != nil {
					info, _ := os.Stat(job.file.AbsPath)
					mtime := int64(0)
					fileSize := int64(len(content))
					if info != nil {
						mtime = info.ModTime().Unix()
						fileSize = info.Size()
					}
					cHash := calculateHash(content)
					digest := e.ExtractASTDigest(job.file.RelPath, content, job.file.Config)
					dHash := calculateHash([]byte(digest))
					state = &storage.ProjectFileState{
						ProjectID:   proj.ID,
						FilePath:    job.file.RelPath,
						MTime:       mtime,
						FileSize:    fileSize,
						ContentHash: cHash,
						DigestHash:  dHash,
					}
				}

				resultCh <- indexResult{
					idx:        job.idx,
					file:       job.file,
					symbols:    symbols,
					refs:       refs,
					deps:       deps,
					modelsList: modelsList,
					fileState:  state,
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	processedCount := 0
	for res := range resultCh {
		processedCount++
		if res.err == nil {
			allSymbols = append(allSymbols, res.symbols...)
			allRefs = append(allRefs, res.refs...)
			allDeps = append(allDeps, res.deps...)
			allModels = append(allModels, res.modelsList...)
			if res.fileState != nil {
				allFileStates = append(allFileStates, res.fileState)
			}
		}

		pct := 0
		if len(files) > 0 {
			pct = (processedCount * 100) / len(files)
			if pct > 100 {
				pct = 100
			}
		}

		e.indexingMu.Lock()
		if prog := e.progressMap[proj.ID]; prog != nil {
			prog.ProcessedFiles = processedCount
			prog.CurrentFile = res.file.RelPath
			prog.Percent = pct
		}
		e.indexingMu.Unlock()
	}

	// 4. Salva os símbolos, referências, dependências e modelos no SQLite
	if err := e.symbolRepo.SaveSymbols(proj.ID, allSymbols); err != nil {
		errMsg := fmt.Sprintf("falha ao salvar símbolos: %v", err)
		_ = e.projectRepo.UpdateStatus(proj.ID, storage.StatusError, errMsg, len(files), 0)
		return err
	}

	if err := e.symbolRepo.SaveReferences(proj.ID, allRefs); err != nil {
		log.Printf("[INDEXER WARN] Falha ao salvar referências: %v\n", err)
	}

	if e.depRepo != nil && len(allDeps) > 0 {
		if err := e.depRepo.SaveDependencies(proj.ID, allDeps); err != nil {
			log.Printf("[INDEXER WARN] Falha ao salvar dependências: %v\n", err)
		}
	}

	if e.dataModelRepo != nil && len(allModels) > 0 {
		if err := e.dataModelRepo.SaveDataModels(proj.ID, allModels); err != nil {
			log.Printf("[INDEXER WARN] Falha ao salvar modelos de dados: %v\n", err)
		}
	}

	// 5. Resolução e enriquecimento de target_file para as dependências
	if e.depRepo != nil && len(allSymbols) > 0 {
		symMap := make(map[string]string)
		for _, s := range allSymbols {
			if s.Name != "" && s.File != "" {
				symMap[s.Name] = s.File
			}
		}
		_ = e.depRepo.UpdateTargetFiles(proj.ID, symMap)
	}

	// 5.1. Cálculo de Centralidade e Relevância dos Símbolos (PageRank de Código)
	if err := CalculateProjectCentrality(proj.ID, e.symbolRepo); err != nil {
		log.Printf("[INDEXER WARN] Falha ao calcular centralidade do projeto: %v\n", err)
	}

	// 5.2. Persistência de baseline dos estados de arquivos (Two-Tier Hash em lote)
	if e.fileStateRepo != nil && len(allFileStates) > 0 {
		_ = e.fileStateRepo.DeleteByProject(proj.ID)
		if err := e.fileStateRepo.UpsertBatch(allFileStates); err != nil {
			log.Printf("[INDEXER WARN] Falha ao salvar estados de arquivos em lote: %v\n", err)
		}
	}

	// 6. Atualiza o status do projeto para concluído
	if err := e.projectRepo.UpdateStatus(proj.ID, storage.StatusReady, "", len(files), len(allSymbols)); err != nil {
		return err
	}

	log.Printf("[INDEXER SUCCESS] Projeto '%s' indexado em %v com sucesso: %d arquivos, %d símbolos, %d referências, %d dependências, %d modelos.\n",
		proj.Name, time.Since(startTime), len(files), len(allSymbols), len(allRefs), len(allDeps), len(allModels))

	return nil
}

// calculateHash calcula o hash SHA-256 em formato hexadecimal de uma fatia de bytes.
func calculateHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// ProcessIncrementalDelta executa a sincronização incremental com Two-Tier Hash evaluation.
func (e *Engine) ProcessIncrementalDelta(projectID string) (*storage.DeltaReport, error) {
	e.indexingMu.Lock()
	if e.indexingMap[projectID] {
		e.indexingMu.Unlock()
		return nil, fmt.Errorf("projeto %s já está sendo processado", projectID)
	}
	e.indexingMap[projectID] = true
	e.indexingMu.Unlock()

	defer func() {
		e.indexingMu.Lock()
		delete(e.indexingMap, projectID)
		delete(e.progressMap, projectID)
		e.indexingMu.Unlock()
	}()

	proj, err := e.projectRepo.GetByID(projectID)
	if err != nil || proj == nil {
		return nil, fmt.Errorf("projeto %s não encontrado", projectID)
	}

	startTime := time.Now()
	_ = e.projectRepo.UpdateStatus(proj.ID, storage.StatusIndexing, "", proj.FileCount, proj.SymbolCount)

	// Se fileStateRepo não estiver inicializado ou estiver vazio, faz fallback para indexProjectInternal
	if e.fileStateRepo == nil {
		if err := e.indexProjectInternal(projectID); err != nil {
			return nil, err
		}
		proj, _ = e.projectRepo.GetByID(projectID)
		return &storage.DeltaReport{
			ProjectID:    projectID,
			DurationMs:   time.Since(startTime).Milliseconds(),
			Strategy:     "full_fallback",
			FilesParsed:  proj.FileCount,
			TotalFiles:   proj.FileCount,
			TotalSymbols: proj.SymbolCount,
			Message:      "Reindexação completa realizada",
		}, nil
	}

	cachedStates, err := e.fileStateRepo.ListByProject(projectID)
	if err != nil || len(cachedStates) == 0 {
		// Se ainda não há estados salvos, roda o index completo para criar a baseline
		if err := e.indexProjectInternal(projectID); err != nil {
			return nil, err
		}
		proj, _ = e.projectRepo.GetByID(projectID)
		return &storage.DeltaReport{
			ProjectID:    projectID,
			DurationMs:   time.Since(startTime).Milliseconds(),
			Strategy:     "baseline_init",
			FilesParsed:  proj.FileCount,
			TotalFiles:   proj.FileCount,
			TotalSymbols: proj.SymbolCount,
			Message:      "Baseline de estados criada",
		}, nil
	}

	deltaEngine := NewDeltaEngine(e.fileStateRepo)
	delta, err := deltaEngine.DetectDelta(proj.ID, proj.Path)
	if err != nil {
		return nil, fmt.Errorf("erro ao detectar delta: %w", err)
	}

	report := &storage.DeltaReport{
		ProjectID: projectID,
		Strategy:  delta.Strategy,
	}

	// 1. Processar Arquivos Deletados (Remoções em Cascata)
	for _, delFile := range delta.DeletedFiles {
		_ = e.symbolRepo.DeleteByFile(proj.ID, delFile)
		if e.depRepo != nil {
			_ = e.depRepo.DeleteByFile(proj.ID, delFile)
		}
		if e.dataModelRepo != nil {
			_ = e.dataModelRepo.DeleteByFile(proj.ID, delFile)
		}
		_ = e.fileStateRepo.Delete(proj.ID, delFile)
		report.FilesDeleted++
	}

	// 2. Processar Arquivos Modificados e Adicionados (Two-Tier Evaluation)
	targetFiles := append(delta.AddedFiles, delta.ModifiedFiles...)
	var modifiedSymbols []*storage.Symbol
	var modifiedRefs []*storage.CallerInfo
	var modifiedDeps []*storage.DependencyEdge
	var modifiedModels []*storage.DataModel

	e.indexingMu.Lock()
	e.progressMap[proj.ID] = &storage.IndexingProgress{
		ProjectID:      proj.ID,
		IsIndexing:     true,
		ProcessedFiles: 0,
		TotalFiles:     len(targetFiles),
		Percent:        0,
	}
	e.indexingMu.Unlock()

	for idx, relPath := range targetFiles {
		absPath := filepath.Join(proj.Path, relPath)
		content, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}

		pct := 0
		if len(targetFiles) > 0 {
			pct = ((idx + 1) * 100) / len(targetFiles)
			if pct > 100 {
				pct = 100
			}
		}

		e.indexingMu.Lock()
		if prog := e.progressMap[proj.ID]; prog != nil {
			prog.ProcessedFiles = idx + 1
			prog.CurrentFile = relPath
			prog.Percent = pct
		}
		e.indexingMu.Unlock()

		info, _ := os.Stat(absPath)
		mtime := int64(0)
		fileSize := int64(len(content))
		if info != nil {
			mtime = info.ModTime().Unix()
			fileSize = info.Size()
		}

		config, ok := GetConfigByFilePath(relPath)
		if !ok {
			continue
		}

		contentHash := calculateHash(content)
		oldState := cachedStates[relPath]

		// Nível 1: content_hash check (AST evaluation)
		if oldState != nil && oldState.ContentHash == contentHash {
			report.FilesSkipped++
			_ = e.fileStateRepo.Upsert(&storage.ProjectFileState{
				ProjectID:     proj.ID,
				FilePath:      relPath,
				MTime:         mtime,
				FileSize:      fileSize,
				ContentHash:   contentHash,
				DigestHash:    oldState.DigestHash,
				LastIndexedAt: time.Now(),
			})
			continue
		}

		// O conteúdo mudou: reparseia AST deste arquivo
		_ = e.symbolRepo.DeleteByFile(proj.ID, relPath)
		if e.depRepo != nil {
			_ = e.depRepo.DeleteByFile(proj.ID, relPath)
		}
		if e.dataModelRepo != nil {
			_ = e.dataModelRepo.DeleteByFile(proj.ID, relPath)
		}

		syms, refs, deps, modelsList, err := e.ExtractASTData(proj.ID, relPath, content, config)
		if err == nil {
			modifiedSymbols = append(modifiedSymbols, syms...)
			modifiedRefs = append(modifiedRefs, refs...)
			modifiedDeps = append(modifiedDeps, deps...)
			modifiedModels = append(modifiedModels, modelsList...)
		}
		report.FilesParsed++

		// Nível 2: digest_hash check
		digest := e.ExtractASTDigest(relPath, content, config)
		digestHash := calculateHash([]byte(digest))

		// Atualiza estado no banco
		_ = e.fileStateRepo.Upsert(&storage.ProjectFileState{
			ProjectID:     proj.ID,
			FilePath:      relPath,
			MTime:         mtime,
			FileSize:      fileSize,
			ContentHash:   contentHash,
			DigestHash:    digestHash,
			LastIndexedAt: time.Now(),
		})
	}

	// 3. Salva novos símbolos, dependências e modelos
	if len(modifiedSymbols) > 0 {
		_ = e.symbolRepo.SaveSymbols(proj.ID, modifiedSymbols)
	}
	if len(modifiedRefs) > 0 {
		_ = e.symbolRepo.SaveReferences(proj.ID, modifiedRefs)
	}
	if e.depRepo != nil && len(modifiedDeps) > 0 {
		_ = e.depRepo.SaveDependencies(proj.ID, modifiedDeps)
	}
	if e.dataModelRepo != nil && len(modifiedModels) > 0 {
		_ = e.dataModelRepo.SaveDataModels(proj.ID, modifiedModels)
	}

	// 4. Se houve alterações de símbolos, recalcula Centralidade e dependências
	if report.FilesParsed > 0 || report.FilesDeleted > 0 {
		allSymbols, _ := e.symbolRepo.GetAllSymbolsForRanking(proj.ID)
		if e.depRepo != nil && len(allSymbols) > 0 {
			symMap := make(map[string]string)
			for _, s := range allSymbols {
				if s.Name != "" && s.File != "" {
					symMap[s.Name] = s.File
				}
			}
			_ = e.depRepo.UpdateTargetFiles(proj.ID, symMap)
		}
		_ = CalculateProjectCentrality(proj.ID, e.symbolRepo)

		// Atualizar contagens em projects
		currentStates, _ := e.fileStateRepo.ListByProject(proj.ID)
		totalFiles := len(currentStates)
		totalSymbols := len(allSymbols)
		_ = e.projectRepo.UpdateStatus(proj.ID, storage.StatusReady, "", totalFiles, totalSymbols)
		report.TotalFiles = totalFiles
		report.TotalSymbols = totalSymbols
	} else {
		report.TotalFiles = proj.FileCount
		report.TotalSymbols = proj.SymbolCount
		_ = e.projectRepo.UpdateStatus(proj.ID, storage.StatusReady, "", proj.FileCount, proj.SymbolCount)
	}

	report.DurationMs = time.Since(startTime).Milliseconds()
	report.Message = fmt.Sprintf("Delta sincronizado via %s em %dms: %d parseados, %d ignorados, %d deletados, %d LLM hits",
		report.Strategy, report.DurationMs, report.FilesParsed, report.FilesSkipped, report.FilesDeleted, report.LLMCacheHits)

	log.Printf("[DELTA INDEXER SUCCESS] %s\n", report.Message)
	return report, nil
}

// ExtractSymbolsAndCallers realiza a extração de símbolos e chamadas via Tree-sitter (compatibilidade).
func (e *Engine) ExtractSymbolsAndCallers(
	projectID string,
	relPath string,
	content []byte,
	config LanguageConfig,
) ([]*storage.Symbol, []*storage.CallerInfo, error) {
	syms, refs, _, _, err := e.ExtractASTData(projectID, relPath, content, config)
	return syms, refs, err
}

// ExtractASTDigest extrai o resumo estrutural/assinaturas de um arquivo via Tree-sitter.
func (e *Engine) ExtractASTDigest(relPath string, content []byte, config LanguageConfig) string {
	if digestExtractor, ok := config.(ASTDigestExtractor); ok {
		lang := config.GetLanguage()
		parser := sitter.NewParser()
		parser.SetLanguage(lang)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		tree, err := parser.ParseCtx(ctx, nil, content)
		if err == nil && tree != nil && tree.RootNode() != nil {
			return digestExtractor.ExtractDigest(relPath, content, tree.RootNode())
		}
	}
	return ""
}

// ExtractASTData realiza a extração de símbolos, chamadas, dependências e modelos via Tree-sitter.
func (e *Engine) ExtractASTData(
	projectID string,
	relPath string,
	content []byte,
	config LanguageConfig,
) ([]*storage.Symbol, []*storage.CallerInfo, []*storage.DependencyEdge, []*storage.DataModel, error) {
	lang := config.GetLanguage()
	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil || tree == nil {
		return nil, nil, nil, nil, fmt.Errorf("falha ao analisar arquivo via Tree-sitter: %w", err)
	}
	defer tree.Close()

	rootNode := tree.RootNode()

	// --- 1. Extração de Símbolos ---
	symbolsQuery, err := sitter.NewQuery([]byte(config.SymbolsQuery()), lang)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("falha ao compilar SymbolsQuery: %w", err)
	}

	defer symbolsQuery.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(symbolsQuery, rootNode)

	var symbols []*storage.Symbol

	for {
		match, ok := cursor.NextMatch()
		if !ok || match == nil {
			break
		}

		var name string
		var kind storage.SymbolKind
		var defNode *sitter.Node

		for _, cap := range match.Captures {
			capName := symbolsQuery.CaptureNameForId(cap.Index)
			nodeText := cap.Node.Content(content)

			switch capName {
			case "function.name":
				name = nodeText
				kind = storage.KindFunction
			case "function.def":
				defNode = cap.Node
			case "method.name":
				name = nodeText
				kind = storage.KindMethod
			case "method.def":
				defNode = cap.Node
			case "struct.name":
				name = nodeText
				kind = storage.KindStruct
			case "struct.def":
				defNode = cap.Node
			case "interface.name":
				name = nodeText
				kind = storage.KindInterface
			case "interface.def":
				defNode = cap.Node
			case "class.name":
				name = nodeText
				kind = storage.KindClass
			case "class.def":
				defNode = cap.Node
			case "type.name":
				name = nodeText
				kind = storage.KindType
			case "type.def":
				defNode = cap.Node
			case "const.name":
				name = nodeText
				kind = storage.KindConstant
			case "const.def":
				defNode = cap.Node
			case "var.name", "variable.name":
				name = nodeText
				kind = storage.KindVariable
			case "var.def", "variable.def":
				defNode = cap.Node
			case "enum.name":
				name = nodeText
				kind = storage.KindEnum
			case "enum.def":
				defNode = cap.Node
			case "import.name":
				name = nodeText
				kind = storage.KindImport
			case "import.def":
				defNode = cap.Node
			}
		}

		if name != "" && kind != "" && defNode != nil {
			startPos := defNode.StartPoint()
			endPos := defNode.EndPoint()

			// Extrai a primeira linha como assinatura
			nodeText := defNode.Content(content)
			signature := strings.TrimSpace(strings.Split(nodeText, "\n")[0])

			symbols = append(symbols, &storage.Symbol{
				ProjectID: projectID,
				File:      relPath,
				Name:      name,
				Kind:      kind,
				Signature: signature,
				Language:  config.Name(),
				StartLine: int(startPos.Row) + 1, // 1-indexed
				EndLine:   int(endPos.Row) + 1,   // 1-indexed
				StartByte: int(defNode.StartByte()),
				EndByte:   int(defNode.EndByte()),
			})
		}
	}

	// Deduplica símbolos pelo par (file, name), priorizando o bloco mais completo
	symbols = deduplicateSymbols(symbols)

	// Ajusta métodos de Python que pertencem a classes
	if config.Name() == "python" {
		linkPythonClassMethods(symbols)
	}

	// --- 2. Extração de Referências/Callers ---
	var callers []*storage.CallerInfo
	callersQueryStr := config.CallersQuery()
	if callersQueryStr != "" {
		callersQuery, err := sitter.NewQuery([]byte(callersQueryStr), lang)
		if err == nil {
			defer callersQuery.Close()

			callCursor := sitter.NewQueryCursor()
			defer callCursor.Close()
			callCursor.Exec(callersQuery, rootNode)

			contentStr := string(content)
			contentLines := strings.Split(contentStr, "\n")

			for {
				match, ok := callCursor.NextMatch()
				if !ok || match == nil {
					break
				}

				for _, cap := range match.Captures {
					capName := callersQuery.CaptureNameForId(cap.Index)
					if capName == "callee" {
						calleeName := cap.Node.Content(content)
						if calleeName != "" {
							lineNum := int(cap.Node.StartPoint().Row) + 1
							var lineText string
							if lineNum-1 < len(contentLines) {
								lineText = strings.TrimSpace(contentLines[lineNum-1])
							}

							callers = append(callers, &storage.CallerInfo{
								ProjectID:  projectID,
								File:       relPath,
								Line:       lineNum,
								Text:       lineText,
								SymbolName: calleeName,
							})
						}
					}
				}
			}
		}
	}

	// --- 2b. Extração de Import-Sites (type references de classes/structs/interfaces) ---
	// Captura referências que não são call-sites: import statements, declarações de tipo,
	// type annotations, herança e implementação. Implementado opcionalmente por linguagem.
	if importer, ok := config.(ImportSiteExtractor); ok {
		importSites := importer.ExtractImportSites(projectID, relPath, content, rootNode)
		callers = append(callers, importSites...)
	}

	// --- 3. Extração de Dependências e Injeção (DI) ---
	var dependencies []*storage.DependencyEdge
	if extractor, ok := config.(DependencyExtractor); ok {
		dependencies = extractor.ExtractDependencies(projectID, relPath, content, rootNode)
	}

	// --- 4. Extração de Modelos de Dados e Schemas ---
	var dataModels []*storage.DataModel
	if extractor, ok := config.(DataModelExtractor); ok {
		dataModels = extractor.ExtractDataModels(projectID, relPath, content, rootNode)
	}

	return symbols, callers, dependencies, dataModels, nil
}

// deduplicateSymbols remove duplicatas no mesmo arquivo mantendo a definição mais ampla.
func deduplicateSymbols(symbols []*storage.Symbol) []*storage.Symbol {
	type key struct {
		file string
		name string
	}

	best := make(map[key]*storage.Symbol)
	for _, sym := range symbols {
		k := key{file: sym.File, name: sym.Name}
		existing, ok := best[k]
		if !ok {
			best[k] = sym
			continue
		}

		// Compara tamanho do byte range
		currSize := sym.EndByte - sym.StartByte
		prevSize := existing.EndByte - existing.StartByte
		if currSize > prevSize {
			best[k] = sym
		}
	}

	result := make([]*storage.Symbol, 0, len(best))
	for _, sym := range symbols {
		k := key{file: sym.File, name: sym.Name}
		if best[k] == sym {
			result = append(result, sym)
			delete(best, k) // evita duplicatas se houvesse múltiplos iguais
		}
	}

	return result
}

// linkPythonClassMethods associa métodos às classes pai no Python com base no range de bytes.
func linkPythonClassMethods(symbols []*storage.Symbol) {
	type classRange struct {
		name      string
		startByte int
		endByte   int
	}

	var classes []classRange
	for _, s := range symbols {
		if s.Kind == storage.KindClass {
			classes = append(classes, classRange{
				name:      s.Name,
				startByte: s.StartByte,
				endByte:   s.EndByte,
			})
		}
	}

	for _, s := range symbols {
		if s.Kind == storage.KindFunction && s.Parent == "" {
			for _, cls := range classes {
				if s.StartByte >= cls.startByte && s.EndByte <= cls.endByte {
					s.Kind = storage.KindMethod
					s.Parent = cls.name
					break
				}
			}
		}
	}
}

// GetImplementation recupera o trecho exato de código correspondente ao símbolo.
func (e *Engine) GetImplementation(project *storage.Project, filePath, symbolName string) (string, error) {
	sym, err := e.symbolRepo.GetSymbolByFileAndName(project.ID, filePath, symbolName)
	if err != nil {
		return "", err
	}

	absPath := filepath.Join(project.Path, filePath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("falha ao ler arquivo '%s': %w", filePath, err)
	}

	if sym == nil {
		// Se não encontrou no banco por arquivo exato, tenta buscar apenas pelo nome no banco
		symbols, _, err := e.symbolRepo.FindSymbol(project.ID, symbolName, 1, 0)
		if err == nil && len(symbols) > 0 {
			sym = symbols[0]
			absPath = filepath.Join(project.Path, sym.File)
			content, err = os.ReadFile(absPath)
			if err != nil {
				return "", fmt.Errorf("falha ao ler arquivo '%s': %w", sym.File, err)
			}
		}
	}

	if sym == nil {
		return "", fmt.Errorf("símbolo '%s' não encontrado no projeto '%s'", symbolName, project.Name)
	}

	if sym.StartByte >= len(content) {
		return "", fmt.Errorf("posição do símbolo fora dos limites do arquivo")
	}

	endByte := sym.EndByte
	if endByte > len(content) {
		endByte = len(content)
	}

	return fmt.Sprintf("[%s:%d-%d]\n%s", sym.File, sym.StartLine, sym.EndLine, string(content[sym.StartByte:endByte])), nil
}

// PeekFile lê um intervalo cirúrgico de linhas com contexto AST, smart anchor e header enriquecido.
func (e *Engine) PeekFile(project *storage.Project, filePath string, startLine, endLine int, anchorSymbol string) (string, error) {
	absPath := filepath.Join(project.Path, filePath)

	// Smart Anchor: posiciona janela automaticamente no símbolo
	anchorWarning := ""
	if anchorSymbol != "" {
		sym, err := e.symbolRepo.GetSymbolByFileAndName(project.ID, filePath, anchorSymbol)
		if err == nil && sym != nil {
			// Centraliza a janela no símbolo
			symLines := sym.EndLine - sym.StartLine + 1
			if symLines <= 50 {
				// Símbolo cabe na janela — centraliza
				padding := (50 - symLines) / 2
				startLine = sym.StartLine - padding
				if startLine < 1 {
					startLine = 1
				}
				endLine = startLine + 49
			} else {
				// Símbolo maior que a janela — começa do início
				startLine = sym.StartLine
				endLine = startLine + 99 // usa janela máxima
			}
		} else {
			// Fallback: tenta buscar pelo nome no projeto inteiro
			symbols, _, findErr := e.symbolRepo.FindSymbol(project.ID, anchorSymbol, 1, 0)
			if findErr == nil && len(symbols) > 0 && symbols[0].File == filePath {
				sym = symbols[0]
				startLine = sym.StartLine
				endLine = startLine + 49
			} else {
				// Fallback final: mostra início do arquivo + aviso
				startLine = 1
				endLine = 50
				anchorWarning = fmt.Sprintf("⚠ Symbol '%s' not found in index. Showing file start.\n", anchorSymbol)
			}
		}
	}

	file, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("falha ao abrir arquivo '%s': %w", filePath, err)
	}
	defer file.Close()

	if startLine <= 0 {
		startLine = 1
	}
	if endLine <= 0 || endLine < startLine {
		endLine = startLine + 49
	}
	// Limite de janela máxima de 100 linhas
	if endLine-startLine+1 > 100 {
		endLine = startLine + 99
	}

	var totalLines int
	var selectedLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		totalLines++
		if totalLines >= startLine && totalLines <= endLine {
			selectedLines = append(selectedLines, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("falha ao ler linhas de '%s': %w", filePath, err)
	}

	if totalLines == 0 {
		return fmt.Sprintf("[%s (empty file)]\n", filePath), nil
	}

	actualStart := startLine
	if actualStart > totalLines {
		actualStart = totalLines
	}
	actualEnd := endLine
	if actualEnd > totalLines {
		actualEnd = totalLines
	}

	// Header enriquecido com linguagem e indicador de janela
	lang := detectLanguageByExtension(filePath)
	windowUsed := actualEnd - actualStart + 1
	if windowUsed > len(selectedLines) {
		windowUsed = len(selectedLines)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "[%s (lines %d-%d of %d) | %s | window: %d/100]\n", filePath, actualStart, actualEnd, totalLines, lang, windowUsed)

	// Aviso de anchor não encontrado (se houver)
	if anchorWarning != "" {
		sb.WriteString(anchorWarning)
	}

	// Symbol Context Overlay — consulta símbolos no range (silencioso se não houver)
	overlaySymbols, _ := e.symbolRepo.GetSymbolsByFileAndLineRange(project.ID, filePath, actualStart, actualEnd)
	if len(overlaySymbols) > 0 {
		sb.WriteString("⊕ Symbols in range:\n")
		for _, sym := range overlaySymbols {
			relation := "active"
			if sym.StartLine >= actualStart && sym.EndLine <= actualEnd {
				relation = "exact match"
			} else if sym.StartLine < actualStart {
				relation = "parent scope"
			}
			fmt.Fprintf(&sb, "  ƒ %s (%s, lines %d-%d) — %s\n", sym.Name, sym.Kind, sym.StartLine, sym.EndLine, relation)
		}
		sb.WriteString("\n")
	}

	for i, line := range selectedLines {
		lineNum := actualStart + i
		fmt.Fprintf(&sb, "%d: %s\n", lineNum, line)
	}

	return sb.String(), nil
}

// binaryExtensions contém extensões de arquivos binários que devem ser ignorados na busca textual.
var binaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".ico": true, ".webp": true, ".svg": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true,
	".db": true, ".sqlite": true, ".sqlite3": true,
	".wasm": true, ".class": true, ".pyc": true, ".pyo": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true, ".7z": true, ".rar": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
	".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".mkv": true, ".flac": true, ".wav": true, ".ogg": true,
	".o": true, ".obj": true, ".bin": true, ".dat": true,
	".lock": true, ".sum": true,
	".min.js": true, ".min.css": true,
}

// errStopWalk é o sentinel error usado para interromper o filepath.Walk quando o limite de matches é atingido.
var errStopWalk = fmt.Errorf("grep: stop walk limit reached")

// isBinaryExtension verifica se o arquivo tem extensão binária conhecida.
func isBinaryExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if binaryExtensions[ext] {
		return true
	}
	// Verificação para extensões compostas como .min.js
	base := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(base, ".min.js") || strings.HasSuffix(base, ".min.css") {
		return true
	}
	return false
}

// isBinaryContent verifica se os primeiros bytes do arquivo contêm bytes nulos (indicativo de binário).
func isBinaryContent(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

// GrepOptions encapsula todos os parâmetros opcionais para busca textual no projeto.
type GrepOptions struct {
	CaseSensitive bool
	IncludeExts   []string
	ContextLines  int
}

// grepFileResult encapsula os matches encontrados em um único arquivo (usado pelo worker pool).
type grepFileResult struct {
	Matches []storage.GrepMatch
}

// GrepProject realiza busca textual por expressão regular em arquivos do projeto com paginação,
// respeitando .gitignore, filtrando binários e usando worker pool paralelo para I/O.
func (e *Engine) GrepProject(ctx context.Context, project *storage.Project, pattern, pathPrefix string, limit, offset int, opts GrepOptions) ([]storage.GrepMatch, bool, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	if opts.ContextLines < 0 {
		opts.ContextLines = 0
	}
	if opts.ContextLines > 5 {
		opts.ContextLines = 5
	}

	// Compila a regex respeitando case-sensitivity
	regexPattern := pattern
	if !opts.CaseSensitive {
		regexPattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, false, fmt.Errorf("expressão regular inválida: %w", err)
	}

	searchRoot := project.Path
	if pathPrefix != "" {
		searchRoot = filepath.Join(project.Path, pathPrefix)
	}

	// Prepara o set de extensões permitidas para filtro rápido
	var extFilter map[string]bool
	if len(opts.IncludeExts) > 0 {
		extFilter = make(map[string]bool, len(opts.IncludeExts))
		for _, ext := range opts.IncludeExts {
			ext = strings.TrimSpace(ext)
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			extFilter[strings.ToLower(ext)] = true
		}
	}

	// Carrega .gitignore da raiz do projeto
	var gitIgnore *ignore.GitIgnore
	gitignorePath := filepath.Join(project.Path, ".gitignore")
	if data, gitErr := os.ReadFile(gitignorePath); gitErr == nil {
		gitIgnore = ignore.CompileIgnoreLines(strings.Split(string(data), "\n")...)
	}

	// ──────────────────────────────────────────────────
	// FASE 1: Walk rápido — coleta apenas paths elegíveis
	// ──────────────────────────────────────────────────
	const maxPaths = 5000
	paths := make([]string, 0, 256)
	relPaths := make([]string, 0, 256)

	_ = filepath.Walk(searchRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		// Checagem de cancelamento
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		relPath, _ := filepath.Rel(project.Path, path)

		if info.IsDir() {
			name := info.Name()
			// Usa DefaultIgnoredDirs (21+ dirs) + hidden dirs
			if DefaultIgnoredDirs[name] || (strings.HasPrefix(name, ".") && name != ".") {
				return filepath.SkipDir
			}
			// Respeita .gitignore para diretórios
			if gitIgnore != nil && gitIgnore.MatchesPath(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		// Respeita .gitignore para arquivos
		if gitIgnore != nil && gitIgnore.MatchesPath(relPath) {
			return nil
		}

		// Skip de arquivos muito grandes (> 2MB) ou vazios
		if info.Size() > 2*1024*1024 || info.Size() == 0 {
			return nil
		}

		// Skip de extensões binárias conhecidas
		if isBinaryExtension(path) {
			return nil
		}

		// Filtro por extensão (se fornecido)
		if extFilter != nil {
			ext := strings.ToLower(filepath.Ext(path))
			if !extFilter[ext] {
				return nil
			}
		}

		paths = append(paths, path)
		relPaths = append(relPaths, relPath)

		if len(paths) >= maxPaths {
			return errStopWalk
		}
		return nil
	})

	if len(paths) == 0 {
		return []storage.GrepMatch{}, false, nil
	}

	// ──────────────────────────────────────────────────
	// FASE 2: Grep paralelo com worker pool
	// ──────────────────────────────────────────────────
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8
	}
	if numWorkers > len(paths) {
		numWorkers = len(paths)
	}

	type indexedPath struct {
		idx     int
		absPath string
		relPath string
	}

	pathCh := make(chan indexedPath, len(paths))
	resultCh := make(chan grepFileResult, numWorkers)

	// Alimenta o channel de paths
	for i, p := range paths {
		pathCh <- indexedPath{idx: i, absPath: p, relPath: relPaths[i]}
	}
	close(pathCh)

	// Limite global de matches para evitar explosão de memória (atomic via mutex)
	const maxGlobalMatches = 1000
	var (
		mu            sync.Mutex
		globalCount   int
		stopCollected bool
	)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var localMatches []storage.GrepMatch

			for ip := range pathCh {
				// Checagem de cancelamento
				if ctx.Err() != nil {
					break
				}

				// Verifica se já atingiu o limite global
				mu.Lock()
				if stopCollected {
					mu.Unlock()
					break
				}
				mu.Unlock()

				fileMatches := e.grepSingleFile(ip.absPath, ip.relPath, re, opts.ContextLines)
				if len(fileMatches) > 0 {
					localMatches = append(localMatches, fileMatches...)

					mu.Lock()
					globalCount += len(fileMatches)
					if globalCount >= maxGlobalMatches {
						stopCollected = true
					}
					mu.Unlock()
				}
			}

			if len(localMatches) > 0 {
				resultCh <- grepFileResult{Matches: localMatches}
			}
		}()
	}

	// Espera todos os workers terminarem e fecha o channel de resultados
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Coleta todos os resultados
	var allMatches []storage.GrepMatch
	for result := range resultCh {
		allMatches = append(allMatches, result.Matches...)
	}

	// Ordena as correspondências pelo score de centralidade do arquivo (arquivos de negócio antes de testes/mocks)
	fileImports, _ := e.symbolRepo.GetFileImportCounts(project.ID)
	fileScores := make(map[string]float64)
	getFileScore := func(f string) float64 {
		if s, ok := fileScores[f]; ok {
			return s
		}
		s := CalculateSymbolScore(f, "", fileImports[f], 0)
		fileScores[f] = s
		return s
	}

	sort.SliceStable(allMatches, func(i, j int) bool {
		sI := getFileScore(allMatches[i].File)
		sJ := getFileScore(allMatches[j].File)
		if sI != sJ {
			return sI > sJ
		}
		if allMatches[i].File != allMatches[j].File {
			return allMatches[i].File < allMatches[j].File
		}
		return allMatches[i].Line < allMatches[j].Line
	})

	total := len(allMatches)
	if offset >= total {
		return []storage.GrepMatch{}, false, nil
	}

	end := offset + limit
	hasMore := false
	if end < total {
		hasMore = true
	} else {
		end = total
	}

	return allMatches[offset:end], hasMore, nil
}

// grepSingleFile realiza busca textual em um único arquivo com suporte a context lines.
func (e *Engine) grepSingleFile(absPath, relPath string, re *regexp.Regexp, contextLines int) []storage.GrepMatch {
	f, err := os.Open(absPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	// Detecção heurística de binário: lê os primeiros 512 bytes
	probe := make([]byte, 512)
	n, _ := f.Read(probe)
	if n > 0 && isBinaryContent(probe[:n]) {
		return nil
	}
	// Reposiciona para o início do arquivo
	if _, err := f.Seek(0, 0); err != nil {
		return nil
	}

	scanner := bufio.NewScanner(f)
	// Aumenta o buffer do scanner para linhas longas (1MB)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var matches []storage.GrepMatch

	if contextLines <= 0 {
		// Modo rápido: sem contexto, apenas scan direto
		lineNum := 1
		for scanner.Scan() {
			line := scanner.Text()
			if re.MatchString(line) {
				matches = append(matches, storage.GrepMatch{
					File:    relPath,
					Line:    lineNum,
					Content: strings.TrimSpace(line),
				})
			}
			lineNum++
		}
		_ = scanner.Err()
	} else {
		// Modo com contexto: usa buffer circular para linhas anteriores
		beforeBuf := make([]string, contextLines)
		beforeIdx := 0
		beforeCount := 0

		// Primeiro, lê todas as linhas para poder capturar ContextAfter
		var allLines []string
		for scanner.Scan() {
			allLines = append(allLines, scanner.Text())
		}
		_ = scanner.Err()

		for lineIdx, line := range allLines {
			lineNum := lineIdx + 1

			if re.MatchString(line) {
				// Captura linhas antes
				var ctxBefore []string
				available := beforeCount
				if available > contextLines {
					available = contextLines
				}
				for i := available; i > 0; i-- {
					idx := (beforeIdx - i + contextLines) % contextLines
					ctxBefore = append(ctxBefore, strings.TrimSpace(beforeBuf[idx]))
				}

				// Captura linhas depois
				var ctxAfter []string
				for i := 1; i <= contextLines && lineIdx+i < len(allLines); i++ {
					ctxAfter = append(ctxAfter, strings.TrimSpace(allLines[lineIdx+i]))
				}

				matches = append(matches, storage.GrepMatch{
					File:          relPath,
					Line:          lineNum,
					Content:       strings.TrimSpace(line),
					ContextBefore: ctxBefore,
					ContextAfter:  ctxAfter,
				})
			}

			// Atualiza buffer circular
			beforeBuf[beforeIdx] = line
			beforeIdx = (beforeIdx + 1) % contextLines
			beforeCount++
		}
	}

	return matches
}
