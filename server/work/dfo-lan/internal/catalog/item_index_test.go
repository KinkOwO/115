package catalog

import (
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func TestSupplementItemIndexPreservesExistingDropsAndSkipsGoldAndEquipment(t *testing.T) {
	checksum := strings.Repeat("1", 64)
	source := pvf.ArchiveSnapshot{Checksum: checksum}
	original := LootItem{ID: 7, Kind: "stackable", Weight: 100, StackLimit: 20, Script: ScriptRecord{Path: "original"}}
	loot := LootCatalog{Source: source, Items: map[uint32]LootItem{7: original}}
	index := ItemIndex{Source: source, Items: map[uint32]ItemIndexEntry{
		0: {ID: 0, Kind: "stackable", Path: "gold"}, 7: {ID: 7, Kind: "stackable", StackLimit: 500},
		8: {ID: 8, Kind: "stackable", Path: "new", StackableType: "[material]", StackLimit: 99},
		9: {ID: 9, Kind: "equipment", Path: "gear"}, 10: {ID: 10, Kind: "avatar", Path: "avatar"}}}
	if err := loot.SupplementItemIndex(index); err != nil {
		t.Fatal(err)
	}
	if len(loot.Items) != 2 || loot.Items[7].Weight != 100 || loot.Items[7].StackLimit != 20 || loot.Items[7].Script.Path != "original" {
		t.Fatal("drop membership or source values changed")
	}
	if it := loot.Items[8]; it.Weight != 0 || it.StackLimit != 99 || it.StackableType != "[material]" || it.Script.Path != "new" {
		t.Fatal(it)
	}
	index.Source.Checksum = strings.Repeat("2", 64)
	if err := loot.SupplementItemIndex(index); err == nil {
		t.Fatal("mixed source accepted")
	}
}
