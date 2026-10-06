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

// IndexStore implementa IndexReplacer executando todas as escritas em uma única transação SQLite.
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
	defer tx.Rollback()

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
