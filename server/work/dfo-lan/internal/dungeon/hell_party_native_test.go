package dungeon_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"encoding/hex"
	"os"
	"sort"
	"testing"
)

// Explicit current-PVF coverage. Never opens player storage or the client.
func TestHellPartyCurrentPVFEntryCoverage(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native Hell Party entry coverage")
	}
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256")})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	world, err := source.World("")
	if err != nil {
		t.Fatal(err)
	}
	c, err := source.RuntimeFullDungeons(world, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseMapSource()
	overlay, unavailable, err := source.HellPartyMaps(c)
	if err != nil {
		t.Fatal(err)
	}
	if err = catalog.ApplyHellPartyMaps(&c, overlay); err != nil {
		t.Fatal(err)
	}
	ids := []uint32{}
	for id, d := range c.Dungeons {
		if d.HellParty != nil {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	entered, missing := 0, 0
	for _, id := range ids {
		d := c.Dungeons[id]
		r := protocol.DungeonSelection{ID: id, Difficulty: 2, Mode: 1, Party: 65535}
		s, err := dungeon.Select(c, r, 115, nil)
		if _, ok := c.Maps[d.HellParty.SealMap]; !ok {
			missing++
			if err == nil {
				t.Errorf("dungeon %d accepted missing source seal map", id)
			}
			continue
		}
		if err != nil {
			t.Errorf("dungeon %d (%s): %v", id, d.Script.Path, err)
			continue
		}
		entered++
		if s.HellPosition == nil || *s.HellPosition != d.HellParty.SealPosition {
			t.Errorf("dungeon %d lost Hell coordinate", id)
		}
		found := false
		for _, room := range s.Maze.Rooms {
			if [2]byte{room.X, room.Y} == *s.HellPosition {
				found = room.Map == d.HellParty.SealMap
			}
		}
		if !found {
			t.Errorf("dungeon %d lost seal map", id)
		}
		if _, err := c.MapScript(d.HellParty.SealMap); err != nil {
			t.Errorf("dungeon %d seal read: %v", id, err)
		}
		info := protocol.DungeonInfo(protocol.DungeonInfoState{ID: id, Difficulty: r.Difficulty, Maze: s.Maze.Index, Boss: s.Maze.Boss, Hell: s.HellPosition})
		if info[10] != s.HellPosition[0] || info[11] != s.HellPosition[1] {
			t.Errorf("dungeon %d NOTI28 mismatch", id)
		}
	}
	if entered == 0 {
		t.Fatal("no native Hell dungeons tested")
	}
	t.Logf("source=%s declared=%d entered=%d missing=%d unavailable maps=%v", c.Source.Checksum, len(ids), entered, missing, unavailable)
	// The actual 2026-10-03 report: role 10 selecting dungeon 87, Expert,
	// Mode 1, no quest. This failed solely at the old ID103 probe gate.
	body, _ := hex.DecodeString("570000000200000100ffff000000000000000000000000000000000000000000")
	r, err := protocol.DecodeDungeonSelection(body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = dungeon.Select(c, r, 115, nil); err != nil {
		t.Fatalf("captured dungeon87 request: %v", err)
	}
}
