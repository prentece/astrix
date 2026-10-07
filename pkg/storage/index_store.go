package storage

import "fmt"

// IndexSnapshot agrupa todo o resultado de uma indexação completa de um projeto.
type IndexSnapshot struct {
	ProjectID    string
	Symbols      []*Symbol
	References   []*CallerInfo
	Dependencies []*DependencyEdge
	DataModels   []*DataModel
	FileStates   []*ProjectFileState
	// ReplaceFileStates indica se os estados de arquivo (baseline do delta) também devem ser substituídos.
	ReplaceFileStates bool
}

// IndexReplacer substitui atomicamente todo o índice de um projeto.
type IndexReplacer interface {
	ReplaceProjectIndex(snapshot *IndexSnapshot) error
}

// IncrementalIndexDelta agrupa todas as alterações de um ciclo de sincronização incremental.
type IncrementalIndexDelta struct {
	ProjectID     string
	DeletedFiles  []string
	ModifiedFiles []string
	Symbols       []*Symbol
	References    []*CallerInfo
	Dependencies  []*DependencyEdge
	DataModels    []*DataModel
	FileStates    []*ProjectFileState
}

// IncrementalApplier aplica deltas incrementais no banco de forma atômica.
type IncrementalApplier interface {
	ApplyIncrementalDelta(delta *IncrementalIndexDelta) error
}

// IndexStore implementa IndexReplacer e IncrementalApplier executando escritas em transações SQLite.
type IndexStore struct {
	db *DB
}

// NewIndexStore cria um IndexStore sobre o banco informado.
func NewIndexStore(db *DB) *IndexStore {
	return &IndexStore{db: db}
}

// ReplaceProjectIndex apaga o índice anterior e grava o novo em uma única transação.
// Em qualquer falha ocorre rollback completo e o índice anterior permanece intacto.
func (s *IndexStore) ReplaceProjectIndex(snap *IndexSnapshot) error {
	if snap == nil || snap.ProjectID == "" {
		return fmt.Errorf("snapshot de índice inválido")
	}

	tx, err := s.db.conn.Begin()
	if err != nil {
		return fmt.Errorf("falha ao iniciar transação de reindexação: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := clearSymbolsTx(tx, snap.ProjectID); err != nil {
		return fmt.Errorf("falha ao limpar símbolos: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM dependency_graph WHERE project_id = ?`, snap.ProjectID); err != nil {
		return fmt.Errorf("falha ao limpar dependências: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM data_models WHERE project_id = ?`, snap.ProjectID); err != nil {
		return fmt.Errorf("falha ao limpar modelos de dados: %w", err)
	}

	if err := insertSymbolsTx(tx, snap.ProjectID, snap.Symbols); err != nil {
		return fmt.Errorf("falha ao salvar símbolos: %w", err)
	}
	if err := insertReferencesTx(tx, snap.ProjectID, snap.References); err != nil {
		return fmt.Errorf("falha ao salvar referências: %w", err)
	}
	if err := insertDependenciesTx(tx, snap.ProjectID, snap.Dependencies); err != nil {
		return fmt.Errorf("falha ao salvar dependências: %w", err)
	}
	if err := insertDataModelsTx(tx, snap.ProjectID, snap.DataModels); err != nil {
		return fmt.Errorf("falha ao salvar modelos de dados: %w", err)
	}

	if snap.ReplaceFileStates {
		if _, err := tx.Exec(`DELETE FROM project_file_states WHERE project_id = ?`, snap.ProjectID); err != nil {
			return fmt.Errorf("falha ao limpar estados de arquivos: %w", err)
		}
		if err := upsertFileStatesTx(tx, snap.FileStates); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ApplyIncrementalDelta aplica remoções e inserções de um delta incremental em uma única transação SQLite.
// Se qualquer operação falhar, o rollback é acionado e nenhuma modificação parcial é persistida.
func (s *IndexStore) ApplyIncrementalDelta(delta *IncrementalIndexDelta) error {
	if delta == nil || delta.ProjectID == "" {
		return fmt.Errorf("delta incremental inválido")
	}

	tx, err := s.db.conn.Begin()
	if err != nil {
		return fmt.Errorf("falha ao iniciar transação incremental: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Limpa símbolos, referências, dependências e modelos dos arquivos deletados e modificados
	filesToClear := append([]string{}, delta.DeletedFiles...)
	filesToClear = append(filesToClear, delta.ModifiedFiles...)

	for _, file := range filesToClear {
		if _, err := tx.Exec(`DELETE FROM symbols WHERE project_id = ? AND file = ?`, delta.ProjectID, file); err != nil {
			return fmt.Errorf("falha ao deletar símbolos de %s: %w", file, err)
		}
		if _, err := tx.Exec(`DELETE FROM references_table WHERE project_id = ? AND file = ?`, delta.ProjectID, file); err != nil {
			return fmt.Errorf("falha ao deletar referências de %s: %w", file, err)
		}
		if _, err := tx.Exec(`DELETE FROM dependency_graph WHERE project_id = ? AND source_file = ?`, delta.ProjectID, file); err != nil {
			return fmt.Errorf("falha ao deletar dependências de %s: %w", file, err)
		}
		if _, err := tx.Exec(`DELETE FROM data_models WHERE project_id = ? AND file = ?`, delta.ProjectID, file); err != nil {
			return fmt.Errorf("falha ao deletar modelos de %s: %w", file, err)
		}
	}

	// 2. Remove estados dos arquivos deletados
	for _, delFile := range delta.DeletedFiles {
		if _, err := tx.Exec(`DELETE FROM project_file_states WHERE project_id = ? AND filepath = ?`, delta.ProjectID, delFile); err != nil {
			return fmt.Errorf("falha ao remover estado de arquivo %s: %w", delFile, err)
		}
	}

	// 3. Insere novos símbolos, referências, dependências e modelos
	if err := insertSymbolsTx(tx, delta.ProjectID, delta.Symbols); err != nil {
		return fmt.Errorf("falha ao salvar novos símbolos: %w", err)
	}
	if err := insertReferencesTx(tx, delta.ProjectID, delta.References); err != nil {
		return fmt.Errorf("falha ao salvar novas referências: %w", err)
	}
	if err := insertDependenciesTx(tx, delta.ProjectID, delta.Dependencies); err != nil {
		return fmt.Errorf("falha ao salvar novas dependências: %w", err)
	}
	if err := insertDataModelsTx(tx, delta.ProjectID, delta.DataModels); err != nil {
		return fmt.Errorf("falha ao salvar novos modelos de dados: %w", err)
	}

	// 4. Upsert de file states
	if len(delta.FileStates) > 0 {
		if err := upsertFileStatesTx(tx, delta.FileStates); err != nil {
			return fmt.Errorf("falha ao atualizar estados de arquivos: %w", err)
		}
	}

	return tx.Commit()
}
