//go:build sqlite

package main

import (
	"context"
	"dfolan/internal/database"
	"path/filepath"
	"testing"
	"time"
)

// The repository mirror predates the SQLite engine. Run this same auction
// contract with -tags sqlite in the current dual-engine server checkout.
func TestBakalAuctionSQLiteCardsGatewayAndResume(t *testing.T) {
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
	runBakalAuctionCardsGatewayAndResume(t, ctx, store)
}
