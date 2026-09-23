package main

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneAvatarRemovalRefreshGate(t *testing.T) {
	catalogPath := filepath.Join(t.TempDir(), "equipment.json")
	catalogBytes, err := json.Marshal(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: "test"},
		Rows: []inventory.EquipmentDefinition{
			{ID: 513570010, Path: "avatar/clone.equ", SHA256: strings.Repeat("0", 64), Fields: map[string][]pvf.Token{
				"[item category]": {{Type: 6, Text: "clear avatar"}},
			}},
			{ID: 513572726, Path: "avatar/look.equ", SHA256: strings.Repeat("1", 64), Fields: map[string][]pvf.Token{
				"[item category]": {{Type: 6, Text: "avatar"}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(catalogPath, catalogBytes, 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := inventory.LoadEquipmentCatalog(catalogPath, "test")
	if err != nil {
		t.Fatal(err)
	}
	observed := protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 8, SourceItem: 0, DestinationList: 3, DestinationSlot: 2, DestinationItem: 513570010}
	if !cloneAvatarRemoval(observed, catalog) {
		t.Fatal("observed Clone removal must refresh the detailed avatar association")
	}
	for name, change := range map[string]func(*protocol.ItemMoveRequest){
		"ordinary look":   func(r *protocol.ItemMoveRequest) { r.DestinationItem = 513572726 },
		"other worn slot": func(r *protocol.ItemMoveRequest) { r.DestinationSlot = 12 },
		"occupied bag":    func(r *protocol.ItemMoveRequest) { r.SourceItem = 513572726 },
		"equip clone":     func(r *protocol.ItemMoveRequest) { r.SourceList, r.DestinationList = 3, 1 },
		"unknown item":    func(r *protocol.ItemMoveRequest) { r.DestinationItem = 123456 },
	} {
		t.Run(name, func(t *testing.T) {
			r := observed
			change(&r)
			if cloneAvatarRemoval(r, catalog) {
				t.Fatalf("unrelated move unexpectedly enabled detailed refresh: %+v", r)
			}
		})
	}
	if cloneAvatarRemoval(observed, nil) {
		t.Fatal("missing catalog cannot authorize Clone-only refresh")
	}
}
