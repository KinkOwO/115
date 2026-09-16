package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestDungeonGateNativeCursor(t *testing.T) {
	b, e := os.ReadFile("testdata/native_dungeon_gate_cursor.json")
	if e != nil {
		t.Fatal(e)
	}
	var native struct {
		Payload  string                       `json:"payload_hex"`
		Consumed int                          `json:"consumed"`
		Reads    []struct{ Offset, Size int } `json:"reads"`
	}
	if e = json.Unmarshal(b, &native); e != nil {
		t.Fatal(e)
	}
	want, e := hex.DecodeString(native.Payload)
	if e != nil {
		t.Fatal(e)
	}
	got := EnterDungeonSelection()
	if !bytes.Equal(got, want) || len(got) != native.Consumed {
		t.Fatalf("native cursor mismatch: %x", got)
	}
	for _, p := range [][]byte{nil, make([]byte, 7), make([]byte, 9), {0, 0, 0, 0, 1, 0, 0, 0}} {
		if _, e = DecodeDungeonGate(p); e == nil {
			t.Fatalf("accepted invalid gate %x", p)
		}
	}
	if id, e := DecodeDungeonGate(make([]byte, 8)); e != nil || id != 0 {
		t.Fatalf("live19 gate: %d %v", id, e)
	}
}

func TestDungeonEntryNativeCursors(t *testing.T) {
	start, e := StartMap(StartMapState{Position: [2]byte{0, 1}, Seed: 12345, Map: 76121, Monsters: []DungeonMonster{{Entity: 1, SourceIndex: 0, Level: 3, Rank: 3, Team: 100, Template: 109014870}}})
	if e != nil {
		t.Fatal(e)
	}
	packets := map[string][]byte{"dungeon_info": DungeonInfo(DungeonInfoState{ID: 3, Maze: 1, Boss: [2]byte{3, 0}}), "start_map": start}
	for name, got := range packets {
		b, e := os.ReadFile("testdata/native_" + name + "_cursor.json")
		if e != nil {
			t.Fatal(e)
		}
		var native struct {
			Payload  string `json:"payload_hex"`
			Consumed int    `json:"consumed"`
		}
		if e = json.Unmarshal(b, &native); e != nil {
			t.Fatal(e)
		}
		want, e := hex.DecodeString(native.Payload)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(got, want) || len(got) != native.Consumed {
			t.Fatalf("%s cursor mismatch\ngot %x\nwant%x", name, got, want)
		}
	}
	live, _ := hex.DecodeString("030000000000000000ffff0000000000490c0000000000000000000000000000")
	r, e := DecodeDungeonSelection(live)
	if e != nil || r.ID != 3 || r.Quest != 3145 || r.Party != 65535 {
		t.Fatalf("live20 selection: %+v %v", r, e)
	}
	live[31] = 1
	if _, e = DecodeDungeonSelection(live); e == nil {
		t.Fatal("accepted nonzero padding")
	}
}

func TestSourceTeamsReachNativeSpawnReader(t *testing.T) {
	p, e := StartMap(StartMapState{Position: [2]byte{0, 1}, Seed: 12345, Map: 76121, Monsters: []DungeonMonster{
		{Entity: 1, SourceIndex: 0, Level: 3, Rank: 3, Team: 100, Template: 109014870},
		{Entity: 2, SourceIndex: 1, Level: 3, Team: 0, Template: 109014870, NonCombat: true},
	}})
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile("testdata/native_start_map_teams.json")
	if e != nil {
		t.Fatal(e)
	}
	var native struct {
		Payload  string `json:"payload_hex"`
		Consumed int    `json:"consumed"`
		Spawns   []struct{ Team uint32 }
	}
	if e = json.Unmarshal(b, &native); e != nil {
		t.Fatal(e)
	}
	want, e := hex.DecodeString(native.Payload)
	if e != nil || !bytes.Equal(p, want) || len(p) != native.Consumed || len(native.Spawns) != 2 || native.Spawns[0].Team != 100 || native.Spawns[1].Team != 0 {
		t.Fatal("source teams do not match native reader")
	}
}
