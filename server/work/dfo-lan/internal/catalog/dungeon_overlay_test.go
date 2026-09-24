package catalog

import (
	"dfolan/internal/catalog/pvf"
	"path/filepath"
	"testing"
)

func TestTrainingRoomOverlayMatchesSource(t *testing.T) {
	overlay, err := LoadDungeons(filepath.Join("..", "..", "configs", "dungeons.training-room.json"))
	if err != nil {
		t.Fatal(err)
	}
	const source = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	if overlay.Source.Checksum != source || len(overlay.Dungeons) != 4 || len(overlay.Maps) != 7 {
		t.Fatalf("unexpected training room import: source=%s dungeons=%d maps=%d", overlay.Source.Checksum, len(overlay.Dungeons), len(overlay.Maps))
	}
	base := DungeonCatalog{Source: pvf.ArchiveSnapshot{Checksum: source}, Dungeons: map[uint32]DungeonDefinition{}, Maps: map[uint32]ScriptRecord{}}
	if err := MergeDungeonCatalog(&base, overlay); err != nil {
		t.Fatal(err)
	}
	if _, ok := base.Maps[36250]; !ok {
		t.Fatal("training room start map missing")
	}
	if err := MergeDungeonCatalog(&base, overlay); err == nil {
		t.Fatal("duplicate dungeon was overwritten")
	}
	base.Source.Checksum = "different"
	if err := MergeDungeonCatalog(&base, overlay); err == nil {
		t.Fatal("mismatched PVF was accepted")
	}
}
