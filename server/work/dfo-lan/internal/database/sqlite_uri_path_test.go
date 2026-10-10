package database

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLiteDSNPreservesLiteralURICharactersInPath(t *testing.T) {
	for _, name := range []string{"中文 空格", "literal %25 #", "literal %23 #", "100% 完整"} {
		path := filepath.Join(t.TempDir(), name, "角色 %25 #.sqlite3")
		u, err := url.Parse(sqliteDSN(path, 5000))
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.ToSlash(path)
		if !strings.HasPrefix(want, "/") {
			want = "/" + want
		}
		if u.Scheme != "file" || u.Path != want || u.Fragment != "" || u.Query().Get("_busy_timeout") != "5000" {
			t.Fatalf("literal filename changed: %+v want %q", u, want)
		}
	}
}

func TestSQLiteSpecialPathsPreserveMarkedDataAcrossOpenBackupRestoreAndGM(t *testing.T) {
	ctx := context.Background()
	seed := filepath.Join(t.TempDir(), "seed.sqlite3")
	store, err := Open(ctx, Config{Driver: DriverSQLite, SQLitePath: seed})
	if err != nil {
		t.Fatal(err)
	}
	account, err := store.DevelopmentAccount(ctx, "marked-account")
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, Character{AccountID: account, Name: "MarkedRole", Profession: 2, Request: []byte{}, ConfigVersion: strings.Repeat("a", 64), State: json.RawMessage(`{"level":115,"marker":"literal path must preserve me"}`)}, 100)
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	store.Close()
	seedBytes, err := os.ReadFile(seed)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"中文 空格", "目录 %25 #", "目录 %23 #", "目录 100%"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, name)
			if err := os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, "角色 %25 #.sqlite3")
			if err := os.WriteFile(path, seedBytes, 0600); err != nil {
				t.Fatal(err)
			}
			assertMarked := func() {
				s, err := Open(ctx, Config{Driver: DriverSQLite, SQLitePath: path})
				if err != nil {
					t.Fatal(err)
				}
				actual, err := s.DatabaseName(ctx)
				if err != nil {
					s.Close()
					t.Fatal(err)
				}
				a, e1 := os.Stat(path)
				b, e2 := os.Stat(actual)
				if e1 != nil || e2 != nil || !os.SameFile(a, b) {
					s.Close()
					t.Fatalf("engine opened %s instead of %s", actual, path)
				}
				r, err := s.AdminCharacter(ctx, role.ID)
				s.Close()
				if err != nil || !strings.Contains(string(r.State), "literal path must preserve me") {
					t.Fatal("marked character lost", r, err)
				}
				rows, err := ReadGMDirectory(ctx, Config{Driver: DriverSQLite, SQLitePath: path})
				if err != nil || len(rows) != 1 || rows[0].Username != "marked-account" || len(rows[0].Characters) != 1 || rows[0].Characters[0].ID != role.ID {
					t.Fatal("directory data differs", rows, err)
				}
			}
			assertMarked()
			backups := filepath.Join(directory, "备份 %23 #")
			entry, err := Backup(ctx, SQLiteTarget(path), backups, nil)
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string]int64{}
			for _, row := range entry.Tables {
				counts[row.Name] = row.Rows
			}
			if counts["accounts"] != 1 || counts["characters"] != 1 {
				t.Fatal("backup lost marked rows", counts)
			}
			if err := VerifyBackup(entry); err != nil {
				t.Fatal(err)
			}
			execOnSQLite(t, path, "INSERT INTO accounts(username) VALUES('changed-after-backup')")
			if countAccounts(t, path) != 2 {
				t.Fatal("mutation fixture failed")
			}
			for i := 0; i < 2; i++ {
				report, err := RestoreBackup(ctx, entry, SQLiteTarget(path), backups, nil)
				if err != nil || !report.Verified || report.TargetBackup == nil {
					t.Fatal("restore was not verified/backed up", report, err)
				}
				assertMarked()
			}
			// No URI-decoded or fragment-truncated sibling database was created.
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 || entries[0].Name() != name {
				t.Fatal("unexpected sibling created by URI interpretation", entries, err)
			}
		})
	}
}
