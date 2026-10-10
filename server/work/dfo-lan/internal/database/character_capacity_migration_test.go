package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCharacterCapacityMigrationPreservesExistingSave(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "pre-capacity.sqlite3")
	db, err := openSQLite(ctx, path, 1, 5000, true)
	if err != nil {
		t.Fatal(err)
	}
	// Build the previously released schema, keeping all existing migration
	// checksums, then insert a real saved aggregate before the new migration.
	for _, name := range sqliteMigrationSections {
		if name == "0038_character_capacity.sql" {
			break
		}
		if err = migrateSQLite(ctx, db, name); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	state := []byte(`{"level":115,"opaque":{"gear":123456},"inventory":{"version":"ordinary-bag-v1","items":[]}}`)
	if _, err = db.ExecContext(ctx, "INSERT INTO accounts(id,username) VALUES(1,'existing-account')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "INSERT INTO characters(id,account_id,wire_id,name,profession,create_request,config_version,state,roster_order) VALUES(1,1,1,'existing-role',0,?, ?,?,1)", []byte{1}, "existing-version", state); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, Config{Driver: DriverSQLite, SQLitePath: path})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	roles, err := store.Characters(ctx, 1)
	if err != nil || len(roles) != 1 || roles[0].ID != 1 || roles[0].Name != "existing-role" || string(roles[0].State) != string(state) {
		t.Fatalf("existing save changed: %+v %v", roles, err)
	}
	capacity, err := store.AccountCharacterCapacity(ctx, 1, 24)
	if err != nil || capacity != 24 {
		t.Fatalf("existing account base=%d %v", capacity, err)
	}
}
