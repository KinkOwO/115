package main

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDungeonCloneRefreshAfterWornMoves(t *testing.T) {
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
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := inventory.LoadEquipmentCatalog(path, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{
		role:          storage.Character{WireID: 503, Profession: 11, State: json.RawMessage(`{"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100},"inventory":{"version":"ordinary-bag-v1","worn":[{"slot":3,"template":517500000},{"slot":12,"template":101000013}]}}`)},
		activeDungeon: &dungeon.Session{},
		characters:    &character.Service{DetailedWornCandidate: true, Equipment: catalog},
	}
	for _, tc := range []struct {
		name string
		move protocol.ItemMoveRequest
	}{
		{"ordinary equipment", protocol.ItemMoveRequest{SourceList: 1, DestinationList: 3, DestinationSlot: 12}},
		{"clone avatar", protocol.ItemMoveRequest{SourceList: 1, DestinationList: 3, DestinationSlot: 3}},
		{"appearance avatar", protocol.ItemMoveRequest{SourceList: 3, SourceSlot: 3, DestinationList: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, enabled, err := dungeonCloneEquipmentRefresh(w, tc.move)
			if err != nil || !enabled {
				t.Fatalf("worn move did not refresh Clone: enabled=%v err=%v", enabled, err)
			}
			want := []string{"equipment_dungeon_clone_detached", "equipment_dungeon_clone_reattached", "equipment_dungeon_nonavatar_worn_restored"}
			if len(plan) != len(want) {
				t.Fatalf("refresh sequence length %d, want %d", len(plan), len(want))
			}
			for i, name := range want {
				if plan[i].Name != name {
					t.Fatalf("refresh[%d]=%s, want %s", i, plan[i].Name, name)
				}
			}
		})
	}
	for _, tc := range []struct {
		name string
		move protocol.ItemMoveRequest
		w    *worldSession
	}{
		{"bag only", protocol.ItemMoveRequest{SourceList: 1, DestinationList: 1}, w},
		{"town", protocol.ItemMoveRequest{SourceList: 1, DestinationList: 3}, &worldSession{role: w.role, characters: w.characters}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, enabled, err := dungeonCloneEquipmentRefresh(tc.w, tc.move)
			if err != nil || enabled || len(plan) != 0 {
				t.Fatalf("unexpected refresh: enabled=%v packets=%d err=%v", enabled, len(plan), err)
			}
		})
	}
	withoutClone := *w
	withoutClone.role = w.role
	withoutClone.role.State = json.RawMessage(`{"source_sha256":"fixture","inventory":{"version":"ordinary-bag-v1","worn":[{"slot":12,"template":101000013}]}}`)
	plan, enabled, err := dungeonCloneEquipmentRefresh(&withoutClone,
		protocol.ItemMoveRequest{SourceList: 1, DestinationList: 3, DestinationSlot: 12})
	if err != nil || enabled || len(plan) != 0 {
		t.Fatalf("character without Clone was refreshed: enabled=%v packets=%d err=%v", enabled, len(plan), err)
	}
}
