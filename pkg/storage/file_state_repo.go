package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// SQLFileStateRepository implementa FileStateRepository usando SQLite.
type SQLFileStateRepository struct {
	db *sql.DB
}

// NewSQLFileStateRepository cria uma nova instância de SQLFileStateRepository.
func NewSQLFileStateRepository(db *sql.DB) *SQLFileStateRepository {
	return &SQLFileStateRepository{db: db}
}

// Get busca o estado de um arquivo específico.
func (r *SQLFileStateRepository) Get(projectID, filepath string) (*ProjectFileState, error) {
	query := `SELECT project_id, filepath, mtime, file_size, content_hash, digest_hash, last_indexed_at 
	          FROM project_file_states 
	          WHERE project_id = ? AND filepath = ?`
	row := r.db.QueryRow(query, projectID, filepath)

	var s ProjectFileState
	var lastIndexedStr string
	err := row.Scan(&s.ProjectID, &s.FilePath, &s.MTime, &s.FileSize, &s.ContentHash, &s.DigestHash, &lastIndexedStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar estado do arquivo %s: %w", filepath, err)
	}

	if t, err := time.Parse(time.RFC3339, lastIndexedStr); err == nil {
		s.LastIndexedAt = t
	} else if t, err := time.Parse("2006-01-02 15:04:05", lastIndexedStr); err == nil {
		s.LastIndexedAt = t
	}

	return &s, nil
}

// ListByProject retorna um mapa de filepath -> ProjectFileState para o projeto.
func (r *SQLFileStateRepository) ListByProject(projectID string) (map[string]*ProjectFileState, error) {
	query := `SELECT project_id, filepath, mtime, file_size, content_hash, digest_hash, last_indexed_at 
	          FROM project_file_states 
	          WHERE project_id = ?`
	rows, err := r.db.Query(query, projectID)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar estados de arquivos do projeto %s: %w", projectID, err)
	}
	defer func() { _ = rows.Close() }()

	results := make(map[string]*ProjectFileState)
	for rows.Next() {
		var s ProjectFileState
		var lastIndexedStr string
		if err := rows.Scan(&s.ProjectID, &s.FilePath, &s.MTime, &s.FileSize, &s.ContentHash, &s.DigestHash, &lastIndexedStr); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339, lastIndexedStr); err == nil {
			s.LastIndexedAt = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", lastIndexedStr); err == nil {
			s.LastIndexedAt = t
		}
		results[s.FilePath] = &s
	}

	return results, nil
}

// Upsert insere ou atualiza o estado de um arquivo.
func (r *SQLFileStateRepository) Upsert(state *ProjectFileState) error {
	query := `INSERT INTO project_file_states (project_id, filepath, mtime, file_size, content_hash, digest_hash, last_indexed_at)
	          VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	          ON CONFLICT(project_id, filepath) DO UPDATE SET
	              mtime = excluded.mtime,
	              file_size = excluded.file_size,
	              content_hash = excluded.content_hash,
	              digest_hash = excluded.digest_hash,
	              last_indexed_at = CURRENT_TIMESTAMP`
	_, err := r.db.Exec(query, state.ProjectID, state.FilePath, state.MTime, state.FileSize, state.ContentHash, state.DigestHash)
	if err != nil {
		return fmt.Errorf("erro ao salvar estado do arquivo %s: %w", state.FilePath, err)
	}
	return nil
}

// UpsertBatch insere ou atualiza múltiplos estados de arquivo em lote utilizando uma única transação.
func (r *SQLFileStateRepository) UpsertBatch(states []*ProjectFileState) error {
	if len(states) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação para batch de estados: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := upsertFileStatesTx(tx, states); err != nil {
		return err
	}

	return tx.Commit()
}

// upsertFileStatesTx insere ou atualiza estados de arquivo dentro de uma transação existente.
func upsertFileStatesTx(tx *sql.Tx, states []*ProjectFileState) error {
	if len(states) == 0 {
		return nil
	}

	query := `INSERT INTO project_file_states (project_id, filepath, mtime, file_size, content_hash, digest_hash, last_indexed_at)
	          VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	          ON CONFLICT(project_id, filepath) DO UPDATE SET
	              mtime = excluded.mtime,
	              file_size = excluded.file_size,
	              content_hash = excluded.content_hash,
	              digest_hash = excluded.digest_hash,
	              last_indexed_at = CURRENT_TIMESTAMP`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("erro ao preparar statement de batch de estados: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, s := range states {
		if _, err := stmt.Exec(s.ProjectID, s.FilePath, s.MTime, s.FileSize, s.ContentHash, s.DigestHash); err != nil {
			return fmt.Errorf("erro ao inserir estado do arquivo %s no batch: %w", s.FilePath, err)
		}
	}
	return nil
}

// Delete remove o estado de um único arquivo.
func (r *SQLFileStateRepository) Delete(projectID, filepath string) error {
	query := `DELETE FROM project_file_states WHERE project_id = ? AND filepath = ?`
	_, err := r.db.Exec(query, projectID, filepath)
	if err != nil {
		return fmt.Errorf("erro ao deletar estado do arquivo %s: %w", filepath, err)
	}
	return nil
}

// DeleteByProject remove todos os estados de arquivo de um projeto.
func (r *SQLFileStateRepository) DeleteByProject(projectID string) error {
	query := `DELETE FROM project_file_states WHERE project_id = ?`
	_, err := r.db.Exec(query, projectID)
	if err != nil {
		return fmt.Errorf("erro ao limpar estados do projeto %s: %w", projectID, err)
	}
	return nil
}
