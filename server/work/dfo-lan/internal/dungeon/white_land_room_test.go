package dungeon

import (
	"encoding/hex"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
)

func TestWhiteLandCapturedDownwardRoomMove(t *testing.T) {
	c := catalog.LoadNativeFullDungeons(t)
	s, err := Select(c, protocol.DungeonSelection{ID: 100002746, Quest: 12920, Party: 65535}, 100, map[uint16]bool{12920: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, pos := range [][2]byte{{1, 0}, {2, 0}} {
		s.Loaded = true
		for _, m := range s.Monsters {
			s.Dead[m.Entity] = true
		}
		s, err = s.Move(c, pos)
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := hex.DecodeString("0201950300007e01000000eb92000000000000000000000000000000000700000000000000000000000000000000000000000000000000000000000000f300edfb0100950300007e010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000305ffffffffffff00000000000000baebf5050000000000")
	if err != nil {
		t.Fatal(err)
	}
	pos, err := protocol.DecodeMoveDungeonRoom(p)
	if err != nil || pos != [2]byte{2, 1} {
		t.Fatalf("captured downward target=%v err=%v", pos, err)
	}
	for _, m := range s.Monsters {
		s.Dead[m.Entity] = true
	}
	s.Loaded = true
	next, err := s.Move(c, pos)
	if err != nil {
		t.Fatal(err)
	}
	if next.Room.Map != 100004527 || len(next.Monsters) != 4 {
		t.Fatalf("wrong room or monster count: room=%+v monsters=%+v", next.Room, next.Monsters)
	}
	m := next.Monsters[3]
	if m.SourceIndex != 3 || m.Template != 109013403 || m.Rank != 0 || m.Level != 100 || m.Team != 100 || m.NonCombat {
		t.Fatalf("fixed row with omitted rank changed: %+v", m)
	}
	for _, room := range s.Maze.Rooms {
		monsters, err := fixedMonsters(c.Maps[room.Map], s.Definition.BasisLevel)
		if err != nil {
			t.Fatalf("White Land map %d: %v", room.Map, err)
		}
		if _, err := protocol.StartMap(protocol.StartMapState{Position: [2]byte{room.X, room.Y}, Seed: 1, Map: room.Map, Monsters: monsters}); err != nil {
			t.Fatalf("White Land map %d encoding: %v", room.Map, err)
		}
	}
}

func TestOmittedMonsterRankRetainsValidationAndRowIndices(t *testing.T) {
	num := func(v int32) pvf.Token { return pvf.Token{Type: 0, Value: v} }
	opt := func(s string) pvf.Token { return pvf.Token{Type: 6, Text: s} }
	row := []pvf.Token{num(109013403), num(1), num(0), num(982), num(287), num(0), num(1), num(1), opt("[fixed]")}
	script := catalog.ScriptRecord{Cells: append([]pvf.Token{{Type: 3, Text: "[monster]"}}, row...)}
	script.Cells = append(script.Cells, row...)
	monsters, err := fixedMonsters(script, 100)
	if err != nil || len(monsters) != 2 || monsters[0].Rank != 0 || monsters[1].Rank != 0 || monsters[1].SourceIndex != 1 {
		t.Fatalf("omitted ranks lost row alignment: monsters=%+v err=%v", monsters, err)
	}
	for _, option := range []string{"[random]", "[unknown rank]"} {
		bad := catalog.ScriptRecord{Cells: append([]pvf.Token{{Type: 3, Text: "[monster]"}}, row...)}
		bad.Cells = append(bad.Cells, opt(option))
		if _, err := fixedMonsters(bad, 100); err == nil {
			t.Fatalf("accepted unresolved option %s", option)
		}
	}
	noFixed := catalog.ScriptRecord{Cells: append([]pvf.Token{{Type: 3, Text: "[monster]"}}, row[:8]...)}
	if _, err := fixedMonsters(noFixed, 100); err == nil {
		t.Fatal("accepted non-fixed placement with omitted rank")
	}
	duplicate := catalog.ScriptRecord{Cells: append([]pvf.Token{{Type: 3, Text: "[monster]"}}, row...)}
	duplicate.Cells = append(duplicate.Cells, opt("[normal]"), opt("[boss]"))
	if _, err := fixedMonsters(duplicate, 100); err == nil {
		t.Fatal("accepted duplicate explicit ranks")
	}
}
