package database

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
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
// back a transaction handle whose type differs per engine, so no single signature
// describes it - confining that difference here keeps every call site engine-neutral.
type engine interface {
	queries() querySet
	begin(ctx context.Context) (txHandle, error)
	holdAdminGuard(ctx context.Context) (func(), error)
	close() error
}

// txHandle is a transaction-bound query surface. commit and rollback take a context
// so the interface stays stable even though database/sql's versions take none.
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
// releasing removes it. SQLite is the only engine since 2026-10-05 (owner decision, see
// root AGENTS.md §0.6), so this is the only guard implementation there is: the
// PostgreSQL advisory-lock variant went with the engine.
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
