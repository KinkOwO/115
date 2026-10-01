package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestNewActiveSkillsUseFreeShortcutsWithoutOverwritingPlayerBindings(t *testing.T) {
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	l, e := LoadLearningCatalog("../../configs/skills.next27.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	s := Service{Catalog: c, Learning: l}
	for job := range c.Professions {
		var active, passive uint16
		for id, d := range l.index[job] {
			if d.Active() {
				active = id
			} else {
				passive = id
			}
		}
		if active == 0 || passive == 0 {
			t.Fatal("source skill types missing", job)
		}
		rows := []protocol.LearnedSkill{{ID: 65530, Slot: 0, Level: 1}, {ID: 65531, Slot: 2, Level: 1}, {ID: active, Slot: 14, Level: 1}, {ID: passive, Slot: 15, Level: 1}}
		s.placeNewShortcuts(job, rows, []uint16{passive, active})
		if rows[0].Slot != 0 || rows[1].Slot != 2 || rows[2].Slot != 1 || rows[3].Slot != 15 {
			t.Fatal(job, rows)
		}
		rows[2].Slot = 20
		s.placeNewShortcuts(job, rows, nil)
		if rows[2].Slot != 20 {
			t.Fatal("upgrade moved an existing binding")
		}
		var full []protocol.LearnedSkill
		for i := 0; i < 14; i++ {
			full = append(full, protocol.LearnedSkill{ID: uint16(65000 + i), Slot: uint16(i), Level: 1})
		}
		full = append(full, protocol.LearnedSkill{ID: active, Slot: 14, Level: 1})
		s.placeNewShortcuts(job, full, []uint16{active})
		if full[14].Slot != 14 {
			t.Fatal("full bar overwritten")
		}
	}
}

// Read the documented native protobuf fields to check profession ownership,
// starter grants, and the distinction between book entries and shortcuts.
func pbFields(t *testing.T, p []byte) map[int][][]byte {
	t.Helper()
	out := map[int][][]byte{}
	for len(p) > 0 {
		tag, n := binary.Uvarint(p)
		if n <= 0 {
			t.Fatal("invalid tag")
		}
		p = p[n:]
		switch tag & 7 {
		case 0:
			_, n = binary.Uvarint(p)
			if n <= 0 {
				t.Fatal("invalid scalar")
			}
			out[int(tag>>3)] = append(out[int(tag>>3)], p[:n])
			p = p[n:]
		case 2:
			l, n := binary.Uvarint(p)
			if n <= 0 || l > uint64(len(p)-n) {
				t.Fatal("invalid bytes")
			}
			p = p[n:]
			out[int(tag>>3)] = append(out[int(tag>>3)], p[:int(l)])
			p = p[int(l):]
		default:
			t.Fatal("unsupported native protobuf wire type")
		}
	}
	return out
}
func TestAllProfessionsKeepStarterRanksAndBookSlots(t *testing.T) {
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	l, e := LoadLearningCatalog("../../configs/skills.next27.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	s := Service{Catalog: c, Learning: l}
	for job, p := range c.Professions {
		state := State{Level: 1, SourceSHA256: p.RawSHA256, InitialSkills: p.InitialSkills}
		raw, _ := json.Marshal(state)
		b, e := s.EntrySkills(storage.Character{Profession: job, ConfigVersion: c.Source.SaveIdentity(), State: raw})
		if e != nil {
			t.Fatalf("profession%d: %v", job, e)
		}
		f := pbFields(t, b[4:])
		if len(f[2]) != 2 {
			t.Fatal("skill pages missing")
		}
		known, e := knownSkills(state, 0)
		if e != nil {
			t.Fatal(e)
		}
		for _, page := range f[2] {
			slots := map[uint64]bool{}
			granted := map[uint16]byte{}
			for _, item := range pbFields(t, page)[3] {
				q := pbFields(t, item)
				slot, _ := binary.Uvarint(q[1][0])
				id, _ := binary.Uvarint(q[2][0])
				rank, _ := binary.Uvarint(q[3][0])
				if _, ok := l.index[job][uint16(id)]; !ok {
					t.Fatal("another profession skill leaked")
				}
				if slots[slot] || slot >= 255 {
					t.Fatalf("profession%d duplicate/invalid book slot%d", job, slot)
				}
				slots[slot] = true
				if rank == 0 {
					t.Fatalf("unlearned skill row should not be projected: id=%d slot=%d", id, slot)
				} else {
					granted[uint16(id)] = byte(rank)
				}
			}
			if len(granted) != len(known) {
				t.Fatal("extra initial skills granted")
			}
			for id, rank := range known {
				if granted[id] != rank {
					t.Fatal("source initial rank changed")
				}
			}
		}
	}
}
