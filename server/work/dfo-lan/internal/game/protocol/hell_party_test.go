package protocol

import (
	"encoding/binary"
	"testing"
)

func TestNativeHellHiddenQueueAndAPCPreload(t *testing.T) {
	body, err := StartMap(StartMapState{Map: 60051, HellPartyMode: 1, Monsters: []DungeonMonster{{Entity: 4101, Template: 10627, Level: 65, Rank: 5, Team: 100, APC: true, SourceIndex: 10000, Hidden: true, SpawnOrder: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	row := body[37:59]
	if body[7] != 1 || body[8] != 0 || binary.LittleEndian.Uint16(row) != 2 || binary.LittleEndian.Uint32(row[2:]) != 10000 || binary.LittleEndian.Uint16(row[6:]) != 4101 || binary.LittleEndian.Uint32(row[8:]) != 10627 || row[12] != 65 || row[13] != 5 || row[15] != 1 || int8(row[16]) != -1 || binary.LittleEndian.Uint32(row[17:]) != 100 {
		t.Fatalf("native hidden queue fields: %x", body)
	}
	preload, err := HellPartyMonsterInfo([]HellPartyAPC{{Template: 10627, Level: 65}})
	if err != nil || len(preload) != 12 || binary.LittleEndian.Uint32(preload) != 1 || binary.LittleEndian.Uint32(preload[4:]) != 10627 || binary.LittleEndian.Uint32(preload[8:]) != 65 {
		t.Fatalf("native preload pairs: %x %v", preload, err)
	}
	if len(HellPartyClear()) != 0 {
		t.Fatal("NOTI777 unexpectedly carries reward rows")
	}
}
