package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
	"reflect"
	"testing"
)

func TestLocalMonsterAuthorityAcrossRoomsAndDeaths(t *testing.T) {
	c, e := catalog.LoadDungeons(testfixture.DungeonPath(t, "dungeons.generated.json"))
	if e != nil {
		t.Fatal(e)
	}
	req := protocol.DungeonSelection{ID: 3, Party: 65535, Quest: 3145}
	normal, e := Select(c, req, 1, map[uint16]bool{3145: true})
	if e != nil {
		t.Fatal(e)
	}
	owned, e := Select(c, req, 1, map[uint16]bool{3145: true}, true)
	if e != nil {
		t.Fatal(e)
	}
	for i, m := range owned.Monsters {
		if m.Entity%8 != 0 || normal.Monsters[i].Entity != uint16(4096+i) {
			t.Fatal("authority policy leaked", m)
		}
		original := normal.Monsters[i]
		m.Entity = original.Entity
		if !reflect.DeepEqual(m, original) {
			t.Fatal("source monster changed", m, original)
		}
	}
	owned.Loaded = true
	if _, e = owned.Move(c, [2]byte{1, 1}); e == nil {
		t.Fatal("live monsters did not block gate")
	}
	for _, m := range owned.Monsters {
		if _, e = owned.ConfirmDeath(uint32(m.Entity), 9, 3); e == nil {
			t.Fatal("foreign kill accepted")
		}
		if fresh, e := owned.ConfirmDeath(uint32(m.Entity), 3, 3); e != nil || !fresh {
			t.Fatal(e)
		}
		if fresh, e := owned.ConfirmDeath(uint32(m.Entity), 3, 3); e != nil || fresh {
			t.Fatal("duplicate", e)
		}
	}
	next, e := owned.Move(c, [2]byte{1, 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range next.Monsters {
		if m.Entity%8 != 0 || m.Entity < owned.NextEntity {
			t.Fatal("new room lost authority", m)
		}
	}
	revisit, e := next.enterRoom(c, owned.Room)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(revisit.Monsters, owned.Monsters) || len(revisit.LivingMonsters()) != 0 {
		t.Fatal("revisit resurrected or renumbered monsters")
	}
	next.NextEntity = 65528
	next.Visited = map[uint32][]protocol.DungeonMonster{}
	last, e := next.enterRoom(c, catalog.DungeonRoom{Map: 76121})
	if e == nil || last != nil {
		t.Fatal("identity space wrapped")
	}
}

func TestLocalMonsterAuthorityReservedHellWaves(t *testing.T) {
	c, s := hellWaveFixture()
	s.localMonsterAuthority = true
	s.NextEntity = 4104
	run, e := newHellPartyRun(c, s, func(n uint64) (uint64, error) { return n - 1, nil })
	if e != nil {
		t.Fatal(e)
	}
	s.HellParty = run
	for _, m := range run.Rows {
		if m.Entity%8 != 0 {
			t.Fatal(m)
		}
		if _, ok := run.Actors[m.Entity]; !ok {
			t.Fatal("wave reward identity lost")
		}
	}
	next, e := s.enterRoom(c, catalog.DungeonRoom{Map: 7})
	if e != nil {
		t.Fatal(e)
	}
	next.Loaded = true
	for _, m := range next.Monsters {
		if _, e = next.ConfirmDeath(uint32(m.Entity), 3, 3); e != nil {
			t.Fatal(e)
		}
	}
	if !next.HellPartyCleared() {
		t.Fatal("wave did not clear")
	}
	if advanceMonsterEntity(65528, 8) != 65535 || advanceMonsterEntity(65534, 1) != 65535 {
		t.Fatal("overflow sentinel")
	}
}
