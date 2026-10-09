package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dfolan/internal/database/sqlcgensqlite"
)

// The five methods below have no counterpart in the generated SQLite query tree,
// because PostgreSQL implements them with facilities SQLite does not have: catalogs
// (DatabaseName, FixtureSchema), a sequence (NextMailID) and advisory locks
// (TrySharedAdminGuard, LockMigrations). Each is implemented with the SQLite
// equivalent and the reasoning recorded here rather than assumed - design doc
// sec.3.5 lists them as the engine-specific remainder.

// newSQLiteQueries binds the generated query set to a pool handle.
func newSQLiteQueries(db *sql.DB) *sqliteQueries {
	return &sqliteQueries{Queries: sqlcgensqlite.New(db), db: db}
}

// newSQLiteQueriesTx binds it to a transaction instead, so the engine seam can hand
// callers the same querySet inside and outside a transaction.
func newSQLiteQueriesTx(tx *sql.Tx) *sqliteQueries {
	return &sqliteQueries{Queries: sqlcgensqlite.New(tx), tx: tx}
}

// queryRow and exec route to the live handle, which is what keeps the five
// engine-specific methods usable in both contexts.
func (q *sqliteQueries) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	if q.tx != nil {
		return q.tx.QueryRowContext(ctx, query, args...)
	}
	return q.db.QueryRowContext(ctx, query, args...)
}

func (q *sqliteQueries) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if q.tx != nil {
		return q.tx.ExecContext(ctx, query, args...)
	}
	return q.db.ExecContext(ctx, query, args...)
}

// DatabaseName: PostgreSQL reads current_database(); on SQLite the equivalent is the
// attached file of the main database. This cannot live in the .sql tree because
// sqlc's schema model does not know pragma virtual tables - verified by trying, the
// generator reports `relation "pragma_database_list" does not exist`.
func (q *sqliteQueries) DatabaseName(ctx context.Context) (string, error) {
	var name string
	if err := q.queryRow(ctx,
		"SELECT file FROM pragma_database_list WHERE name='main'").Scan(&name); err != nil {
		return "", storageError(err)
	}
	return name, nil
}

// AccountSlotBonus reads the account-level character-slot bonus granted by the
// cash-shop Character Slot Extension Kit (takes effect on purchase). The column
// is added to existing saves by the 0039 migration section; a missing column
// here is a real error.
func (q *sqliteQueries) AccountSlotBonus(ctx context.Context, accountID int64) (int32, error) {
	var bonus int32
	if err := q.queryRow(ctx,
		"SELECT character_slots_bonus FROM accounts WHERE id = ?", accountID).Scan(&bonus); err != nil {
		return 0, storageError(err)
	}
	return bonus, nil
}

// FixtureSchema: PostgreSQL reports current_schema() and a fixture isolates itself in
// a temporary schema. SQLite has no schemas; D18 isolates a fixture with its own
// database file instead, so the single schema name is the truthful answer.
func (q *sqliteQueries) FixtureSchema(ctx context.Context) (string, error) {
	return "main", nil
}

// LockMigrations: PostgreSQL takes a transaction-scoped advisory lock. SQLite already
// provides exactly that guarantee - the DSN sets _txlock=immediate, so the migration
// transaction takes the write lock up front and releases it on commit, rollback, or
// process death. There is no statement to run, and adding one would create a lock that
// cannot fail.
func (q *sqliteQueries) LockMigrations(ctx context.Context) error {
	return nil
}

// NextMailID: PostgreSQL allocates from mailbox_id_seq, and callers allocate
// attachment ids BEFORE inserting the message, so attachments and the message share
// one number space. AUTOINCREMENT alone cannot reproduce that (it allocates at
// INSERT), but advancing sqlite_sequence can: that value is the very high-water mark
// the allocator consults, and it is writable by design. Running it in a transaction
// keeps the read-modify-write atomic under BEGIN IMMEDIATE.
func (q *sqliteQueries) NextMailID(ctx context.Context) (int64, error) {
	// Inside a transaction the caller owns commit and rollback, so the bump runs on
	// that transaction directly; otherwise it gets one of its own.
	if q.tx != nil {
		return advanceMailSequence(ctx, q.tx)
	}
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, storageError(err)
	}
	defer tx.Rollback()
	id, err := advanceMailSequence(ctx, tx)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// advanceMailSequence performs the atomic read-modify-write described on NextMailID.
func advanceMailSequence(ctx context.Context, tx *sql.Tx) (int64, error) {
	res, err := tx.ExecContext(ctx,
		"UPDATE sqlite_sequence SET seq = seq + 1 WHERE name = 'character_mail'")
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if affected == 0 {
		// No row yet: the table has never been written, so open the shared space.
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO sqlite_sequence(name, seq) VALUES('character_mail', 1)"); err != nil {
			return 0, err
		}
	}
	var id int64
	if err := tx.QueryRowContext(ctx,
		"SELECT seq FROM sqlite_sequence WHERE name = 'character_mail'").Scan(&id); err != nil {
		return 0, err
	}
	if err := alignMailSequenceTable(ctx, tx, id); err != nil {
		return 0, err
	}
	return id, nil
}

// alignMailSequenceTable keeps the 20261004 upgrade package's counter in step with ours.
//
// That package stores the same number space in mailbox_id_sequence instead of relying on
// sqlite_sequence. A save it created therefore carries that table, and if this
// implementation advanced only its own counter the two would drift: each would hand out an
// id the other had already used. Aligning is a no-op on a save without the table, so it
// costs nothing for databases this implementation created itself.
func alignMailSequenceTable(ctx context.Context, tx *sql.Tx, id int64) error {
	var present int
	if err := tx.QueryRowContext(ctx,
		"SELECT count(*) FROM sqlite_master WHERE type='table' AND name='mailbox_id_sequence'").
		Scan(&present); err != nil {
		return err
	}
	if present == 0 {
		return nil
	}
	// Never move the other counter backwards: it may already be ahead of us.
	_, err := tx.ExecContext(ctx,
		"UPDATE mailbox_id_sequence SET value = ? WHERE name = 'mailbox_id_seq' AND value < ?",
		id+1, id+1)
	return err
}

// TrySharedAdminGuard: PostgreSQL takes a shared advisory lock that the launcher takes
// exclusively for administrative writes, and the server releases it automatically when
// the holding connection closes. SQLite has no advisory locks, so the equivalent is a
// lease file beside the database (design doc sec.3.3): holding it IS the guard, and the
// launcher (W2) uses the same path so both sides coordinate.
//
// Deliberately NOT implemented here: reclaiming a lease whose owner died. Detecting a
// live process portably is not something the standard library offers (on Windows
// FindProcess succeeds for any pid), and a wrong "it is dead" answer would let two
// administrators write at once - the exact thing the guard exists to prevent. Instead
// the lease records its holder and the error names the file, so an operator can clear a
// lease left by a crash. The crash-safe replacement (a lease row with a heartbeat) is
// the §3.3 follow-up and is tracked in the design doc.
// AdminLeaseTTL bounds how long an unrefreshed lease is trusted. The holder refreshes
// every AdminLeaseRefresh, so the margin is six refreshes: long enough that a busy or
// suspended holder is never mistaken for a dead one, short enough that a crash does not
// wedge administrative writes until someone notices.
const (
	AdminLeaseTTL     = 60 * time.Second
	AdminLeaseRefresh = 10 * time.Second
)

// TrySharedAdminGuard: PostgreSQL takes a shared advisory lock that the launcher takes
// exclusively for administrative writes, and the database releases it when the holding
// connection dies. SQLite has no advisory locks, so the equivalent is a lease file beside
// the database (design doc sec.3.3) whose holder refreshes its mtime: holding it IS the
// guard, and a lease nobody has refreshed within AdminLeaseTTL is abandoned and would
// otherwise leave GM writes refused until an operator intervened.
func (q *sqliteQueries) TrySharedAdminGuard(ctx context.Context) (bool, error) {
	path, err := q.adminGuardPath(ctx)
	if err != nil {
		return false, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, writeErr := file.WriteString(strconv.Itoa(os.Getpid()))
			closeErr := file.Close()
			if writeErr != nil {
				return false, writeErr
			}
			return true, closeErr
		}
		if !errors.Is(err, os.ErrExist) {
			return false, err
		}

		info, statErr := os.Stat(path)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				// The holder released between our create and our stat; try again.
				continue
			}
			return false, statErr
		}
		if age := time.Since(info.ModTime()); age < AdminLeaseTTL {
			holder, _ := os.ReadFile(path)
			return false, fmt.Errorf(
				"已有 GM 写入正在进行（SQLite 管理租约文件 %s 由进程 %s 持有，最近一次续租 %s 前）；若确认没有 GM 在写，删除该文件后重试",
				path, strings.TrimSpace(string(holder)), age.Round(time.Second))
		}
		// Abandoned. Removing first means the retry's O_EXCL create is still the single
		// point of exclusion: if two processes race here, only one create can win.
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return false, removeErr
		}
	}
	return false, nil
}

// RefreshAdminGuardLease keeps a held lease from expiring. It is a no-op when the file is
// gone, so a holder that already released does not resurrect the lease.
func (q *sqliteQueries) RefreshAdminGuardLease(ctx context.Context) error {
	path, err := q.adminGuardPath(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

// ReleaseAdminGuardLease gives the lease up.
func (q *sqliteQueries) ReleaseAdminGuardLease(ctx context.Context) error {
	path, err := q.adminGuardPath(ctx)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// adminGuardPath derives the lease path from the main database file, so a database and
// its guard cannot drift apart.
func (q *sqliteQueries) adminGuardPath(ctx context.Context) (string, error) {
	name, err := q.DatabaseName(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(name) == "" || name == ":memory:" {
		return filepath.Join(os.TempDir(), fmt.Sprintf("dfolan-admin-guard-%d", os.Getpid())), nil
	}
	return name + ".admin-guard", nil
}

// Compile-time proof that the SQLite adapter satisfies the same engine-neutral
// surface the PostgreSQL package satisfies directly.
var _ querySet = (*sqliteQueries)(nil)
