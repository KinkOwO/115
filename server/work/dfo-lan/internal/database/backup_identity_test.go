package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExistingSQLiteIdentityAliasesAndMissingFiles(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "自定义 玩家.sqlite3")
	alias := filepath.Join(dir, "alias.sqlite3")
	foreign := filepath.Join(dir, "foreign.sqlite3")
	for _, p := range []string{actual, foreign} {
		if err := os.WriteFile(p, []byte("same contents"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Link(actual, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := ExistingSQLitePath(actual, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := ExistingSQLitePath(actual, foreign); err == nil {
		t.Fatal("distinct same-content DB accepted")
	}
	missing := filepath.Join(dir, "missing.sqlite3")
	if _, err := ExistingSQLitePath(missing, ""); err == nil {
		t.Fatal("missing DB accepted")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("missing DB created")
	}
	if _, err := ExistingSQLitePath(dir, ""); err == nil {
		t.Fatal("directory accepted as DB")
	}
	for _, e := range []BackupEntry{{Database: alias}, {Source: SQLiteTarget(actual).Label}} {
		if err := VerifyBackupDatabase(e, actual); err != nil {
			t.Fatal(err)
		}
	}
	for _, e := range []BackupEntry{{Database: foreign}, {Source: SQLiteTarget(foreign).Label}, {}} {
		if err := VerifyBackupDatabase(e, actual); err == nil {
			t.Fatal("foreign/unbound backup accepted")
		}
	}
}
