package database

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGMDirectoryReadsLiveWALWithoutSaveWrites(t *testing.T) {
	ctx := context.Background()
	cfg := Config{SQLitePath: filepath.Join(t.TempDir(), "live.db"), MaxConnections: 2}
	s, e := Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a, e := s.DevelopmentAccount(ctx, "alice")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DevelopmentAccount(ctx, "empty_account"); e != nil {
		t.Fatal(e)
	}
	role, e := s.CreateCharacter(ctx, Character{AccountID: a, Name: "SavedRole", Profession: 2, Request: []byte{}, ConfigVersion: strings.Repeat("a", 64), State: json.RawMessage(`{"level":115,"secret_save_field":"never expose"}`)}, 100)
	if e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(cfg.SQLitePath)
	if e != nil {
		t.Fatal(e)
	}
	// The writer remains open, with its save lease held, and changes are in WAL.
	rows, e := ReadGMDirectory(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 2 || rows[0].Username != "alice" || len(rows[0].Characters) != 1 || rows[0].Characters[0].ID != role.ID || rows[0].Characters[0].Level != 115 || len(rows[1].Characters) != 0 {
		t.Fatalf("directory=%+v", rows)
	}
	b, _ := json.Marshal(rows)
	if strings.Contains(string(b), "secret_save_field") || strings.Contains(string(b), "password") {
		t.Fatal("private data exposed")
	}
	after, e := os.ReadFile(cfg.SQLitePath)
	if e != nil || string(before) != string(after) {
		t.Fatal("directory wrote the database")
	}
	db, e := openGMDirectory(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.ExecContext(ctx, "UPDATE accounts SET username='corrupted' WHERE id=?", a); e == nil {
		t.Fatal("read-only directory permitted a write")
	}
	rows, e = ReadGMDirectory(ctx, cfg)
	if e != nil || rows[0].Username != "alice" {
		t.Fatal("write refusal changed saved account")
	}
}

func TestGMDirectoryDoesNotCreateMissingDatabase(t *testing.T) {
	p := filepath.Join(t.TempDir(), "missing.db")
	if _, e := ReadGMDirectory(context.Background(), Config{SQLitePath: p}); e == nil {
		t.Fatal("missing store accepted")
	}
	if _, e := os.Stat(p); !os.IsNotExist(e) {
		t.Fatal("missing store was created")
	}
}
