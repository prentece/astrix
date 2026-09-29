package service

import (
	"astrix/pkg/indexer"
	"astrix/pkg/storage"
	"context"
	"errors"
	"path/filepath"
)

// CodeService gerencia operações de inteligência de código, AST, busca semântica e leitura cirúrgica.
type CodeService struct {
	projectRepo    storage.ProjectRepository
	symbolRepo     storage.SymbolRepository
	depRepo        storage.DependencyGraphRepository
	dataModelRepo  storage.DataModelRepository
	engine         *indexer.Engine
}

// NewCodeService cria uma nova instância de CodeService.
func NewCodeService(
	projectRepo storage.ProjectRepository,
	symbolRepo storage.SymbolRepository,
	depRepo storage.DependencyGraphRepository,
	dataModelRepo storage.DataModelRepository,
	engine *indexer.Engine,
) *CodeService {
	return &CodeService{
		projectRepo:   projectRepo,
		symbolRepo:    symbolRepo,
		depRepo:       depRepo,
		dataModelRepo: dataModelRepo,
		engine:        engine,
	}
}

// FindSymbol busca símbolos na tabela AST com correspondência exata ou parcial e suporte a paginação.
func (s *CodeService) FindSymbol(projectID, symbolName string, limit, offset int) ([]*storage.Symbol, bool, error) {
	if projectID == "" {
		return nil, false, errors.New("project_id é obrigatório")
	}
	if symbolName == "" {
		return nil, false, errors.New("symbol_name é obrigatório")
	}
	if limit <= 0 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	return s.symbolRepo.FindSymbol(projectID, symbolName, limit, offset)
}

// GetImplementation extrai a implementação AST cirúrgica de um símbolo.
func (s *CodeService) GetImplementation(projectID, filePath, symbolName string) (string, error) {
	if projectID == "" {
		return "", errors.New("project_id é obrigatório")
	}
	if filePath == "" {
		return "", errors.New("filepath é obrigatório")
	}
	if symbolName == "" {
		return "", errors.New("symbol_name é obrigatório")
	}

	proj, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return "", err
	}
	if proj == nil {
		return "", errors.New("projeto não encontrado")
	}

	// Normaliza para caminho relativo dentro do projeto
	relPath := filePath
	if filepath.IsAbs(filePath) {
		if r, err := filepath.Rel(proj.Path, filePath); err == nil {
			relPath = r
		}
	}

	return s.engine.GetImplementation(proj, relPath, symbolName)
}

// BundleRequest representa uma entrada de símbolo para consulta em lote.
type BundleRequest struct {
	Filepath   string `json:"filepath"`
	SymbolName string `json:"symbol_name"`
}

// BundleResult representa a saída de uma entrada de lote, com erro parcial tolerado.
type BundleResult struct {
	Filepath   string `json:"filepath"`
	SymbolName string `json:"symbol_name"`
	Code       string `json:"code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// GetImplementationBundle extrai a implementação AST de múltiplos símbolos em uma única chamada.
// Erros parciais são tolerados: símbolos não encontrados retornam o campo Error preenchido.
func (s *CodeService) GetImplementationBundle(projectID string, symbols []BundleRequest) ([]BundleResult, error) {
	if projectID == "" {
		return nil, errors.New("project_id é obrigatório")
	}
	if len(symbols) == 0 {
		return nil, errors.New("symbols não pode ser vazio")
	}

	proj, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, err
	}
	if proj == nil {
		return nil, errors.New("projeto não encontrado")
	}

	results := make([]BundleResult, 0, len(symbols))
	for _, req := range symbols {
		res := BundleResult{
			Filepath:   req.Filepath,
			SymbolName: req.SymbolName,
		}
		if req.Filepath == "" || req.SymbolName == "" {
			res.Error = "filepath e symbol_name são obrigatórios"
			results = append(results, res)
			continue
		}

		relPath := req.Filepath
		if filepath.IsAbs(req.Filepath) {
			if r, err := filepath.Rel(proj.Path, req.Filepath); err == nil {
				relPath = r
			}
		}

		code, err := s.engine.GetImplementation(proj, relPath, req.SymbolName)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Code = code
		}
		results = append(results, res)
	}
	return results, nil
}

// FindReferences busca referências e chamadores de um símbolo no projeto com paginação.
func (s *CodeService) FindReferences(projectID, symbolName string, limit, offset int) ([]*storage.CallerInfo, bool, error) {
	if projectID == "" {
		return nil, false, errors.New("project_id é obrigatório")
	}
	if symbolName == "" {
		return nil, false, errors.New("symbol_name é obrigatório")
	}
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	return s.symbolRepo.FindReferences(projectID, symbolName, limit, offset)
}

// PeekFile lê um intervalo cirúrgico de linhas de um arquivo do projeto com contexto AST.
func (s *CodeService) PeekFile(projectID, filePath string, startLine, endLine int, anchorSymbol string) (string, error) {
	if projectID == "" {
		return "", errors.New("project_id é obrigatório")
	}
	if filePath == "" {
		return "", errors.New("filepath é obrigatório")
	}

	proj, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return "", err
	}
	if proj == nil {
		return "", errors.New("projeto não encontrado")
	}

	relPath := filePath
	if filepath.IsAbs(filePath) {
		if r, err := filepath.Rel(proj.Path, filePath); err == nil {
			relPath = r
		}
	}

	return s.engine.PeekFile(proj, relPath, startLine, endLine, anchorSymbol)
}

// GrepCode realiza busca textual com regex respeitando .gitignore com paginação.
func (s *CodeService) GrepCode(ctx context.Context, projectID, pattern, pathPrefix string, limit, offset int, opts indexer.GrepOptions) ([]storage.GrepMatch, bool, error) {
	if projectID == "" {
		return nil, false, errors.New("project_id é obrigatório")
	}
	if pattern == "" {
		return nil, false, errors.New("pattern é obrigatório")
	}
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	proj, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, false, err
	}
	if proj == nil {
		return nil, false, errors.New("projeto não encontrado")
	}

	return s.engine.GrepProject(ctx, proj, pattern, pathPrefix, limit, offset, opts)
}

// GetProjectStructure constrói a árvore de diretórios do projeto em formato estruturado.
func (s *CodeService) GetProjectStructure(projectID, subPath string) (*storage.FileNode, error) {
	if projectID == "" {
		return nil, errors.New("project_id é obrigatório")
	}

	proj, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, err
	}
	if proj == nil {
		return nil, errors.New("projeto não encontrado")
	}

	return indexer.BuildDirectoryTree(proj.Path, subPath, 3)
}

// GetProjectStructureTree constrói a árvore textual compacta (estilo tree) com profundidade e filtros.
func (s *CodeService) GetProjectStructureTree(projectID, subPath string, depth int, showHidden bool) (string, error) {
	if projectID == "" {
		return "", errors.New("project_id é obrigatório")
	}
	if depth <= 0 {
		depth = 2
	}

	proj, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return "", err
	}
	if proj == nil {
		return "", errors.New("projeto não encontrado")
	}

	// Lazy delta sync síncrono para garantir consistência da árvore e índices
	if s.engine != nil && proj.Status == storage.StatusReady {
		_, _ = s.engine.ProcessIncrementalDelta(projectID)
	}

	return indexer.BuildTreeText(proj.Path, subPath, depth, showHidden)
}

// QueryStructuredFile consulta chaves/nós pontuais em JSON, YAML ou CSV de forma eficiente.
func (s *CodeService) QueryStructuredFile(projectID, filePath, query string) (string, error) {
	if projectID == "" {
		return "", errors.New("project_id é obrigatório")
	}
	if filePath == "" {
		return "", errors.New("filepath é obrigatório")
	}
	if query == "" {
		return "", errors.New("query é obrigatório")
	}

	proj, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return "", err
	}
	if proj == nil {
		return "", errors.New("projeto não encontrado")
	}

	relPath := filePath
	if filepath.IsAbs(filePath) {
		if r, err := filepath.Rel(proj.Path, filePath); err == nil {
			relPath = r
		}
	}

	return indexer.QueryStructuredFile(proj.Path, relPath, query)
}


// ArchitectureGraphNode representa um nó na árvore do grafo de dependências arquiteturais.
type ArchitectureGraphNode struct {
	Symbol       string                   `json:"symbol"`
	File         string                   `json:"file,omitempty"`
	Line         int                      `json:"line,omitempty"`
	RelType      string                   `json:"rel_type,omitempty"` // 'injects', 'uses', 'instantiates'
	IsCycle      bool                     `json:"is_cycle,omitempty"`
	Dependencies []*ArchitectureGraphNode `json:"dependencies,omitempty"`
}

// GetArchitectureGraph extrai a árvore de dependências e injeções de um símbolo com controle de profundidade e direção.
func (s *CodeService) GetArchitectureGraph(projectID, symbol, direction string, depth int) (*ArchitectureGraphNode, error) {
	if projectID == "" {
		return nil, errors.New("project_id é obrigatório")
	}
	if symbol == "" {
		return nil, errors.New("symbol_name é obrigatório")
	}
	if s.depRepo == nil {
		return nil, errors.New("dependency repository não inicializado")
	}
	if depth <= 0 {
		depth = 2
	}
	if depth > 5 {
		depth = 5
	}
	if direction != "upstream" {
		direction = "downstream"
	}

	// Localiza arquivo e linha do símbolo raiz se existir
	rootFile := ""
	rootLine := 0
	if syms, _, err := s.symbolRepo.FindSymbol(projectID, symbol, 1, 0); err == nil && len(syms) > 0 {
		rootFile = syms[0].File
		rootLine = syms[0].StartLine
	}

	rootNode := &ArchitectureGraphNode{
		Symbol: symbol,
		File:   rootFile,
		Line:   rootLine,
	}

	visitedPath := map[string]bool{symbol: true}
	s.buildGraphRecursive(projectID, rootNode, direction, depth, 0, visitedPath)

	return rootNode, nil
}

func (s *CodeService) buildGraphRecursive(projectID string, current *ArchitectureGraphNode, direction string, maxDepth, currentDepth int, visitedPath map[string]bool) {
	if currentDepth >= maxDepth {
		return
	}

	var edges []*storage.DependencyEdge
	var err error

	if direction == "downstream" {
		edges, err = s.depRepo.GetDownstreamEdges(projectID, current.Symbol)
	} else {
		edges, err = s.depRepo.GetUpstreamEdges(projectID, current.Symbol)
	}

	if err != nil || len(edges) == 0 {
		return
	}

	for _, edge := range edges {
		childSym := edge.TargetSymbol
		childFile := edge.TargetFile
		if direction == "upstream" {
			childSym = edge.SourceSymbol
			childFile = edge.SourceFile
		}

		childLine := 0
		if childFile != "" {
			if syms, _, err := s.symbolRepo.FindSymbol(projectID, childSym, 1, 0); err == nil && len(syms) > 0 {
				childLine = syms[0].StartLine
				if childFile == "" {
					childFile = syms[0].File
				}
			}
		}

		isCycle := visitedPath[childSym]

		childNode := &ArchitectureGraphNode{
			Symbol:  childSym,
			File:    childFile,
			Line:    childLine,
			RelType: edge.RelationshipType,
			IsCycle: isCycle,
		}

		current.Dependencies = append(current.Dependencies, childNode)

		if !isCycle {
			visitedPath[childSym] = true
			s.buildGraphRecursive(projectID, childNode, direction, maxDepth, currentDepth+1, visitedPath)
			delete(visitedPath, childSym)
		}
	}
}

// GetDataModel busca a definição de schema ou DTO pré-processada pelo nome.
func (s *CodeService) GetDataModel(projectID, modelName string) (*storage.DataModel, error) {
	if projectID == "" {
		return nil, errors.New("project_id é obrigatório")
	}
	if modelName == "" {
		return nil, errors.New("model_name é obrigatório")
	}
	if s.dataModelRepo == nil {
		return nil, errors.New("data_model repository não inicializado")
	}

	return s.dataModelRepo.GetByName(projectID, modelName)
}

// ListDataModels lista todos os modelos e schemas indexados no projeto.
func (s *CodeService) ListDataModels(projectID string) ([]*storage.DataModel, error) {
	if projectID == "" {
		return nil, errors.New("project_id é obrigatório")
	}
	if s.dataModelRepo == nil {
		return nil, errors.New("data_model repository não inicializado")
	}

	return s.dataModelRepo.ListByProject(projectID)
}


// GetSymbolStats retorna estatísticas de mapeamento de símbolos por arquivo para a tela de mapeamento.
func (s *CodeService) GetSymbolStats(projectID string) ([]*storage.FileSymbolStats, error) {
	if projectID == "" {
		return nil, errors.New("project_id é obrigatório")
	}

	proj, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		return nil, err
	}
	if proj == nil {
		return nil, errors.New("projeto não encontrado")
	}

	return s.symbolRepo.GetSymbolCountsByFile(projectID)
}

// GetSymbolsByFile retorna todos os símbolos AST associados a um arquivo específico.
func (s *CodeService) GetSymbolsByFile(projectID, file string) ([]*storage.Symbol, error) {
	if projectID == "" {
		return nil, errors.New("project_id é obrigatório")
	}
	if file == "" {
		return nil, errors.New("file é obrigatório")
	}
	return s.symbolRepo.GetSymbolsByFileAndLineRange(projectID, file, 0, 1000000)
}
