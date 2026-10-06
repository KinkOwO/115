package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBoostSpecialProfessionsSourceAwakening(t *testing.T) {
	dir := os.Getenv("US115_TEST_BOOST_SPECIAL_SOURCE")
	if dir == "" {
		t.Skip("explicit current CHR tokens required")
	}
	s, g, _, _, _ := adminFixture(t)
	for _, v := range []struct {
		job    byte
		file   string
		skills []uint16
	}{{9, "00-dsswordman.chr.tokens.json", []uint16{263, 256, 255, 269}}, {10, "01-creatormage.chr.tokens.json", []uint16{273, 268, 274, 261, 262, 407}}} {
		b, e := os.ReadFile(filepath.Join(dir, v.file))
		if e != nil {
			t.Fatal(e)
		}
		var cells []pvf.Token
		if e = json.Unmarshal(b, &cells); e != nil {
			t.Fatal(e)
		}
		prof := s.Catalog.Professions[v.job]
		prof.AwakeningSkills = catalog.AwakeningSkillGrants(cells)
		s.Catalog.Professions[v.job] = prof
		g.Professions = s.Catalog
		if !baseGrowAwakening(prof) {
			t.Fatalf("job%d source stages missing", v.job)
		}
		raw, e := s.InitialState115(protocol.CreateRequest{Name: "SpecialBoost", Profession: v.job, Options: []byte{0, 0, 0, 0, 0, 0, 255, 0, 0, 0, 0, 0}})
		if e != nil {
			t.Fatal(v.job, e)
		}
		var before map[string]json.RawMessage
		if e = json.Unmarshal(raw, &before); e != nil {
			t.Fatal(e)
		}
		role := Character{ID: 1, AccountID: 1, WireID: 503, Name: "SpecialBoost", Profession: v.job, ConfigVersion: s.Catalog.Source.Checksum, State: raw}
		role.Request = append(binary.LittleEndian.AppendUint32([]byte{v.job}, uint32(len(role.Name))), []byte(role.Name)...)
		role.Request = append(role.Request, []byte{0, 0, 0, 0, 0, 0, 255, 0, 0, 0, 0, 0}...)
		next, e := s.BoostLevel(role, g, 115)
		if e != nil {
			t.Fatal(v.job, e)
		}
		var state State
		if e = json.Unmarshal(next.State, &state); e != nil {
			t.Fatal(e)
		}
		packed, e := state.WireAdvancement()
		if e != nil || packed != 0x30 || state.Advancement != 0 || state.Awakening != 3 || state.Level != 115 {
			t.Fatal(state.Advancement, state.Awakening, packed, e)
		}
		for _, id := range v.skills {
			for tree := range state.LearnedSkills {
				if state.LearnedSkills[tree][id] == 0 {
					t.Fatalf("job%d tree%d lost granted skill%d", v.job, tree, id)
				}
			}
		}
		if _, e = s.EntrySkills(next); e != nil {
			t.Fatal("post-boost skill serialization", v.job, e)
		}
		if _, e = s.EntryBasicProbe(next, [2]byte{}); e != nil {
			t.Fatal("post-boost actor serialization", v.job, e)
		}
		var after map[string]json.RawMessage
		if e = json.Unmarshal(next.State, &after); e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(before["inventory"], after["inventory"]) {
			t.Fatal("special growth changed equipment")
		}
	}
}

