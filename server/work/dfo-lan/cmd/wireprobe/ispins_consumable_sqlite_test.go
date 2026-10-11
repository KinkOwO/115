//go:build sqlite

package main

import (
	"context"
	"dfolan/internal/database"
	"path/filepath"
	"testing"
	"time"
)

func TestIspinsPotionSQLiteAtomicLimitAndReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	store, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "potion.sqlite3"), MaxConnections: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runIspinsPotionAtomicLimitAndReplay(t, ctx, store)
}
