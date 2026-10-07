package legion

import (
	"dfolan/internal/catalog"
	"encoding/binary"
	"testing"
	"time"
)

func TestBakalNewRunClearsPreviousLocationTableBeforeRoster(t *testing.T) {
	r, e := catalog.ImportBakalRaid(catalog.OpenNativeArchive(t))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.DungeonCatalog.CloseMapSource() })
	o, e := PrepareBakalOpening(r, false)
	if e != nil {
		t.Fatal(e)
	}
	at := time.Unix(1800000000, 0)
	o.Start("owned", at)
	frames := o.Tick(at.Add(time.Duration(r.StartDelaySecs) * time.Second))
	// Model the client incremental table with a stale Eclair portrait from
	// the prior run (real023721 trace: first38, next36).
	state := map[uint32]uint32{38: BakalMonsterType("eclair")}
	previous := map[uint32]uint32{38: BakalMonsterType("eclair")}
	resetSeen, rosterSeen := false, false
	for _, f := range frames {
		if f.ID != 2286 {
			continue
		}
		if f.Name == "bakal_map_locations_reset" {
			if rosterSeen {
				t.Fatal("reset erased new roster")
			}
			resetSeen = true
		}
		if f.Name == "bakal_monsters" {
			rosterSeen = true
		}
		b := f.Body
		for i := 0; i < int(b[0]); i++ {
			row := b[1+19*i:]
			kind, loc := binary.LittleEndian.Uint32(row), binary.LittleEndian.Uint32(row[4:])
			mode := binary.LittleEndian.Uint32(row[8:])
			if mode != 2 {
				state[loc] = kind
			}
			if mode == 1 {
				previous[loc] = kind
			}
		}
	}
	if !resetSeen || !rosterSeen || state[38] != 0 || previous[38] != 0 {
		t.Fatal("previous random location portrait survived new run")
	}
	for _, p := range o.MonsterPlacements() {
		if state[uint32(p.Location)] != BakalMonsterType(p.Name) {
			t.Fatal("reset removed current live actor")
		}
	}
}
