package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/binary"
	"strings"
	"testing"
)

func TestOdysseyDungeonRequiresModeOwnership(t *testing.T) {
	w := &worldSession{role: storage.Character{ID: 1}, dungeons: &catalog.DungeonCatalog{Dungeons: map[uint32]catalog.DungeonDefinition{100004934: {ID: 100004934, Odyssey: true, DesignatedDifficulty: 2}}}}
	p := make([]byte, 32)
	binary.LittleEndian.PutUint32(p, 100004934)
	p[4] = 2
	binary.LittleEndian.PutUint16(p[9:], 65535)
	// An ordinary character cannot opt into Odyssey by sending its dungeon ID.
	_, _, err := w.selectDungeon(p)
	if err == nil || !strings.Contains(err.Error(), "requires an Odyssey character") {
		t.Fatal(err)
	}
	if _, err := protocol.DecodeDungeonSelection(p); err != nil {
		t.Fatal(err)
	}
}
