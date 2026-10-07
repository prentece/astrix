package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// DB encapsula a conexão SQLite do Astrix.
type DB struct {
	conn *sql.DB
}

// NewDatabase inicializa o banco de dados SQLite, cria o esquema e ativa WAL mode.
func NewDatabase(dbPath string) (*DB, error) {
	// Garante que o diretório pai existe
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("falha ao criar diretório para o banco de dados: %w", err)
	}

	// Abre a conexão SQLite com pragmas otimizados
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON&_synchronous=NORMAL&_cache_size=-64000", dbPath)
	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir conexão SQLite: %w", err)
	}

	// Limita conexões para evitar contenção de lock no SQLite
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("falha ao aplicar migrações: %w", err)
	}

	return db, nil
}

// Close fecha a conexão com o banco de dados.
func (d *DB) Close() error {
	return d.conn.Close()
}

// Conn retorna o ponteiro para o sql.DB subjacente.
func (d *DB) Conn() *sql.DB {
	return d.conn
}

// migrate cria as tabelas e índices necessários no banco.
func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		path TEXT NOT NULL UNIQUE,
		language TEXT NOT NULL,
		status TEXT NOT NULL,
		error_message TEXT DEFAULT '',
		file_count INTEGER DEFAULT 0,
		symbol_count INTEGER DEFAULT 0,
		auto_sync BOOLEAN DEFAULT 1,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		indexed_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS symbols (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id TEXT NOT NULL,
		file TEXT NOT NULL,
		name TEXT NOT NULL,
		kind TEXT NOT NULL,
		signature TEXT NOT NULL,
		parent TEXT DEFAULT '',
		language TEXT NOT NULL,
		start_line INTEGER NOT NULL,
		end_line INTEGER NOT NULL,
		start_byte INTEGER NOT NULL,
		end_byte INTEGER NOT NULL,
		relevance_score REAL DEFAULT 0.0,
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS references_table (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id TEXT NOT NULL,
		file TEXT NOT NULL,
		line INTEGER NOT NULL,
		text TEXT NOT NULL,
		symbol_name TEXT NOT NULL,
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
		UNIQUE(project_id, file, line, symbol_name)
	);

	CREATE INDEX IF NOT EXISTS idx_symbols_project_name ON symbols(project_id, name);
	CREATE INDEX IF NOT EXISTS idx_symbols_project_file ON symbols(project_id, file);
	CREATE INDEX IF NOT EXISTS idx_symbols_project_kind ON symbols(project_id, kind);
	CREATE INDEX IF NOT EXISTS idx_references_project_symbol ON references_table(project_id, symbol_name);
	CREATE INDEX IF NOT EXISTS idx_references_project_file ON references_table(project_id, file);

	-- Limpeza definitiva de subsistema de trajetórias/episódios/atalhos/sessões
	DROP TABLE IF EXISTS episode_projects;
	DROP TABLE IF EXISTS navigation_shortcuts;
	DROP TABLE IF EXISTS trajectory_settings;
	DROP TABLE IF EXISTS symbol_co_occurrences;
	DROP TABLE IF EXISTS task_episodes;
	DROP TABLE IF EXISTS session_events;
	DROP TABLE IF EXISTS sessions;


	DROP TABLE IF EXISTS api_endpoints;

	CREATE TABLE IF NOT EXISTS dependency_graph (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id TEXT NOT NULL,
		source_symbol TEXT NOT NULL,
		target_symbol TEXT NOT NULL,
		source_file TEXT NOT NULL,
		target_file TEXT DEFAULT '',
		relationship_type TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_depgraph_project_source ON dependency_graph(project_id, source_symbol);
	CREATE INDEX IF NOT EXISTS idx_depgraph_project_target ON dependency_graph(project_id, target_symbol);
	CREATE INDEX IF NOT EXISTS idx_depgraph_project_source_file ON dependency_graph(project_id, source_file);

	CREATE TABLE IF NOT EXISTS data_models (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id TEXT NOT NULL,
		model_name TEXT NOT NULL,
		file TEXT NOT NULL,
		kind TEXT NOT NULL,
		line_number INTEGER NOT NULL,
		serialized_fields TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_data_models_project_name ON data_models(project_id, model_name);
	CREATE INDEX IF NOT EXISTS idx_data_models_project_file ON data_models(project_id, file);

	CREATE TABLE IF NOT EXISTS project_file_states (
		project_id TEXT NOT NULL,
		filepath TEXT NOT NULL,
		mtime INTEGER NOT NULL,
		file_size INTEGER NOT NULL,
		content_hash TEXT NOT NULL,
		digest_hash TEXT NOT NULL,
		last_indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (project_id, filepath),
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_file_states_project ON project_file_states(project_id);
	`

	// Remove tabelas legadas de LLM/sumários se existirem
	_, _ = d.conn.Exec(`DROP TABLE IF EXISTS llm_settings; DROP TABLE IF EXISTS file_summaries;`)

	if _, err := d.conn.Exec(schema); err != nil {
		return err
	}

	// Migração dinâmica para colunas adicionadas em bases existentes
	var count int
	_ = d.conn.QueryRow(`SELECT count(*) FROM pragma_table_info('symbols') WHERE name='relevance_score'`).Scan(&count)
	if count == 0 {
		_, _ = d.conn.Exec(`ALTER TABLE symbols ADD COLUMN relevance_score REAL DEFAULT 0.0`)
	}

	var countAutoSync int
	_ = d.conn.QueryRow(`SELECT count(*) FROM pragma_table_info('projects') WHERE name='auto_sync'`).Scan(&countAutoSync)
	if countAutoSync == 0 {
		_, _ = d.conn.Exec(`ALTER TABLE projects ADD COLUMN auto_sync BOOLEAN DEFAULT 1`)
	}

	_, _ = d.conn.Exec(`CREATE INDEX IF NOT EXISTS idx_symbols_project_score ON symbols(project_id, relevance_score DESC)`)

	return nil
}
