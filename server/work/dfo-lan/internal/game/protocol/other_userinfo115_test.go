package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestOtherUserInfo115NativeReadVector(t *testing.T) {
	data, err := os.ReadFile("testdata/native_other_userinfo115.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		Payload  string `json:"payload_hex"`
		Consumed int    `json:"consumed"`
	}
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString(vector.Payload)
	if err != nil {
		t.Fatal(err)
	}
	s := OtherUserInfo115{ActorServerID: 65534, Experience: 0x1122334455667788,
		Stats:      PackedEntryStats{HP: 5200, MP: 4800, BasePercent: 100},
		SkillTrees: [2][]EntrySkill{{{179, 7}}, {{174, 1}, {169, 1}}}}
	got, err := OtherUserInfoReply115(s)
	if err != nil {
		t.Fatal(err)
	}
	if vector.Consumed != len(got) || !bytes.Equal(got, want) {
		t.Fatalf("native mode3 mismatch %d/%d", len(got), len(want))
	}
}

func TestOtherUserInfo115NonemptyNativeFixture(t *testing.T) {
	s := OtherUserInfo115{ActorServerID: 65534, Stats: PackedEntryStats{HP: 5200, MP: 4800, BasePercent: 100},
		Worn: []DetailedWorn{{Slot: 12, Template: 27000, Durability: 25}, {Slot: 0, Template: 39000, AvatarOptions: []byte{1, 2}, AvatarSockets: []byte{3, 4}}}}
	p, err := OtherUserInfoReply115(s)
	if err != nil {
		t.Fatal(err)
	}
	// Optional file is an offline input to the immutable native PE emulator.
	if path := os.Getenv("DFO_ROBOT_INSPECT_VECTOR_OUTPUT"); path != "" {
		b, err := json.Marshal(map[string]any{"payload_hex": hex.EncodeToString(p), "types": map[uint32]int{27000: 12, 39000: 0}})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOtherUserInfo115RejectsInvalidIdentityAndInstances(t *testing.T) {
	s := OtherUserInfo115{ActorServerID: 7, Stats: PackedEntryStats{HP: 1, MP: 1, BasePercent: 100}}
	for _, id := range []uint16{0, 65535} {
		s.ActorServerID = id
		if _, err := OtherUserInfoReply115(s); err == nil {
			t.Fatal("invalid actor accepted")
		}
	}
	s.ActorServerID = 7
	s.Worn = []DetailedWorn{{Slot: 48, Template: 27000}}
	if _, err := OtherUserInfoReply115(s); err == nil {
		t.Fatal("out-of-table worn slot accepted")
	}
	s.Worn = nil
	s.SkillTrees[0] = []EntrySkill{{ID: 1, Level: 1}, {ID: 1, Level: 2}}
	if _, err := OtherUserInfoReply115(s); err == nil {
		t.Fatal("duplicate skills accepted")
	}
}
