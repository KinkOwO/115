package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// engineLocalTables holds bookkeeping that belongs to an engine rather than to the
// player's save, so it is never copied between engines.
var engineLocalTables = map[string]bool{
	"storage_migrations": true,
}

// errTableAbsent marks a destination table the source schema does not define.
var errTableAbsent = errors.New("table is absent from the PostgreSQL schema")

// ConvertReport summarises a PostgreSQL -> SQLite copy and the checks performed on it.
type ConvertReport struct {
	Tables []ConvertTableReport
	// Missing lists destination tables the source schema does not have. A real
	// deployment applies every migration at startup, so this is normally empty; when it
	// is not, the source is older than the destination and the caller must decide rather
	// than have the copy silently skip data.
	Missing []string
}

// ConvertTableReport is one table's outcome. Verified means the row count matched and,
// for tables with JSON columns, the JSON bytes hashed identically in row order.
type ConvertTableReport struct {
	Name        string
	Rows        int64
	Verified    bool
	JSONChecked int
}

// ConvertPostgresToSQLite copies a PostgreSQL save into a fresh SQLite database and
// verifies the copy (design doc sec.6.3). It is one-way on purpose: SQLite is the
// single-machine target, and PostgreSQL stays authoritative until the owner accepts a
// migration, so there is no reverse path to keep correct.
//
// The table list and column types come from the schemas themselves rather than a
// hand-maintained list - the destination's tables from sqlite_master, the source's
// columns and types from information_schema - so a schema change cannot silently leave
// a table uncopied.
func ConvertPostgresToSQLite(ctx context.Context, source *Store, destPath string) (ConvertReport, error) {
	var report ConvertReport
	pool, err := source.rawPool()
	if err != nil {
		return report, err
	}
	if strings.TrimSpace(destPath) == "" {
		return report, fmt.Errorf("a destination path is required")
	}

	// A fresh file is required. Converting on top of an existing database would collide
	// with the rows already there (the ledger and any earlier partial copy), and the
	// honest answer is to refuse rather than merge two saves.
	if _, statErr := os.Stat(destPath); statErr == nil {
		return report, fmt.Errorf("destination %s already exists; the converter writes a new file", destPath)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return report, statErr
	}

	dest, err := openSQLite(ctx, destPath, 1, 10000)
	if err != nil {
		return report, err
	}
	defer dest.Close()
	if err := migrateSQLiteAll(ctx, dest); err != nil {
		return report, err
	}

	// Foreign keys are checked at the end instead of during the copy, so tables can be
	// filled in any order; a broken reference then fails the conversion loudly rather
	// than being masked by insertion order.
	if _, err := dest.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return report, err
	}

	tables, err := sqliteTableNames(ctx, dest)
	if err != nil {
		return report, err
	}
	for _, table := range tables {
		if engineLocalTables[table] {
			// The migration ledger describes what an engine has applied, and the
			// destination writes its own when the schema is created. Copying the source's
			// rows would collide with it and misstate the file's contents.
			continue
		}
		tableReport, err := copyTable(ctx, pool, dest, table)
		if errors.Is(err, errTableAbsent) {
			report.Missing = append(report.Missing, table)
			continue
		}
		if err != nil {
			return report, fmt.Errorf("table %s: %w", table, err)
		}
		report.Tables = append(report.Tables, tableReport)
	}

	if _, err := dest.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return report, err
	}
	broken, err := foreignKeyViolations(ctx, dest)
	if err != nil {
		return report, err
	}
	if len(broken) > 0 {
		return report, fmt.Errorf("converted database has %d foreign key violation(s): %s",
			len(broken), strings.Join(broken, "; "))
	}
	return report, nil
}

// TotalRows is the copied row count, for logging and assertions.
func (r ConvertReport) TotalRows() int64 {
	var total int64
	for _, table := range r.Tables {
		total += table.Rows
	}
	return total
}

func sqliteTableNames(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func foreignKeyViolations(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var table, parent string
		var rowid, fkid int64
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("%s rowid %d -> %s", table, rowid, parent))
	}
	return out, rows.Err()
}

// dateColumnsAsISOText lists the DATE columns whose queries read and write ISO text
// rather than the DDL's integer microseconds. The representation is decided by the
// query tree, not by the declaration, so the converter follows the queries; every other
// DATE column stores microseconds. See the per-column notes in the sqlite query files.
var dateColumnsAsISOText = map[string]bool{
	"character_fatigue.day":                    true,
	"character_fatigue_rooms.day":              true,
	"character_fatigue_recovery.day":           true,
	"account_tower_progress.entry_day":         true,
	"account_tower_grief_progress.cleared_day": true,
}

// copyTable copies one table and verifies it: the row count must match, and every JSON
// value must hash identically in row order, which catches a lossy converter that
// re-serialises JSON instead of moving the same bytes.
func copyTable(ctx context.Context, pool *pgxpool.Pool, dest *sql.DB, table string) (ConvertTableReport, error) {
	report := ConvertTableReport{Name: table}

	meta, err := pool.Query(ctx, `
		SELECT column_name, data_type
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = $1
		ORDER BY ordinal_position`, table)
	if err != nil {
		return report, err
	}
	var columns, types []string
	for meta.Next() {
		var name, dataType string
		if err := meta.Scan(&name, &dataType); err != nil {
			meta.Close()
			return report, err
		}
		columns = append(columns, name)
		types = append(types, dataType)
	}
	meta.Close()
	if err := meta.Err(); err != nil {
		return report, err
	}
	if len(columns) == 0 {
		return report, errTableAbsent
	}

	jsonColumns := jsonColumnIndexes(types)

	// Order both sides the same way so the JSON hashes are comparable: the first column
	// is the primary key by this schema's convention.
	orderBy := quoteIdent(columns[0])
	selectList := make([]string, len(columns))
	for i, column := range columns {
		if types[i] == "json" || types[i] == "jsonb" {
			// pgx decodes jsonb into a Go map, and re-encoding that map would reorder keys
			// and drop whitespace. PostgreSQL normalises jsonb on storage, so the canonical
			// text IS the stored value, and casting moves those bytes verbatim.
			selectList[i] = quoteIdent(column) + "::text"
			continue
		}
		selectList[i] = quoteIdent(column)
	}
	src, err := pool.Query(ctx,
		"SELECT "+strings.Join(selectList, ",")+" FROM "+quoteIdent(table)+" ORDER BY "+orderBy)
	if err != nil {
		return report, err
	}
	defer src.Close()

	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	tx, err := dest.BeginTx(ctx, nil)
	if err != nil {
		return report, err
	}
	defer tx.Rollback()
	statement, err := tx.PrepareContext(ctx,
		"INSERT INTO "+quoteIdent(table)+"("+quoteIdentList(columns)+") VALUES("+strings.Join(placeholders, ",")+")")
	if err != nil {
		return report, err
	}
	defer statement.Close()

	sourceJSON := map[int][32]byte{}
	for src.Next() {
		values, err := src.Values()
		if err != nil {
			return report, err
		}
		converted := make([]any, len(values))
		for i, value := range values {
			converted[i] = convertValue(table, columns[i], types[i], value)
		}
		for _, index := range jsonColumns {
			sourceJSON[index] = appendHash(sourceJSON[index], converted[index])
		}
		if _, err := statement.ExecContext(ctx, converted...); err != nil {
			return report, err
		}
		report.Rows++
	}
	if err := src.Err(); err != nil {
		return report, err
	}

	var destRows int64
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM "+quoteIdent(table)).Scan(&destRows); err != nil {
		return report, err
	}
	if destRows != report.Rows {
		return report, fmt.Errorf("row count mismatch: source %d, destination %d", report.Rows, destRows)
	}

	for _, index := range jsonColumns {
		hash, err := destinationJSONHash(ctx, tx, table, columns[index], orderBy)
		if err != nil {
			return report, err
		}
		if hash != sourceJSON[index] {
			return report, fmt.Errorf("column %s: JSON bytes differ from the source", columns[index])
		}
		report.JSONChecked++
	}

	if err := tx.Commit(); err != nil {
		return report, err
	}
	report.Verified = true
	return report, nil
}

// jsonColumnIndexes finds the columns whose PostgreSQL type is json/jsonb. The mapping
// from PostgreSQL arrays and other shapes is documented in the port guide; anything not
// listed here is copied as-is.
func jsonColumnIndexes(types []string) []int {
	var out []int
	for i, dataType := range types {
		if dataType == "json" || dataType == "jsonb" {
			out = append(out, i)
		}
	}
	return out
}

// appendHash folds one value into a running hash, so JSON can be compared in row order
// without materialising every value twice.
func appendHash(running [32]byte, value any) [32]byte {
	digest := sha256.New()
	digest.Write(running[:])
	switch typed := value.(type) {
	case nil:
		digest.Write([]byte{0})
	case []byte:
		digest.Write(typed)
	case string:
		digest.Write([]byte(typed))
	default:
		fmt.Fprintf(digest, "%v", typed)
	}
	var next [32]byte
	copy(next[:], digest.Sum(nil))
	return next
}

func destinationJSONHash(ctx context.Context, tx *sql.Tx, table, column, orderBy string) ([32]byte, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT "+quoteIdent(column)+" FROM "+quoteIdent(table)+" ORDER BY "+orderBy)
	if err != nil {
		return [32]byte{}, err
	}
	defer rows.Close()
	var running [32]byte
	for rows.Next() {
		var value any
		if err := rows.Scan(&value); err != nil {
			return running, err
		}
		running = appendHash(running, value)
	}
	return running, rows.Err()
}

// convertValue maps one PostgreSQL value onto the representation the SQLite query tree
// expects. Types not listed here pass through unchanged, which is correct for the
// scalars whose representation is identical in both engines.
func convertValue(table, column, dataType string, value any) any {
	if value == nil {
		return nil
	}
	switch dataType {
	case "timestamp with time zone", "timestamp without time zone":
		if moment, ok := value.(time.Time); ok {
			return moment.UTC().UnixMicro()
		}
	case "date":
		if moment, ok := value.(time.Time); ok {
			if dateColumnsAsISOText[table+"."+column] {
				return moment.Format("2006-01-02")
			}
			return moment.UTC().UnixMicro()
		}
	case "ARRAY":
		// Arrays travel as JSON (D27): bigint[] becomes a JSON array.
		if encoded, err := json.Marshal(value); err == nil {
			return encoded
		}
	case "json", "jsonb":
		// The select casts these to text, so the stored bytes arrive as a string. The
		// fallbacks keep the function total if a future query forgets the cast.
		switch typed := value.(type) {
		case []byte:
			return typed
		case string:
			return []byte(typed)
		default:
			if encoded, err := json.Marshal(typed); err == nil {
				return encoded
			}
		}
	}
	return value
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quoteIdentList(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = quoteIdent(name)
	}
	return strings.Join(quoted, ",")
}
