package database

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool exposes the PostgreSQL pool to tests that drive raw SQL for setup and
// assertions. The Store no longer holds a driver handle directly - that moved behind
// the engine seam - so tests ask for it explicitly. It panics rather than returning an
// error so the existing call sites stay terse; these tests are PostgreSQL-only by
// nature (they pass DFO_TEST_POSTGRES_DSN) and fail loudly when it is missing.
func testPool(t *testing.T, s *Store) *pgxpool.Pool {
	t.Helper()
	pool, err := s.rawPool()
	if err != nil {
		t.Fatalf("test pool: %v", err)
	}
	return pool
}