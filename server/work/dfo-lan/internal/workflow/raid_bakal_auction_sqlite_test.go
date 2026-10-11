//go:build sqlite

package workflow

import (
	"context"
	"dfolan/internal/database"
	"path/filepath"
	"testing"
	"time"
)

// The repository mirror predates the SQLite engine. Run this same auction
// contract with -tags sqlite in the current dual-engine server checkout.
func TestBakalAuctionSQLiteConcurrentReplayAndRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	store, err := database.Open(ctx, database.Config{
		Driver:         database.DriverSQLite,
		SQLitePath:     filepath.Join(t.TempDir(), "auction.sqlite3"),
		MaxConnections: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	runBakalAuctionConcurrentReplayAndRestart(t, ctx, store)
}
