package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/hex"
	"path/filepath"
	"testing"
)

func TestYellowDragonCapturedEntry(t *testing.T) {
	base := filepath.Join("..", "..", "configs")
	c := catalog.LoadNativeFullDungeons(t)
	if err := catalog.AttachTournamentQuestMaps(&c, filepath.Join(base, "dungeons.tournament-quest-maps.json")); err != nil {
		t.Fatal(err)
	}
	body, err := hex.DecodeString("e2edf5050300000000ffff0000000000d8350000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	request, err := protocol.DecodeDungeonSelection(body)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Select(c, request, 115, map[uint16]bool{13784: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.Room.Map != 100008699 || s.Maze.Quest != 13784 || s.Tournament == nil || len(s.Monsters) != 4 {
		t.Fatalf("quest arena did not resolve: map=%d quest=%d tournament=%v actors=%d", s.Room.Map, s.Maze.Quest, s.Tournament != nil, len(s.Monsters))
	}
	info, err := protocol.TournamentInfo(s.Tournament.Opening)
	if err != nil {
		t.Fatal(err)
	}
	mapInfo, err := protocol.TournamentMapInfo(s.Maze.Start, s.Tournament.Seed, s.Room.Map)
	if err != nil {
		t.Fatal(err)
	}
	if len(info) != 260 || binary.LittleEndian.Uint32(info) != request.ID || len(mapInfo) != 15 || binary.LittleEndian.Uint32(mapInfo[8:12]) != s.Room.Map {
		t.Fatalf("tournament entry body mismatch: info=%d map=%d", len(info), len(mapInfo))
	}
	if _, err := protocol.StartMap(protocol.StartMapState{Position: s.Maze.Start, Seed: s.Tournament.Seed, Map: s.Room.Map, Monsters: s.Monsters}); err != nil {
		t.Fatal(err)
	}
	rate, prizes, err := TournamentPrizeRules(s.Definition)
	if err != nil || rate != 16 || len(prizes) != 3 || prizes[0] != [3]uint32{3329, 60, 1} || prizes[1] != [3]uint32{3329, 35, 2} || prizes[2] != [3]uint32{3329, 5, 3} {
		t.Fatalf("source tournament champion rewards: rate=%d prizes=%v err=%v", rate, prizes, err)
	}
	// This is the next quest's actual live CMD16 after yellow-dragon completion.
	blueBody, err := hex.DecodeString("e3edf5050300000000ffff0000000000d9350000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	blueRequest, err := protocol.DecodeDungeonSelection(blueBody)
	if err != nil {
		t.Fatal(err)
	}
	blue, err := Select(c, blueRequest, 115, map[uint16]bool{13785: true})
	if err != nil {
		t.Fatal(err)
	}
	if blue.Room.Map != 100008700 || blue.Maze.Quest != 13785 || blue.Tournament == nil || len(blue.Monsters) != 4 {
		t.Fatalf("blue quest arena did not resolve: map=%d quest=%d tournament=%v actors=%d", blue.Room.Map, blue.Maze.Quest, blue.Tournament != nil, len(blue.Monsters))
	}
	rate, prizes, err = TournamentPrizeRules(blue.Definition)
	if err != nil || rate != 16 || len(prizes) != 3 || prizes[0] != [3]uint32{3323, 60, 1} || prizes[1] != [3]uint32{3323, 35, 2} || prizes[2] != [3]uint32{3323, 5, 3} {
		t.Fatalf("blue source champion rewards: rate=%d prizes=%v err=%v", rate, prizes, err)
	}
}
