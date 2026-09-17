package protocol

import "testing"

func TestStartMapCachedRoom(t *testing.T) {
	s := StartMapState{Map: 100016035, Position: [2]byte{1, 2}, ReuseRoom: true}
	p, err := StartMap(s)
	if err != nil || len(p) != 34 || p[31] != 0 || p[32] != 0 || p[33] != 255 {
		t.Fatal(p, err)
	}
	s.LayerChange = true
	if _, err = StartMap(s); err == nil {
		t.Fatal("cached fresh layer accepted")
	}
	s.LayerChange = false
	s.Monsters = []DungeonMonster{{Entity: 1, Template: 1}}
	if _, err = StartMap(s); err == nil {
		t.Fatal("cached spawn rows accepted")
	}
}
