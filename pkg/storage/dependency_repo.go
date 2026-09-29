package storage

import (
	"database/sql"
	"time"
)

// DependencyGraphRepo gerencia a persistência e consulta de arestas do grafo de dependências.
type DependencyGraphRepo struct {
	db *DB
}

// NewDependencyGraphRepo cria uma nova instância de DependencyGraphRepo.
func NewDependencyGraphRepo(db *DB) *DependencyGraphRepo {
	return &DependencyGraphRepo{db: db}
}

// ClearProjectDependencies remove todas as dependências registradas de um projeto.
func (r *DependencyGraphRepo) ClearProjectDependencies(projectID string) error {
	_, err := r.db.conn.Exec(`DELETE FROM dependency_graph WHERE project_id = ?`, projectID)
	return err
}

// DeleteByFile remove todas as dependências originadas ou direcionadas a um arquivo específico.
func (r *DependencyGraphRepo) DeleteByFile(projectID, file string) error {
	_, err := r.db.conn.Exec(`DELETE FROM dependency_graph WHERE project_id = ? AND (source_file = ? OR target_file = ?)`, projectID, file, file)
	return err
}

// SaveDependencies persiste uma lista de arestas de dependência em lote dentro de uma transação.
func (r *DependencyGraphRepo) SaveDependencies(projectID string, edges []*DependencyEdge) error {
	if len(edges) == 0 {
		return nil
	}

	tx, err := r.db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO dependency_graph (project_id, source_symbol, target_symbol, source_file, target_file, relationship_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, edge := range edges {
		createdAt := edge.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		_, err := stmt.Exec(
			projectID,
			edge.SourceSymbol,
			edge.TargetSymbol,
			edge.SourceFile,
			edge.TargetFile,
			edge.RelationshipType,
			createdAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetDownstreamEdges obtém todas as arestas onde source_symbol = symbol.
func (r *DependencyGraphRepo) GetDownstreamEdges(projectID, symbol string) ([]*DependencyEdge, error) {
	query := `
		SELECT id, project_id, source_symbol, target_symbol, source_file, target_file, relationship_type, created_at
		FROM dependency_graph
		WHERE project_id = ? AND source_symbol = ?
		ORDER BY relationship_type ASC, target_symbol ASC
	`
	return r.queryEdges(query, projectID, symbol)
}

// GetUpstreamEdges obtém todas as arestas onde target_symbol = symbol.
func (r *DependencyGraphRepo) GetUpstreamEdges(projectID, symbol string) ([]*DependencyEdge, error) {
	query := `
		SELECT id, project_id, source_symbol, target_symbol, source_file, target_file, relationship_type, created_at
		FROM dependency_graph
		WHERE project_id = ? AND target_symbol = ?
		ORDER BY relationship_type ASC, source_symbol ASC
	`
	return r.queryEdges(query, projectID, symbol)
}

// GetAllEdges obtém todas as arestas de um projeto.
func (r *DependencyGraphRepo) GetAllEdges(projectID string) ([]*DependencyEdge, error) {
	query := `
		SELECT id, project_id, source_symbol, target_symbol, source_file, target_file, relationship_type, created_at
		FROM dependency_graph
		WHERE project_id = ?
		ORDER BY source_symbol ASC, target_symbol ASC
	`
	return r.queryEdges(query, projectID)
}

// UpdateTargetFiles preenche o target_file de arestas baseado em um mapa de symbol -> filepath.
func (r *DependencyGraphRepo) UpdateTargetFiles(projectID string, symbolToFileMap map[string]string) error {
	if len(symbolToFileMap) == 0 {
		return nil
	}

	tx, err := r.db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		UPDATE dependency_graph
		SET target_file = ?
		WHERE project_id = ? AND target_symbol = ? AND (target_file = '' OR target_file IS NULL)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for sym, file := range symbolToFileMap {
		if file != "" {
			if _, err := stmt.Exec(file, projectID, sym); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (r *DependencyGraphRepo) queryEdges(query string, args ...any) ([]*DependencyEdge, error) {
	rows, err := r.db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []*DependencyEdge
	for rows.Next() {
		var edge DependencyEdge
		if err := rows.Scan(
			&edge.ID,
			&edge.ProjectID,
			&edge.SourceSymbol,
			&edge.TargetSymbol,
			&edge.SourceFile,
			&edge.TargetFile,
			&edge.RelationshipType,
			&edge.CreatedAt,
		); err != nil {
			return nil, err
		}
		edges = append(edges, &edge)
	}

	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return edges, nil
}
