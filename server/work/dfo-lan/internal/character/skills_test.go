package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/storage"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestStagedSlayerInitialSkillsAgainstNativeReader(t *testing.T) {
	c, err := catalog.LoadCharacters("../../configs/characters.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	p := c.Professions[0]
	raw, err := json.Marshal(State{Level: 1, InitialSkills: p.InitialSkills, SourceSHA256: p.RawSHA256})
	if err != nil {
		t.Fatal(err)
	}
	s := Service{Catalog: c}
	got, err := s.EntrySkills(storage.Character{Profession: 0, State: raw})
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("../game/protocol/testdata/native_self_skills.json")
	if err != nil {
		t.Fatal(err)
	}
	var native struct {
		Payload string `json:"payload_hex"`
	}
	if err = json.Unmarshal(fixture, &native); err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString(native.Payload)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("staged catalog and native initial skill reader differ: %v", err)
	}
}

func TestInitialSkillColumns(t *testing.T) {
	skills := initialSkills(State{Level: 1, InitialSkills: []int32{179, 7, 1, 174, 1, 1}})
	if len(skills) != 2 || skills[0].ID != 179 || skills[0].Level != 7 {
		t.Fatalf("source skill columns: %+v", skills)
	}
	// Undecorable tuples are skipped (recorded as zero), never refused: an
	// incomplete triple, a non-grant condition, an out-of-range level or id
	// must not take down the whole entry append packet.
	for _, cells := range [][]int32{{179, 7}, {179, 7, 5}, {179, 256, 1}, {70000, 1, 1}} {
		if got := initialSkills(State{Level: 1, InitialSkills: cells}); len(got) != 0 {
			t.Fatalf("undecorable tuple %v should be skipped, got %+v", cells, got)
		}
	}
}
