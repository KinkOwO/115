package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestCloudDurabilityCoversEveryPooledAndReplacementConnection(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, Config{SQLitePath: filepath.Join(t.TempDir(), "full.db"), MaxConnections: 4, DurableWrites: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	db := s.engine.(*sqliteEngine).db
	connections := []*sql.Conn{}
	for range 4 {
		c, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		connections = append(connections, c)
		var mode int
		if err = c.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&mode); err != nil || mode != 2 {
			t.Fatalf("pool connection not FULL: %d %v", mode, err)
		}
	}
	for _, c := range connections {
		c.Close()
	}
	db.SetMaxIdleConns(0)
	c, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var mode int
	if err = c.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&mode); err != nil || mode != 2 {
		t.Fatalf("replacement connection lost durability: %d %v", mode, err)
	}
}
func TestCloudDurabilityOmittedPreservesLocalNormalMode(t *testing.T) {
	s, err := Open(context.Background(), Config{SQLitePath: filepath.Join(t.TempDir(), "local.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var mode int
	if err = s.engine.(*sqliteEngine).db.QueryRow("PRAGMA synchronous").Scan(&mode); err != nil || mode != 1 {
		t.Fatalf("local default changed: %d %v", mode, err)
	}
}
