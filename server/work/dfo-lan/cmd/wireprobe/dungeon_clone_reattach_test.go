package main

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDungeonCloneReattachRestoresOrdinaryGearLast(t *testing.T) {
	path := filepath.Join(t.TempDir(), "equipment.json")
	data, err := json.Marshal(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: "fixture"},
		Rows: []inventory.EquipmentDefinition{
			{ID: 517500000, Path: "clone.equ", SHA256: strings.Repeat("a", 64), Fields: map[string][]pvf.Token{
				"[item category]": {{Type: 6, Text: "clear avatar"}},
			}},
			{ID: 112500000, Path: "default.equ", SHA256: strings.Repeat("b", 64)},
			{ID: 101000013, Path: "weapon.equ", SHA256: strings.Repeat("c", 64)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := inventory.LoadEquipmentCatalog(path, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	role := storage.Character{
		ID: 5, WireID: 503, Profession: 11,
		State: json.RawMessage(`{"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100},"inventory":{"version":"ordinary-bag-v1","worn":[{"slot":3,"template":517500000},{"slot":12,"template":101000013}]}}`),
	}
	w := &worldSession{
		role: role, activeDungeon: &dungeon.Session{},
		characters: &character.Service{DetailedWornCandidate: true, Equipment: catalog},
	}
	plan, err := w.finishDungeonLoading(make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	var actual []string
	for _, packet := range plan {
		if packet.Name == "dungeon_worn_visuals_restored" || strings.HasPrefix(packet.Name, "dungeon_clone_") || packet.Name == "dungeon_nonavatar_worn_restored" {
			actual = append(actual, packet.Name)
		}
	}
	want := []string{"dungeon_worn_visuals_restored", "dungeon_clone_detached", "dungeon_clone_reattached", "dungeon_nonavatar_worn_restored"}
	if len(actual) != len(want) {
		t.Fatalf("unexpected Clone loading sequence: %v", actual)
	}
	for i := range want {
		if actual[i] != want[i] {
			t.Fatalf("Clone loading order %v, want %v", actual, want)
		}
	}
	if plan[len(plan)-2].ID != 14 || plan[len(plan)-1].ID != 1361 {
		t.Fatal("ordinary equipment must follow both mode-1 packets, then buff registration must bind the new actor")
	}
}
