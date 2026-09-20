package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestSelfSkillNativeProtobuf(t *testing.T) {
	var fixture struct {
		Payload string `json:"payload_hex"`
	}
	b, e := os.ReadFile("testdata/native_self_skills.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &fixture); e != nil {
		t.Fatal(e)
	}
	expected, e := hex.DecodeString(fixture.Payload)
	if e != nil {
		t.Fatal(e)
	}
	got, e := SkillInfo(1, []LearnedSkill{{ID: 179, Level: 7, Slot: 65535}, {ID: 174, Level: 1, Slot: 65535}, {ID: 169, Level: 1, Slot: 0}, {ID: 46, Level: 1, Slot: 1}, {ID: 190, Level: 1, Slot: 65535}, {ID: 5, Level: 1, Slot: 2}, {ID: 511, Level: 1, Slot: 65535}, {ID: 452, Level: 1, Slot: 65535}})
	if e != nil || !bytes.Equal(got, expected) {
		t.Fatalf("native skill protobuf mismatch: %x %v", got, e)
	}
	if _, e = SkillInfo(1, []LearnedSkill{{ID: 46, Level: 1, Slot: 0}, {ID: 46, Level: 2, Slot: 1}}); e == nil {
		t.Fatal("duplicate learned skill accepted")
	}
}

func TestSkillInfoSerializesCommandVectorField4(t *testing.T) {
	got, err := SkillInfo(1, []LearnedSkill{{ID: 109, Level: 1, Slot: 2, Commands: []uint32{8}}})
	if err != nil {
		t.Fatal(err)
	}
	// Field 4 is tag 32 (0x20) inside each repeated skill row. Both trees
	// carry the same row, matching the current native self-skill contract.
	if !bytes.Contains(got, []byte{0x20, 0x08}) {
		t.Fatalf("command vector field4 missing: %x", got)
	}
}
func TestDungeonDeathAndReturnNativeReaders(t *testing.T) {
	for _, row := range []struct {
		name string
		body []byte
	}{{"death_empty", MonsterDeathConfirmed(4096)}, {"selection_return", DungeonSelectionReturn()}} {
		b, e := os.ReadFile("testdata/native_" + row.name + "_cursor.json")
		if e != nil {
			t.Fatal(e)
		}
		var fixture struct {
			Payload string `json:"payload_hex"`
		}
		if e = json.Unmarshal(b, &fixture); e != nil {
			t.Fatal(e)
		}
		if hex.EncodeToString(row.body) != fixture.Payload {
			t.Fatal("native cursor mismatch", row.name)
		}
	}
}
