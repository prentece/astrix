package storage

import (
	"database/sql"
	"errors"
	"time"
)

// ProjectRepo gerencia as operações de persistência de projetos.
type ProjectRepo struct {
	db *DB
}

// NewProjectRepo cria uma nova instância de ProjectRepo.
func NewProjectRepo(db *DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

// Create insere um novo projeto no banco.
func (r *ProjectRepo) Create(p *Project) error {
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.Status == "" {
		p.Status = StatusPending
	}
	if !p.AutoSync {
		p.AutoSync = true
	}

	query := `
	INSERT INTO projects (id, name, path, language, status, error_message, file_count, symbol_count, auto_sync, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.conn.Exec(query, p.ID, p.Name, p.Path, p.Language, p.Status, p.ErrorMessage, p.FileCount, p.SymbolCount, p.AutoSync, p.CreatedAt, p.UpdatedAt)
	return err
}

// GetByID busca um projeto pelo seu ID.
func (r *ProjectRepo) GetByID(id string) (*Project, error) {
	query := `
	SELECT id, name, path, language, status, error_message, file_count, symbol_count, auto_sync, created_at, updated_at, indexed_at
	FROM projects
	WHERE id = ?
	`
	row := r.db.conn.QueryRow(query, id)

	var p Project
	var indexedAt sql.NullTime
	err := row.Scan(&p.ID, &p.Name, &p.Path, &p.Language, &p.Status, &p.ErrorMessage, &p.FileCount, &p.SymbolCount, &p.AutoSync, &p.CreatedAt, &p.UpdatedAt, &indexedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if indexedAt.Valid {
		p.IndexedAt = &indexedAt.Time
	}
	return &p, nil
}

// ListAll lista todos os projetos cadastrados ordenados por data de criação.
func (r *ProjectRepo) ListAll() ([]*Project, error) {
	query := `
	SELECT id, name, path, language, status, error_message, file_count, symbol_count, auto_sync, created_at, updated_at, indexed_at
	FROM projects
	ORDER BY created_at DESC
	`
	rows, err := r.db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var projects []*Project
	for rows.Next() {
		var p Project
		var indexedAt sql.NullTime
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Language, &p.Status, &p.ErrorMessage, &p.FileCount, &p.SymbolCount, &p.AutoSync, &p.CreatedAt, &p.UpdatedAt, &indexedAt); err != nil {
			return nil, err
		}
		if indexedAt.Valid {
			p.IndexedAt = &indexedAt.Time
		}
		projects = append(projects, &p)
	}
	return projects, nil
}

// SetAutoSync atualiza a preferência de sincronização automática do projeto.
func (r *ProjectRepo) SetAutoSync(id string, autoSync bool) error {
	now := time.Now().UTC()
	query := `UPDATE projects SET auto_sync = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.conn.Exec(query, autoSync, now, id)
	return err
}

// UpdateStatus atualiza o status de indexação e métricas do projeto.
func (r *ProjectRepo) UpdateStatus(id string, status ProjectStatus, errMsg string, fileCount, symbolCount int) error {
	now := time.Now().UTC()
	var indexedAt *time.Time
	if status == StatusReady {
		indexedAt = &now
	}

	query := `
	UPDATE projects
	SET status = ?, error_message = ?, file_count = ?, symbol_count = ?, updated_at = ?, indexed_at = COALESCE(?, indexed_at)
	WHERE id = ?
	`
	_, err := r.db.conn.Exec(query, status, errMsg, fileCount, symbolCount, now, indexedAt, id)
	return err
}

// ResetDanglingIndexingStatus redefine projetos que ficaram travados em 'indexing' para 'ready'.
func (r *ProjectRepo) ResetDanglingIndexingStatus() error {
	query := `UPDATE projects SET status = 'ready' WHERE status = 'indexing'`
	_, err := r.db.conn.Exec(query)
	return err
}

// Delete remove o projeto, seus eventos de telemetria, sessões órfãs e símbolos/referências associados.
func (r *ProjectRepo) Delete(id string) error {
	tx, err := r.db.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Remove o registro do projeto (symbols, references, etc. deletados via FK CASCADE)
	if _, err := tx.Exec("DELETE FROM projects WHERE id = ?", id); err != nil {
		return err
	}

	return tx.Commit()
}
