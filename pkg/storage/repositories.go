package storage

// SymbolRepository abstrai a persistência e consulta sintática de símbolos e referências AST.
type SymbolRepository interface {
	ClearProjectData(projectID string) error
	DeleteByFile(projectID, file string) error
	SaveSymbols(projectID string, symbols []*Symbol) error
	SaveReferences(projectID string, references []*CallerInfo) error
	FindSymbol(projectID, name string, limit, offset int) ([]*Symbol, bool, error)
	GetSymbolByFileAndName(projectID, file, symbolName string) (*Symbol, error)
	FindReferences(projectID, symbolName string, limit, offset int) ([]*CallerInfo, bool, error)
	UpdateRelevanceScores(projectID string, symbolScores map[int64]float64) error
	GetFileImportCounts(projectID string) (map[string]int, error)
	GetSymbolReferenceCounts(projectID string) (map[string]int, error)
	GetAllSymbolsForRanking(projectID string) ([]*Symbol, error)
	GetSymbolsByFileAndLineRange(projectID, file string, startLine, endLine int) ([]*Symbol, error)
	GetSymbolCountsByFile(projectID string) ([]*FileSymbolStats, error)
}

// ProjectRepository abstrai as operações de persistência e ciclo de vida de projetos.
type ProjectRepository interface {
	Create(project *Project) error
	GetByID(id string) (*Project, error)
	ListAll() ([]*Project, error)
	UpdateStatus(id string, status ProjectStatus, errMsg string, fileCount, symbolCount int) error
	SetAutoSync(id string, autoSync bool) error
	ResetDanglingIndexingStatus() error
	Delete(id string) error
}


// DependencyGraphRepository abstrai a persistência e consulta do grafo de dependências arquiteturais.
type DependencyGraphRepository interface {
	SaveDependencies(projectID string, edges []*DependencyEdge) error
	GetDownstreamEdges(projectID, symbol string) ([]*DependencyEdge, error)
	GetUpstreamEdges(projectID, symbol string) ([]*DependencyEdge, error)
	GetAllEdges(projectID string) ([]*DependencyEdge, error)
	UpdateTargetFiles(projectID string, symbolToFileMap map[string]string) error
	ResolveTargetFiles(projectID string, symbols []*Symbol) error
	DeleteByFile(projectID, file string) error
	ClearProjectDependencies(projectID string) error
}

// DataModelRepository abstrai a persistência e consulta cirúrgica de Schemas, DTOs e Entidades.
type DataModelRepository interface {
	SaveDataModels(projectID string, models []*DataModel) error
	GetDataModel(projectID, modelName string) (*DataModel, error)
	GetByName(projectID, modelName string) (*DataModel, error)
	ListDataModels(projectID string, limit, offset int) ([]*DataModel, bool, error)
	ListByProject(projectID string) ([]*DataModel, error)
	DeleteByFile(projectID, file string) error
	ClearProjectDataModels(projectID string) error
}


// FileStateRepository abstrai o armazenamento de estado e hashes de arquivos para indexação delta.
type FileStateRepository interface {
	Get(projectID, filepath string) (*ProjectFileState, error)
	ListByProject(projectID string) (map[string]*ProjectFileState, error)
	Upsert(state *ProjectFileState) error
	UpsertBatch(states []*ProjectFileState) error
	Delete(projectID, filepath string) error
	DeleteByProject(projectID string) error
}
