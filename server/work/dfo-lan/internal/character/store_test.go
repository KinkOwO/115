package character_test

import (
	"dfolan/internal/catalog"
	. "dfolan/internal/character"
	"dfolan/internal/database"
	"os"
	"path/filepath"
	"testing"
)

func TestPersistenceConstructorNilCompatibility(t *testing.T) {
	var concrete *database.Store
	c := catalog.Characters{Professions: map[byte]catalog.Profession{0: {}}}
	if _, err := New(concrete, c, Rules{MaxCharacters: 24, InitialLevel: 1}); err == nil {
		t.Fatal("typed nil storage accepted by character constructor")
	}
	path := filepath.Join(t.TempDir(), "fatigue.json")
	if err := os.WriteFile(path, []byte(`{"daily_limit":156,"room_cost":1,"timezone":"UTC","reset_hour":6,"source":"fixture"}`), 0600); err != nil {
		t.Fatal(err)
	}
	fatigue, err := LoadFatigueService(concrete, path)
	if err != nil {
		t.Fatal(err)
	}
	if fatigue.Store != nil {
		t.Fatal("typed nil storage changed offline fatigue optional-store behavior")
	}
}
