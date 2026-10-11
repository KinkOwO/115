package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"encoding/json"
	"path/filepath"
	"testing"
)

// The robot fork predates the awakening transaction helper. Keep this fixture
// local to support training; do not pull unrelated production fixes across.
func supportTrainingSQLiteFixture(t *testing.T, connections int) (*character.Service, *database.Store, database.Character) {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "training.db"), MaxConnections: int32(connections), SQLiteBusyTimeoutMS: 100})
	must115(t, err)
	t.Cleanup(db.Close)
	must115(t, db.Migrate(ctx))
	c, err := catalog.LoadCharacters("../../configs/characters.next25.json")
	must115(t, err)
	account, err := db.DevelopmentAccount(ctx, "training-apc-test")
	must115(t, err)
	raw := json.RawMessage(`{"level":115,"advancement":0,"inventory":{"version":"ordinary-bag-v1","gold":12345},"future_field":{"keep":true}}`)
	role, err := db.CreateCharacter(ctx, database.Character{AccountID: account, Name: "TrainingAPC", Profession: 0, ConfigVersion: c.Source.SaveIdentity(), State: raw, Request: []byte{0}}, 24)
	must115(t, err)
	return &character.Service{Store: db, Catalog: c}, db, role
}
