package storage

import "time"

// ProjectStatus representa os possíveis estados de indexação de um projeto.
type ProjectStatus string

const (
	StatusPending  ProjectStatus = "pending"
	StatusIndexing ProjectStatus = "indexing"
	StatusReady    ProjectStatus = "ready"
	StatusError    ProjectStatus = "error"
)

// Project representa um repositório legado registrado para análise e indexação.
type Project struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Path         string        `json:"path"`
	Language     string        `json:"language"`
	Status       ProjectStatus `json:"status"`
	ErrorMessage string        `json:"error_message,omitempty"`
	FileCount    int           `json:"file_count"`
	SymbolCount  int           `json:"symbol_count"`
	AutoSync     bool          `json:"auto_sync"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	IndexedAt    *time.Time    `json:"indexed_at,omitempty"`
}

// SymbolKind define a categoria semântica do símbolo extraído via AST.
type SymbolKind string

const (
	KindFunction  SymbolKind = "function"
	KindMethod    SymbolKind = "method"
	KindClass     SymbolKind = "class"
	KindStruct    SymbolKind = "struct"
	KindInterface SymbolKind = "interface"
	KindType      SymbolKind = "type"
	KindConstant  SymbolKind = "constant"
	KindVariable  SymbolKind = "variable"
	KindEnum      SymbolKind = "enum"
	KindTrait     SymbolKind = "trait"
	KindImport    SymbolKind = "import"
	KindTest      SymbolKind = "test"
	KindOther     SymbolKind = "other"
)

// Symbol representa uma entidade de código indexada (função, classe, struct, etc.).
type Symbol struct {
	ID             int64      `json:"id"`
	ProjectID      string     `json:"project_id"`
	File           string     `json:"file"` // Caminho relativo dentro do repositório
	Name           string     `json:"name"`
	Kind           SymbolKind `json:"kind"`
	Signature      string     `json:"signature"`
	Parent         string     `json:"parent,omitempty"` // Ex: nome da struct ou classe pai
	Language       string     `json:"language"`
	StartLine      int        `json:"start_line"` // 1-indexed
	EndLine        int        `json:"end_line"`   // 1-indexed
	StartByte      int        `json:"start_byte"`
	EndByte        int        `json:"end_byte"`
	RelevanceScore float64    `json:"relevance_score"`
}

// CallerInfo representa uma referência ou chamada de um símbolo no código.
type CallerInfo struct {
	ID         int64  `json:"id,omitempty"`
	ProjectID  string `json:"project_id,omitempty"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Text       string `json:"text"`
	SymbolName string `json:"symbol_name,omitempty"`
}

// FileNode representa um nó na árvore de diretórios de um projeto.
type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"is_dir"`
	Size     int64       `json:"size,omitempty"`
	Children []*FileNode `json:"children,omitempty"`
}

// GrepMatch representa uma ocorrência encontrada na busca textual.
type GrepMatch struct {
	File          string   `json:"file"`
	Line          int      `json:"line"`
	Content       string   `json:"content"`
	ContextBefore []string `json:"context_before,omitempty"`
	ContextAfter  []string `json:"context_after,omitempty"`
}


// DependencyEdge representa um arco no grafo de dependências e injeção do projeto.
type DependencyEdge struct {
	ID               int64     `json:"id"`
	ProjectID        string    `json:"project_id"`
	SourceSymbol     string    `json:"source_symbol"`
	TargetSymbol     string    `json:"target_symbol"`
	SourceFile       string    `json:"source_file"`
	TargetFile       string    `json:"target_file"`
	RelationshipType string    `json:"relationship_type"` // 'injects', 'uses', 'instantiates'
	CreatedAt        time.Time `json:"created_at"`
}

// ModelField representa um campo minificado de um Schema/DTO/Entidade.
type ModelField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Tag      string `json:"tag,omitempty"`
}

// DataModel representa um schema, DTO ou entidade de dados indexado no projeto.
type DataModel struct {
	ID               int64         `json:"id"`
	ProjectID        string        `json:"project_id"`
	Name             string        `json:"name"`
	File             string        `json:"file"`
	Kind             string        `json:"kind"` // 'class', 'interface', 'struct', 'record', 'type', 'schema'
	Line             int           `json:"line"`
	SerializedFields string        `json:"serialized_fields,omitempty"`
	Fields           []*ModelField `json:"fields"`
	CreatedAt        time.Time     `json:"created_at"`
}


// IndexingProgress representa o progresso em tempo real da indexação de um repositório.
type IndexingProgress struct {
	ProjectID      string `json:"project_id"`
	IsIndexing     bool   `json:"is_indexing"`
	CurrentFile    string `json:"current_file"`
	ProcessedFiles int    `json:"processed_files"`
	TotalFiles     int    `json:"total_files"`
	Percent        int    `json:"percent"`
}

// ProjectFileState armazena o estado de hash e metadados de arquivo para indexação incremental.
type ProjectFileState struct {
	ProjectID     string    `json:"project_id"`
	FilePath      string    `json:"filepath"`
	MTime         int64     `json:"mtime"`
	FileSize      int64     `json:"file_size"`
	ContentHash   string    `json:"content_hash"`
	DigestHash    string    `json:"digest_hash"`
	LastIndexedAt time.Time `json:"last_indexed_at"`
}

// DeltaResult representa as alterações detectadas pelo Delta Engine (via Git ou Stat Cache).
type DeltaResult struct {
	ModifiedFiles  []string          `json:"modified_files"`
	AddedFiles     []string          `json:"added_files"`
	DeletedFiles   []string          `json:"deleted_files"`
	RenamedFiles   map[string]string `json:"renamed_files"` // newPath -> oldPath
	UnchangedCount int               `json:"unchanged_count"`
	Strategy       string            `json:"strategy"` // 'git' ou 'stat'
}

// DeltaReport resume as ações executadas no pipeline de sincronização incremental.
type DeltaReport struct {
	ProjectID    string `json:"project_id"`
	DurationMs   int64  `json:"duration_ms"`
	Strategy     string `json:"strategy"`
	FilesParsed  int    `json:"files_parsed"`
	FilesSkipped int    `json:"files_skipped"`
	FilesDeleted int    `json:"files_deleted"`
	TotalFiles   int    `json:"total_files"`
	TotalSymbols int    `json:"total_symbols"`
	Message      string `json:"message"`
}

// FileSymbolStats representa estatísticas de símbolos por arquivo para a tela de mapeamento.
type FileSymbolStats struct {
	File        string `json:"file"`
	Language    string `json:"language"`
	SymbolCount int    `json:"symbol_count"`
	Status      string `json:"status"` // mapped, empty, error
}
