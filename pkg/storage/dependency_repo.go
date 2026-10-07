package storage

import (
	"database/sql"
	"path/filepath"
	"strings"
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
	defer func() { _ = tx.Rollback() }()

	if err := insertDependenciesTx(tx, projectID, edges); err != nil {
		return err
	}

	return tx.Commit()
}

// insertDependenciesTx insere arestas de dependência dentro de uma transação existente.
func insertDependenciesTx(tx *sql.Tx, projectID string, edges []*DependencyEdge) error {
	if len(edges) == 0 {
		return nil
	}

	stmt, err := tx.Prepare(`
		INSERT INTO dependency_graph (project_id, source_symbol, target_symbol, source_file, target_file, relationship_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

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
	return nil
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
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		UPDATE dependency_graph
		SET target_file = ?
		WHERE project_id = ? AND target_symbol = ? AND (target_file = '' OR target_file IS NULL)
	`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for sym, file := range symbolToFileMap {
		if file != "" {
			if _, err := stmt.Exec(file, projectID, sym); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// ResolveTargetFiles preenche o target_file de arestas pendentes desambiguando símbolos homônimos
// através de heurísticas de proximidade de diretório e penalização de arquivos de teste.
func (r *DependencyGraphRepo) ResolveTargetFiles(projectID string, symbols []*Symbol) error {
	if len(symbols) == 0 {
		return nil
	}

	query := `
		SELECT id, project_id, source_symbol, target_symbol, source_file, target_file, relationship_type, created_at
		FROM dependency_graph
		WHERE project_id = ? AND (target_file = '' OR target_file IS NULL)
	`
	unresolvedEdges, err := r.queryEdges(query, projectID)
	if err != nil || len(unresolvedEdges) == 0 {
		return err
	}

	symMap := make(map[string][]*Symbol)
	for _, s := range symbols {
		if s != nil && s.Name != "" && s.File != "" {
			symMap[s.Name] = append(symMap[s.Name], s)
		}
	}

	tx, err := r.db.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`UPDATE dependency_graph SET target_file = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	updatedCount := 0
	for _, edge := range unresolvedEdges {
		candidates := symMap[edge.TargetSymbol]
		if len(candidates) == 0 {
			continue
		}

		bestFile := resolveBestTargetFile(edge.SourceFile, candidates)
		if bestFile != "" {
			if _, err := stmt.Exec(bestFile, edge.ID); err != nil {
				return err
			}
			updatedCount++
		}
	}

	if updatedCount > 0 {
		return tx.Commit()
	}
	return nil
}

func resolveBestTargetFile(sourceFile string, candidates []*Symbol) string {
	if len(candidates) == 0 {
		return ""
	}
	if len(candidates) == 1 {
		return candidates[0].File
	}

	sourceDir := filepath.Dir(sourceFile)
	bestScore := -999999
	bestFile := candidates[0].File

	for _, cand := range candidates {
		candFile := cand.File
		score := 0

		// Penalização para arquivos de teste
		isTest := strings.HasSuffix(candFile, "_test.go") ||
			strings.Contains(candFile, ".test.") ||
			strings.Contains(candFile, ".spec.") ||
			strings.HasPrefix(candFile, "tests/") ||
			strings.Contains(candFile, "/test/") ||
			strings.Contains(candFile, "/tests/")
		if isTest {
			score -= 1000
		}

		// Bônus se estiver no mesmo diretório
		candDir := filepath.Dir(candFile)
		if sourceDir == candDir {
			score += 500
		} else {
			// Bônus por prefixo comum de diretórios
			commonPrefixLen := commonPathPrefixLength(sourceDir, candDir)
			score += commonPrefixLen * 10
		}

		// Preferência por menor distância relativa
		if rel, err := filepath.Rel(sourceDir, candDir); err == nil {
			depth := len(strings.Split(filepath.Clean(rel), string(filepath.Separator)))
			score -= depth * 5
		}

		if score > bestScore {
			bestScore = score
			bestFile = candFile
		}
	}

	return bestFile
}

func commonPathPrefixLength(p1, p2 string) int {
	parts1 := strings.Split(filepath.Clean(p1), string(filepath.Separator))
	parts2 := strings.Split(filepath.Clean(p2), string(filepath.Separator))
	common := 0
	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		if parts1[i] == parts2[i] {
			common++
		} else {
			break
		}
	}
	return common
}

func (r *DependencyGraphRepo) queryEdges(query string, args ...any) ([]*DependencyEdge, error) {
	rows, err := r.db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
