package main

import (
	"dfolan/internal/database"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStorageSyncUsesConfiguredDatabaseAndSeparateSources(t *testing.T) {
	root, _ := storageRootWithSQLite(t)
	custom := filepath.Join(t.TempDir(), "自定义.sqlite3")
	if err := os.WriteFile(custom, []byte(testSQLiteHeader), 0600); err != nil {
		t.Fatal(err)
	}
	profile, _ := json.Marshal(map[string]string{"driver": "sqlite", "sqlite_path": custom})
	writeStorageFile(t, filepath.Join(storageSyncDir(root), "local.json"), string(profile))
	for _, source := range []string{"storage", "gm-local"} {
		result := storageSyncResult{Action: "list"}
		if err := storageSyncRun(root, &result, storageSyncOptions{Source: source, ExpectedDatabase: custom}); err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(filepath.Dir(custom), database.BackupDirName)
		if source == "gm-local" {
			want = filepath.Join(filepath.Dir(custom), "gm-backups", "local")
		}
		if result.Database != custom || result.BackupsDir != want || result.Source != source {
			t.Fatal(result)
		}
	}
	if err := storageSyncRun(root, &storageSyncResult{Action: "list"}, storageSyncOptions{Source: "../foreign"}); err == nil {
		t.Fatal("arbitrary backup path accepted")
	}
	foreign := filepath.Join(t.TempDir(), "foreign.sqlite3")
	if err := os.WriteFile(foreign, []byte(testSQLiteHeader), 0600); err != nil {
		t.Fatal(err)
	}
	if err := storageSyncRun(root, &storageSyncResult{Action: "list"}, storageSyncOptions{ExpectedDatabase: foreign}); err == nil {
		t.Fatal("foreign expected DB accepted")
	}
}
