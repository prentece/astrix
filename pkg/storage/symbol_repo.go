package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
)

// SymbolRepo gerencia as operações de persistência de símbolos e referências AST.
type SymbolRepo struct {
	db *DB
}

// NewSymbolRepo cria uma nova instância de SymbolRepo.
func NewSymbolRepo(db *DB) *SymbolRepo {
	return &SymbolRepo{db: db}
}

// ClearProjectData remove todos os símbolos e referências de um projeto.
func (r *SymbolRepo) ClearProjectData(projectID string) error {
	tx, err := r.db.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := clearSymbolsTx(tx, projectID); err != nil {
		return err
	}

	return tx.Commit()
}

// clearSymbolsTx remove símbolos e referências de um projeto dentro de uma transação existente.
func clearSymbolsTx(tx *sql.Tx, projectID string) error {
	if _, err := tx.Exec(`DELETE FROM symbols WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM references_table WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	return nil
}

// DeleteByFile remove todos os símbolos e referências de um arquivo específico.
func (r *SymbolRepo) DeleteByFile(projectID, file string) error {
	tx, err := r.db.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM symbols WHERE project_id = ? AND file = ?`, projectID, file); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM references_table WHERE project_id = ? AND file = ?`, projectID, file); err != nil {
		return err
	}

	return tx.Commit()
}

// SaveSymbols salva uma lista de símbolos em lote utilizando transação.
func (r *SymbolRepo) SaveSymbols(projectID string, symbols []*Symbol) error {
	if len(symbols) == 0 {
		return nil
	}

	tx, err := r.db.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertSymbolsTx(tx, projectID, symbols); err != nil {
		return err
	}

	return tx.Commit()
}

// insertSymbolsTx insere símbolos em lote dentro de uma transação existente.
func insertSymbolsTx(tx *sql.Tx, projectID string, symbols []*Symbol) error {
	if len(symbols) == 0 {
		return nil
	}

	stmt, err := tx.Prepare(`
	INSERT INTO symbols (project_id, file, name, kind, signature, parent, language, start_line, end_line, start_byte, end_byte, relevance_score)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, s := range symbols {
		_, err := stmt.Exec(
			projectID,
			s.File,
			s.Name,
			string(s.Kind),
			s.Signature,
			s.Parent,
			s.Language,
			s.StartLine,
			s.EndLine,
			s.StartByte,
			s.EndByte,
			s.RelevanceScore,
		)
		if err != nil {
			return fmt.Errorf("falha ao inserir símbolo %s: %w", s.Name, err)
		}
	}
	return nil
}

// SaveReferences salva uma lista de chamadores/referências em lote utilizando transação.
func (r *SymbolRepo) SaveReferences(projectID string, refs []*CallerInfo) error {
	if len(refs) == 0 {
		return nil
	}

	tx, err := r.db.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertReferencesTx(tx, projectID, refs); err != nil {
		return err
	}

	return tx.Commit()
}

// insertReferencesTx insere referências em lote dentro de uma transação existente.
func insertReferencesTx(tx *sql.Tx, projectID string, refs []*CallerInfo) error {
	if len(refs) == 0 {
		return nil
	}

	stmt, err := tx.Prepare(`
	INSERT OR IGNORE INTO references_table (project_id, file, line, text, symbol_name)
	VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, ref := range refs {
		_, err := stmt.Exec(
			projectID,
			ref.File,
			ref.Line,
			ref.Text,
			ref.SymbolName,
		)
		if err != nil {
			return fmt.Errorf("falha ao inserir referência %s: %w", ref.SymbolName, err)
		}
	}
	return nil
}

// FindSymbol busca símbolos por nome exato ou contenção parcial no projeto com suporte a ordenação por relevância e paginação.
func (r *SymbolRepo) FindSymbol(projectID, symbolName string, limit, offset int) ([]*Symbol, bool, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := `
	SELECT id, project_id, file, name, kind, signature, parent, language, start_line, end_line, start_byte, end_byte, relevance_score
	FROM symbols
	WHERE project_id = ? AND (name = ? OR name LIKE ?)
	ORDER BY 
		CASE WHEN name = ? THEN 0 ELSE 1 END,
		relevance_score DESC,
		name ASC,
		file ASC,
		start_line ASC
	LIMIT ? OFFSET ?
	`
	rows, err := r.db.conn.Query(query, projectID, symbolName, "%"+symbolName+"%", symbolName, limit+1, offset)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = rows.Close() }()

	var symbols []*Symbol
	for rows.Next() {
		var s Symbol
		var kindStr string
		err := rows.Scan(
			&s.ID,
			&s.ProjectID,
			&s.File,
			&s.Name,
			&kindStr,
			&s.Signature,
			&s.Parent,
			&s.Language,
			&s.StartLine,
			&s.EndLine,
			&s.StartByte,
			&s.EndByte,
			&s.RelevanceScore,
		)
		if err != nil {
			return nil, false, err
		}
		s.Kind = SymbolKind(kindStr)
		symbols = append(symbols, &s)
	}

	hasMore := false
	if len(symbols) > limit {
		hasMore = true
		symbols = symbols[:limit]
	}

	return symbols, hasMore, nil
}

// GetSymbolByFileAndName busca a definição exata de um símbolo em um arquivo específico.
func (r *SymbolRepo) GetSymbolByFileAndName(projectID, file, symbolName string) (*Symbol, error) {
	query := `
	SELECT id, project_id, file, name, kind, signature, parent, language, start_line, end_line, start_byte, end_byte, relevance_score
	FROM symbols
	WHERE project_id = ? AND file = ? AND (name = ? OR name LIKE ?)
	ORDER BY CASE WHEN name = ? THEN 0 ELSE 1 END, (end_byte - start_byte) DESC
	LIMIT 1
	`
	row := r.db.conn.QueryRow(query, projectID, file, symbolName, "%"+symbolName+"%", symbolName)

	var s Symbol
	var kindStr string
	err := row.Scan(
		&s.ID,
		&s.ProjectID,
		&s.File,
		&s.Name,
		&kindStr,
		&s.Signature,
		&s.Parent,
		&s.Language,
		&s.StartLine,
		&s.EndLine,
		&s.StartByte,
		&s.EndByte,
		&s.RelevanceScore,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	s.Kind = SymbolKind(kindStr)
	return &s, nil
}

// FindReferences busca as referências/chamadas registradas para um símbolo com paginação.
func (r *SymbolRepo) FindReferences(projectID, symbolName string, limit, offset int) ([]*CallerInfo, bool, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := `
	SELECT id, project_id, file, line, text, symbol_name
	FROM references_table
	WHERE project_id = ? AND symbol_name = ?
	ORDER BY file ASC, line ASC
	LIMIT ? OFFSET ?
	`
	rows, err := r.db.conn.Query(query, projectID, symbolName, limit+1, offset)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = rows.Close() }()

	var refs []*CallerInfo
	for rows.Next() {
		var ref CallerInfo
		err := rows.Scan(
			&ref.ID,
			&ref.ProjectID,
			&ref.File,
			&ref.Line,
			&ref.Text,
			&ref.SymbolName,
		)
		if err != nil {
			return nil, false, err
		}
		refs = append(refs, &ref)
	}

	hasMore := false
	if len(refs) > limit {
		hasMore = true
		refs = refs[:limit]
	}

	return refs, hasMore, nil
}

// UpdateRelevanceScores atualiza os scores de relevância dos símbolos em lote usando transação.
func (r *SymbolRepo) UpdateRelevanceScores(projectID string, symbolScores map[int64]float64) error {
	if len(symbolScores) == 0 {
		return nil
	}

	tx, err := r.db.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`UPDATE symbols SET relevance_score = ? WHERE id = ? AND project_id = ?`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for id, score := range symbolScores {
		if _, err := stmt.Exec(score, id, projectID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetFileImportCounts retorna a quantidade de arquivos distintos que importam ou referenciam cada arquivo do projeto.
func (r *SymbolRepo) GetFileImportCounts(projectID string) (map[string]int, error) {
	counts := make(map[string]int)

	// 1. Contabiliza a partir do grafo de dependências
	rows, err := r.db.conn.Query(`
		SELECT target_file, COUNT(DISTINCT source_file)
		FROM dependency_graph
		WHERE project_id = ? AND target_file != ''
		GROUP BY target_file
	`, projectID)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var file string
			var count int
			if err := rows.Scan(&file, &count); err == nil && file != "" {
				counts[file] = count
			}
		}
	}

	return counts, nil
}

// GetSymbolReferenceCounts retorna a contagem de referências diretas para cada símbolo no projeto.
func (r *SymbolRepo) GetSymbolReferenceCounts(projectID string) (map[string]int, error) {
	counts := make(map[string]int)

	rows, err := r.db.conn.Query(`
		SELECT symbol_name, COUNT(*)
		FROM references_table
		WHERE project_id = ?
		GROUP BY symbol_name
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err == nil && name != "" {
			counts[name] = count
		}
	}

	return counts, nil
}

// GetAllSymbolsForRanking recupera todos os símbolos do projeto para cálculo do ranking de centralidade.
func (r *SymbolRepo) GetAllSymbolsForRanking(projectID string) ([]*Symbol, error) {
	query := `
	SELECT id, project_id, file, name, kind, signature, parent, language, start_line, end_line, start_byte, end_byte, relevance_score
	FROM symbols
	WHERE project_id = ?
	`
	rows, err := r.db.conn.Query(query, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var symbols []*Symbol
	for rows.Next() {
		var s Symbol
		var kindStr string
		err := rows.Scan(
			&s.ID,
			&s.ProjectID,
			&s.File,
			&s.Name,
			&kindStr,
			&s.Signature,
			&s.Parent,
			&s.Language,
			&s.StartLine,
			&s.EndLine,
			&s.StartByte,
			&s.EndByte,
			&s.RelevanceScore,
		)
		if err != nil {
			return nil, err
		}
		s.Kind = SymbolKind(kindStr)
		symbols = append(symbols, &s)
	}

	return symbols, nil
}

// GetSymbolsByFileAndLineRange retorna símbolos cujo range [start_line, end_line] se sobrepõe ao intervalo solicitado.
func (r *SymbolRepo) GetSymbolsByFileAndLineRange(projectID, file string, startLine, endLine int) ([]*Symbol, error) {
	query := `
	SELECT id, project_id, file, name, kind, signature, parent, language, start_line, end_line, start_byte, end_byte, relevance_score
	FROM symbols
	WHERE project_id = ? AND file = ? AND start_line <= ? AND end_line >= ?
	ORDER BY start_line ASC
	`
	rows, err := r.db.conn.Query(query, projectID, file, endLine, startLine)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var symbols []*Symbol
	for rows.Next() {
		var s Symbol
		var kindStr string
		err := rows.Scan(
			&s.ID, &s.ProjectID, &s.File, &s.Name, &kindStr,
			&s.Signature, &s.Parent, &s.Language,
			&s.StartLine, &s.EndLine, &s.StartByte, &s.EndByte,
			&s.RelevanceScore,
		)
		if err != nil {
			return nil, err
		}
		s.Kind = SymbolKind(kindStr)
		symbols = append(symbols, &s)
	}

	return symbols, nil
}

// GetSymbolCountsByFile retorna a contagem de símbolos agrupados por arquivo para a tela de mapeamento.
func (r *SymbolRepo) GetSymbolCountsByFile(projectID string) ([]*FileSymbolStats, error) {
	// Cruza todos os arquivos do projeto (project_file_states) com a tabela de símbolos para incluir arquivos com 0 símbolos
	query := `
	SELECT 
		pfs.filepath as file,
		COALESCE(MAX(s.language), '') as language,
		COUNT(s.id) as symbol_count
	FROM project_file_states pfs
	LEFT JOIN symbols s ON s.project_id = pfs.project_id AND s.file = pfs.filepath
	WHERE pfs.project_id = ?
	GROUP BY pfs.filepath
	ORDER BY symbol_count DESC, pfs.filepath ASC
	`
	rows, err := r.db.conn.Query(query, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var stats []*FileSymbolStats
	for rows.Next() {
		var s FileSymbolStats
		if err := rows.Scan(&s.File, &s.Language, &s.SymbolCount); err != nil {
			return nil, err
		}
		if s.Language == "" {
			ext := filepath.Ext(s.File)
			if len(ext) > 1 {
				s.Language = ext[1:]
			} else {
				s.Language = "text"
			}
		}
		if s.SymbolCount > 0 {
			s.Status = "mapped"
		} else {
			s.Status = "empty"
		}
		stats = append(stats, &s)
	}

	// Fallback para projetos indexados sem registro prévio em project_file_states
	if len(stats) == 0 {
		legacyQuery := `
		SELECT file, language, COUNT(*) as symbol_count
		FROM symbols
		WHERE project_id = ?
		GROUP BY file
		ORDER BY symbol_count DESC, file ASC
		`
		legRows, err := r.db.conn.Query(legacyQuery, projectID)
		if err != nil {
			return nil, err
		}
		defer func() { _ = legRows.Close() }()

		for legRows.Next() {
			var s FileSymbolStats
			if err := legRows.Scan(&s.File, &s.Language, &s.SymbolCount); err != nil {
				return nil, err
			}
			if s.Language == "" {
				ext := filepath.Ext(s.File)
				if len(ext) > 1 {
					s.Language = ext[1:]
				} else {
					s.Language = "text"
				}
			}
			if s.SymbolCount > 0 {
				s.Status = "mapped"
			} else {
				s.Status = "empty"
			}
			stats = append(stats, &s)
		}
	}

	return stats, nil
}
