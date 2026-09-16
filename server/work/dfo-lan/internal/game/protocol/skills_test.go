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
	got, e := SkillInfo(1, []LearnedSkill{{179, 7, 65535}, {174, 1, 65535}, {169, 1, 0}, {46, 1, 1}, {190, 1, 65535}, {5, 1, 2}, {511, 1, 65535}, {452, 1, 65535}})
	if e != nil || !bytes.Equal(got, expected) {
		t.Fatalf("native skill protobuf mismatch: %x %v", got, e)
	}
	if _, e = SkillInfo(1, []LearnedSkill{{46, 1, 0}, {46, 2, 1}}); e == nil {
		t.Fatal("duplicate learned skill accepted")
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
