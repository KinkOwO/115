package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestPackedStatsAgainstNativeExpansion(t *testing.T) {
	raw, e := os.ReadFile("testdata/native_packed91.json")
	if e != nil {
		t.Fatal(e)
	}
	var vectors []struct {
		WireHex string   `json:"wire_hex"`
		Words   []uint32 `json:"expanded_words"`
	}
	if e = json.Unmarshal(raw, &vectors); e != nil {
		t.Fatal(e)
	}
	for _, v := range vectors {
		if len(v.Words) != 40 {
			t.Fatal("native vector field count")
		}
		i := 0
		next := func() uint32 { x := v.Words[i]; i++; return x }
		s := PackedEntryStats{HP: next(), MP: next()}
		for j := range s.Core {
			s.Core[j] = uint16(next())
		}
		for j := range s.Element {
			s.Element[j] = int16(next())
		}
		for j := range s.Status {
			s.Status[j] = int16(next())
		}
		s.Inventory = int32(next())
		for j := range s.Regeneration {
			s.Regeneration[j] = int16(next())
		}
		s.Movement = next()
		for j := range s.AttackCasting {
			s.AttackCasting[j] = uint16(next())
		}
		for j := range s.RecoveryJump {
			s.RecoveryJump[j] = int16(next())
		}
		s.Weight = int32(next())
		s.BasePercent = byte(next())
		s.Extra = math.Float32frombits(next())
		got, e := s.Bytes()
		if e != nil {
			t.Fatal(e)
		}
		want, e := hex.DecodeString(v.WireHex)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("native expansion reconstruction mismatch: %x", got)
		}
	}
}

func TestAdditionConsumesBothNativeSkillTrees(t *testing.T) {
	s := EntryAdditionProbe{ActorServerID: 3, Experience: 0x1122334455667788, Stats: PackedEntryStats{HP: 5200, MP: 4800, BasePercent: 100}, SkillTrees: [2][]EntrySkill{{{ID: 179, Level: 7}}, {{ID: 174, Level: 1}, {ID: 169, Level: 1}}}}
	p, e := UserInfoAdditionProbe(s)
	if e != nil {
		t.Fatal(e)
	}
	if len(p) != 518 {
		t.Fatalf("addition size %d", len(p))
	}
	fixture, e := os.ReadFile("testdata/native_addition_cursor.json")
	if e != nil {
		t.Fatal(e)
	}
	var native struct {
		Payload  string `json:"payload_hex"`
		Consumed int    `json:"consumed"`
	}
	if e = json.Unmarshal(fixture, &native); e != nil {
		t.Fatal(e)
	}
	want, e := hex.DecodeString(native.Payload)
	if e != nil || !bytes.Equal(p, want) || native.Consumed != len(p) {
		t.Fatal("packet differs from independent native cursor fixture")
	}
	if binary.LittleEndian.Uint16(p[255:]) != 3 || binary.LittleEndian.Uint64(p[257:]) != s.Experience || binary.LittleEndian.Uint32(p[265:]) != 91 {
		t.Fatal("prefix/identity/experience/stats boundary")
	}
	if !bytes.Equal(p[386:391], []byte{255, 1, 179, 0, 7}) {
		t.Fatal("first skill tree boundary")
	}
	if !bytes.Equal(p[444:451], []byte{2, 174, 0, 1, 169, 0, 1}) {
		t.Fatal("second skill tree boundary")
	}
	s.SkillTrees[1] = append(s.SkillTrees[1], s.SkillTrees[1][0])
	if _, e = UserInfoAdditionProbe(s); e == nil {
		t.Fatal("duplicate skill accepted")
	}
	s.Stats.BasePercent = 0
	if _, e = s.Stats.Bytes(); e == nil {
		t.Fatal("zero base percentage accepted")
	}
}
