package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// DataModelRepo gerencia a persistência e consulta cirúrgica de Schemas, DTOs e Entidades.
type DataModelRepo struct {
	db *DB
}

// NewDataModelRepo cria uma nova instância de DataModelRepo.
func NewDataModelRepo(db *DB) *DataModelRepo {
	return &DataModelRepo{db: db}
}

// ClearProjectDataModels remove todos os modelos de dados de um projeto.
func (r *DataModelRepo) ClearProjectDataModels(projectID string) error {
	_, err := r.db.conn.Exec(`DELETE FROM data_models WHERE project_id = ?`, projectID)
	return err
}

// DeleteByFile remove todos os modelos de dados de um arquivo específico.
func (r *DataModelRepo) DeleteByFile(projectID, file string) error {
	_, err := r.db.conn.Exec(`DELETE FROM data_models WHERE project_id = ? AND file = ?`, projectID, file)
	return err
}

// SaveDataModels persiste uma lista de modelos de dados em lote dentro de uma transação.
func (r *DataModelRepo) SaveDataModels(projectID string, modelsList []*DataModel) error {
	if len(modelsList) == 0 {
		return nil
	}

	tx, err := r.db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := insertDataModelsTx(tx, projectID, modelsList); err != nil {
		return err
	}

	return tx.Commit()
}

// insertDataModelsTx insere modelos de dados dentro de uma transação existente.
func insertDataModelsTx(tx *sql.Tx, projectID string, modelsList []*DataModel) error {
	if len(modelsList) == 0 {
		return nil
	}

	stmt, err := tx.Prepare(`
		INSERT INTO data_models (project_id, model_name, file, kind, line_number, serialized_fields, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, m := range modelsList {
		fieldsJSON, err := json.Marshal(m.Fields)
		if err != nil {
			fieldsJSON = []byte("[]")
		}

		_, err = stmt.Exec(
			projectID,
			m.Name,
			m.File,
			m.Kind,
			m.Line,
			string(fieldsJSON),
			now,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetDataModel busca a definição de um modelo de dados pelo nome no projeto.
func (r *DataModelRepo) GetDataModel(projectID, modelName string) (*DataModel, error) {
	query := `
		SELECT id, project_id, model_name, file, kind, line_number, serialized_fields, created_at
		FROM data_models
		WHERE project_id = ? AND (model_name = ? OR model_name LIKE ?)
		ORDER BY CASE WHEN model_name = ? THEN 0 ELSE 1 END, id ASC
		LIMIT 1
	`
	row := r.db.conn.QueryRow(query, projectID, modelName, "%"+modelName+"%", modelName)

	var m DataModel
	var fieldsJSON string
	err := row.Scan(
		&m.ID,
		&m.ProjectID,
		&m.Name,
		&m.File,
		&m.Kind,
		&m.Line,
		&fieldsJSON,
		&m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	m.SerializedFields = fieldsJSON
	if fieldsJSON != "" {
		_ = json.Unmarshal([]byte(fieldsJSON), &m.Fields)
	}

	return &m, nil
}

// GetByName é um alias para GetDataModel para manter a consistência de nomenclatura.
func (r *DataModelRepo) GetByName(projectID, modelName string) (*DataModel, error) {
	return r.GetDataModel(projectID, modelName)
}

// ListByProject lista todos os modelos de dados de um projeto.
func (r *DataModelRepo) ListByProject(projectID string) ([]*DataModel, error) {
	query := `
		SELECT id, project_id, model_name, file, kind, line_number, serialized_fields, created_at
		FROM data_models
		WHERE project_id = ?
		ORDER BY model_name ASC, file ASC
	`
	rows, err := r.db.conn.Query(query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*DataModel
	for rows.Next() {
		var m DataModel
		var fieldsJSON string
		err := rows.Scan(
			&m.ID,
			&m.ProjectID,
			&m.Name,
			&m.File,
			&m.Kind,
			&m.Line,
			&fieldsJSON,
			&m.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		m.SerializedFields = fieldsJSON
		if fieldsJSON != "" {
			_ = json.Unmarshal([]byte(fieldsJSON), &m.Fields)
		}
		result = append(result, &m)
	}

	return result, nil
}

// ListDataModels lista os modelos de dados cadastrados no projeto com suporte a paginação.
func (r *DataModelRepo) ListDataModels(projectID string, limit, offset int) ([]*DataModel, bool, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, project_id, model_name, file, kind, line_number, serialized_fields, created_at
		FROM data_models
		WHERE project_id = ?
		ORDER BY model_name ASC, file ASC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.conn.Query(query, projectID, limit+1, offset)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var result []*DataModel
	for rows.Next() {
		var m DataModel
		var fieldsJSON string
		err := rows.Scan(
			&m.ID,
			&m.ProjectID,
			&m.Name,
			&m.File,
			&m.Kind,
			&m.Line,
			&fieldsJSON,
			&m.CreatedAt,
		)
		if err != nil {
			return nil, false, err
		}

		m.SerializedFields = fieldsJSON
		if fieldsJSON != "" {
			_ = json.Unmarshal([]byte(fieldsJSON), &m.Fields)
		}
		result = append(result, &m)
	}

	hasMore := false
	if len(result) > limit {
		hasMore = true
		result = result[:limit]
	}

	return result, hasMore, nil
}
