package db_test

import (
	"astrix/pkg/storage"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// poisonTable substitui `table` por uma view equivalente em que a coluna `poisonCol`
// da última linha (maior rowid) provoca um erro de execução do SQLite
// ("integer overflow") durante o step, isto é, DEPOIS de o laço rows.Next() já
// ter devolvido linhas válidas. Isso simula uma falha de iteração (I/O, corrupção)
// de forma determinística, sem depender de ambiente.
func poisonTable(t *testing.T, db *storage.DB, table, poisonCol string) {
	t.Helper()
	conn := db.Conn()
	// PRAGMA é por conexão: fixa uma única conexão para o teste.
	conn.SetMaxOpenConns(1)

	_, err := conn.Exec(`PRAGMA foreign_keys = OFF`)
	require.NoError(t, err)

	real := table + "_real"
	_, err = conn.Exec(fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, table, real))
	require.NoError(t, err)

	rows, err := conn.Query(fmt.Sprintf(`SELECT name FROM pragma_table_info('%s')`, real))
	require.NoError(t, err)
	var cols []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		if name == poisonCol {
			name = fmt.Sprintf(
				`CASE WHEN rowid = (SELECT max(rowid) FROM %s) THEN abs(-9223372036854775808) ELSE %s END AS %s`,
				real, poisonCol, poisonCol,
			)
		}
		cols = append(cols, name)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	require.Contains(t, strings.Join(cols, ","), "abs(", "coluna %s não encontrada em %s", poisonCol, table)

	_, err = conn.Exec(fmt.Sprintf(`CREATE VIEW %s AS SELECT %s FROM %s`, table, strings.Join(cols, ", "), real))
	require.NoError(t, err)
}

func seedRowsErrFixture(t *testing.T, db *storage.DB) {
	t.Helper()
	conn := db.Conn()
	stmts := []string{
		`INSERT INTO projects (id, name, path, language, status, created_at, updated_at)
		 VALUES ('p1','P1','/tmp/p1','go','ready',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
		        ('p2','P2','/tmp/p2','go','ready',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		`INSERT INTO symbols (project_id,file,name,kind,signature,language,start_line,end_line,start_byte,end_byte)
		 VALUES ('p1','a.go','Foo','function','func Foo()','go',1,5,0,10),
		        ('p1','b.go','Foo','function','func Foo()','go',1,5,0,10)`,
		`INSERT INTO references_table (project_id,file,line,text,symbol_name)
		 VALUES ('p1','a.go',1,'Foo()','Foo'),('p1','b.go',2,'Foo()','Foo')`,
		`INSERT INTO dependency_graph (project_id,source_symbol,target_symbol,source_file,target_file,relationship_type,created_at)
		 VALUES ('p1','A','B','a.go','b.go','calls',CURRENT_TIMESTAMP),
		        ('p1','C','D','c.go','d.go','calls',CURRENT_TIMESTAMP)`,
		`INSERT INTO data_models (project_id,model_name,file,kind,line_number,serialized_fields,created_at)
		 VALUES ('p1','M1','a.go','struct',1,'',CURRENT_TIMESTAMP),
		        ('p1','M2','b.go','struct',1,'',CURRENT_TIMESTAMP)`,
		`INSERT INTO project_file_states (project_id,filepath,mtime,file_size,content_hash,digest_hash)
		 VALUES ('p1','a.go',1,10,'h','d'),('p1','b.go',1,10,'h','d')`,
	}
	for _, s := range stmts {
		_, err := conn.Exec(s)
		require.NoError(t, err)
	}
}

// Toda iteração de sql.Rows SHALL verificar rows.Err(): um erro durante a
// iteração não pode virar um resultado parcial sem erro.
func TestRepos_PropagateRowsIterationError(t *testing.T) {
	cases := []struct {
		name      string
		table     string
		poisonCol string
		// clearFileStates remove project_file_states antes de envenenar,
		// para exercitar o fallback legado de GetSymbolCountsByFile.
		clearFileStates bool
		call            func(db *storage.DB) error
	}{
		{"ProjectRepo.ListAll", "projects", "file_count", false, func(db *storage.DB) error {
			_, err := storage.NewProjectRepo(db).ListAll()
			return err
		}},
		{"SQLFileStateRepository.ListByProject", "project_file_states", "filepath", false, func(db *storage.DB) error {
			_, err := storage.NewSQLFileStateRepository(db.Conn()).ListByProject("p1")
			return err
		}},
		{"DataModelRepo.ListByProject", "data_models", "line_number", false, func(db *storage.DB) error {
			_, err := storage.NewDataModelRepo(db).ListByProject("p1")
			return err
		}},
		{"DataModelRepo.ListDataModels", "data_models", "line_number", false, func(db *storage.DB) error {
			_, _, err := storage.NewDataModelRepo(db).ListDataModels("p1", 50, 0)
			return err
		}},
		{"SymbolRepo.FindSymbol", "symbols", "start_line", false, func(db *storage.DB) error {
			_, _, err := storage.NewSymbolRepo(db).FindSymbol("p1", "Foo", 25, 0)
			return err
		}},
		{"SymbolRepo.FindReferences", "references_table", "line", false, func(db *storage.DB) error {
			_, _, err := storage.NewSymbolRepo(db).FindReferences("p1", "Foo", 30, 0)
			return err
		}},
		{"SymbolRepo.GetFileImportCounts", "dependency_graph", "source_file", false, func(db *storage.DB) error {
			_, err := storage.NewSymbolRepo(db).GetFileImportCounts("p1")
			return err
		}},
		{"SymbolRepo.GetSymbolReferenceCounts", "references_table", "symbol_name", false, func(db *storage.DB) error {
			_, err := storage.NewSymbolRepo(db).GetSymbolReferenceCounts("p1")
			return err
		}},
		{"SymbolRepo.GetAllSymbolsForRanking", "symbols", "start_line", false, func(db *storage.DB) error {
			_, err := storage.NewSymbolRepo(db).GetAllSymbolsForRanking("p1")
			return err
		}},
		{"SymbolRepo.GetSymbolsByFileAndLineRange", "symbols", "start_line", false, func(db *storage.DB) error {
			_, err := storage.NewSymbolRepo(db).GetSymbolsByFileAndLineRange("p1", "b.go", 1, 5)
			return err
		}},
		{"SymbolRepo.GetSymbolCountsByFile", "project_file_states", "filepath", false, func(db *storage.DB) error {
			_, err := storage.NewSymbolRepo(db).GetSymbolCountsByFile("p1")
			return err
		}},
		{"SymbolRepo.GetSymbolCountsByFile (fallback legado)", "symbols", "language", true, func(db *storage.DB) error {
			_, err := storage.NewSymbolRepo(db).GetSymbolCountsByFile("p1")
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			database, err := storage.NewDatabase(t.TempDir() + "/rows_err.db")
			require.NoError(t, err)
			defer database.Close()

			seedRowsErrFixture(t, database)
			if tc.clearFileStates {
				_, err := database.Conn().Exec(`DELETE FROM project_file_states`)
				require.NoError(t, err)
			}
			poisonTable(t, database, tc.table, tc.poisonCol)

			require.Error(t, tc.call(database), "erro de iteração deve ser propagado")
		})
	}
}
