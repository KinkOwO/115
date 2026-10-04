package database

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"dfolan/internal/database/sqlcgen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// errAdminGuardBusy is what both engines report when another session holds the shared
// administrative guard.
var errAdminGuardBusy = errors.New("已有 GM 写入正在进行，请稍后重试")

// engine is the persistence seam: the query surface plus the only things that
// genuinely differ per engine - how a transaction is opened, and how the connection
// is released. Everything above it (the Store and, through it, every domain service)
// is written once against querySet and never learns which engine is underneath.
//
// begin returns the manual transaction form the store already uses throughout
// (begin, defer rollback, commit); inTx wraps it for callers that prefer a callback.
// An earlier design put WithTx on the shared interface, which cannot work: it hands
// back a transaction handle (pgx.Tx versus *sql.Tx), so no single signature describes
// it - confining that difference here keeps every call site engine-neutral.
type engine interface {
	queries() querySet
	begin(ctx context.Context) (txHandle, error)
	holdAdminGuard(ctx context.Context) (func(), error)
	close() error
}

// txHandle is a transaction-bound query surface. commit and rollback take a context
// for symmetry with pgx; database/sql's versions take none.
type txHandle interface {
	queries() querySet
	// exec runs raw SQL. The migration runner needs it: DDL has no generated query,
	// and it must run on the same transaction as the ledger write.
	exec(ctx context.Context, statement string) error
	commit(ctx context.Context) error
	rollback(ctx context.Context) error
}

// inTx runs fn in a transaction: commit on success, rollback on error.
func inTx(ctx context.Context, e engine, fn func(txHandle) error) error {
	tx, err := e.begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.rollback(ctx)
		return err
	}
	return tx.commit(ctx)
}


// rawPool returns the PostgreSQL pool behind the engine, or a clear error when the
// engine has none. Only the test fixture and the diagnostics exception use it; both
// are PostgreSQL-only by design (sec.3.4), and they must say so rather than silently
// doing nothing on SQLite.
func (s *Store) rawPool() (*pgxpool.Pool, error) {
	if access, ok := s.engine.(poolAccess); ok {
		return access.rawPool(), nil
	}
	return nil, errPostgresOnly
}

// errPostgresOnly marks the operations that deliberately have no SQLite equivalent.
var errPostgresOnly = errors.New("this operation requires the PostgreSQL engine")
// poolAccess is implemented by engines that can hand out their raw PostgreSQL pool.
// Only the test fixture uses it, to create and drop an isolated schema; a SQLite
// deployment isolates with its own database file instead (D18), so the fixture
// reports that it needs PostgreSQL rather than pretending otherwise.
type poolAccess interface {
	rawPool() *pgxpool.Pool
}

// ---------------------------------------------------------------------------
// PostgreSQL
// ---------------------------------------------------------------------------

type postgresEngine struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func newPostgresEngine(pool *pgxpool.Pool) *postgresEngine {
	return &postgresEngine{pool: pool, q: sqlcgen.New(pool)}
}

func (e *postgresEngine) queries() querySet { return e.q }

func (e *postgresEngine) begin(ctx context.Context) (txHandle, error) {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &postgresTx{tx: tx, q: e.q.WithTx(tx)}, nil
}

func (e *postgresEngine) rawPool() *pgxpool.Pool { return e.pool }

func (e *postgresEngine) close() error {
	e.pool.Close()
	return nil
}

type postgresTx struct {
	tx pgx.Tx
	q  *sqlcgen.Queries
}

func (t *postgresTx) queries() querySet { return t.q }
func (t *postgresTx) exec(ctx context.Context, statement string) error {
	_, err := t.tx.Exec(ctx, statement)
	return err
}
func (t *postgresTx) commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *postgresTx) rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

// holdAdminGuard keeps a dedicated connection holding the shared advisory lock, which
// is also what releases it: the lock lives with the connection, so closing that
// connection on release (or on process death) frees the guard with no bookkeeping.
func (e *postgresEngine) holdAdminGuard(ctx context.Context) (func(), error) {
	conn, err := e.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	ok, err := sqlcgen.New(conn).TrySharedAdminGuard(ctx)
	if err != nil || !ok {
		conn.Release()
		if err != nil {
			return nil, err
		}
		return nil, errAdminGuardBusy
	}
	var once sync.Once
	return func() {
		// Releasing twice would return the same pooled connection twice.
		if !onceAllowed(&once) {
			return
		}
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = conn.Conn().Close(closeCtx)
		conn.Release()
	}, nil
}

// ---------------------------------------------------------------------------
// SQLite
// ---------------------------------------------------------------------------

type sqliteEngine struct {
	db *sql.DB
	q  *sqliteQueries
}

func newSQLiteEngine(db *sql.DB) *sqliteEngine {
	return &sqliteEngine{db: db, q: newSQLiteQueries(db)}
}

func (e *sqliteEngine) queries() querySet { return e.q }

// begin relies on the DSN's _txlock=immediate: BEGIN IMMEDIATE is the writer
// serialization PostgreSQL row locks provided, so a transaction means the same thing
// to a caller either way.
func (e *sqliteEngine) begin(ctx context.Context) (txHandle, error) {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, storageError(err)
	}
	return &sqliteTx{tx: tx, q: newSQLiteQueriesTx(tx)}, nil
}

func (e *sqliteEngine) close() error { return e.db.Close() }

type sqliteTx struct {
	tx *sql.Tx
	q  *sqliteQueries
}

func (t *sqliteTx) queries() querySet { return t.q }
func (t *sqliteTx) exec(ctx context.Context, statement string) error {
	_, err := t.tx.ExecContext(ctx, statement)
	return err
}
func (t *sqliteTx) commit(ctx context.Context) error {
	return t.tx.Commit()
}
func (t *sqliteTx) rollback(ctx context.Context) error { return t.tx.Rollback() }

// holdAdminGuard uses the lease file the adapter implements (design doc sec.3.3);
// releasing removes it, which is the equivalent of PostgreSQL dropping the advisory
// lock when the connection closes.
func (e *sqliteEngine) holdAdminGuard(ctx context.Context) (func(), error) {
	ok, err := e.q.TrySharedAdminGuard(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errAdminGuardBusy
	}

	// Keep the lease fresh for as long as the guard is held. Without this the lease would
	// expire beneath a live holder and a second administrator could take it - which is why
	// the TTL and the refresh interval are defined together in the adapter.
	stop := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(AdminLeaseRefresh)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				_ = e.q.RefreshAdminGuardLease(context.Background())
			}
		}
	}()

	var once sync.Once
	return func() {
		// Idempotent: a caller that releases twice must not panic, and a lease that is
		// already gone must not be recreated.
		once.Do(func() {
			close(stop)
			<-finished
			_ = e.q.ReleaseAdminGuardLease(context.Background())
		})
	}, nil
}

// onceAllowed runs the guarded body exactly once, for releases that must not repeat.
func onceAllowed(once *sync.Once) bool {
	allowed := false
	once.Do(func() { allowed = true })
	return allowed
}
