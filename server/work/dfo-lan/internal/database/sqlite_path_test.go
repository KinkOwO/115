package database

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// A relative sqlite_path must be refused rather than resolved against the server's working
// directory. The launcher starts the server with cwd=<server/>, so a relative path would
// create the save in a different file from the one the configuration appears to name, and
// nothing would report it.
func TestSQLiteRelativePathIsRefused(t *testing.T) {
	_, err := Open(context.Background(), Config{
		Driver:     DriverSQLite,
		SQLitePath: filepath.Join("runtime", "storage", "game.db"),
	})
	if err == nil {
		t.Fatal("a relative sqlite_path was accepted")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("error %q does not explain that an absolute path is required", err)
	}

	// The implicit form (no driver, sqlite_path present) must refuse it too, since that is
	// the shape the upgrade package's migration tool writes.
	if _, err := Open(context.Background(), Config{SQLitePath: "game.db"}); err == nil {
		t.Error("the implicit form accepted a relative path")
	}

	// An absolute path still works, which is how every real configuration and test writes it.
	store, err := Open(context.Background(), Config{SQLitePath: filepath.Join(t.TempDir(), "abs.db")})
	if err != nil {
		t.Fatalf("an absolute path was refused: %v", err)
	}
	defer store.Close()
}
