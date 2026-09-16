package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestNativeMutationsAndUnlock(t *testing.T) {
	buy, e := SkillPurchaseSuccess(0, 10, 0, []LearnedSkill{{ID: 46, Level: 2, Slot: 14}})
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range []struct {
		name string
		body []byte
	}{{"skill_mutation_28", SkillMoveSuccess(SkillMove{0, 1, 3})[1:]}, {"skill_mutation_29", buy[1:]}, {"delete_1", DeleteCharacterSuccess(2)[1:]}} {
		p, e := os.ReadFile("testdata/native_" + r.name + ".json")
		if e != nil {
			t.Fatal(e)
		}
		var v struct {
			Payload string `json:"payload_hex"`
		}
		if e = json.Unmarshal(p, &v); e != nil {
			t.Fatal(e)
		}
		if hex.EncodeToString(r.body) != v.Payload {
			t.Fatal(r.name, "native mismatch")
		}
	}
}
func TestCurrentLearningRequestTail(t *testing.T) {
	p := []byte{0, 1, 46, 0, 0, 1, 0, 0, 0, 0}
	p = append(p, requestDigest(p)...)
	p = append(p, make([]byte, 2)...)
	r, e := DecodeSkillPurchase(p)
	if e != nil || r.Entries[0].ID != 46 {
		t.Fatal(r, e)
	}
	for _, i := range []int{4, 5, 6, 7, 8, 9, 15} {
		bad := bytes.Clone(p)
		bad[i] = 2
		if i == 5 {
			bad[i] = 0
		}
		if _, e = DecodeSkillPurchase(bad); e == nil {
			t.Fatal("accepted bad field", i)
		}
	}
	m := []byte{0, 1, 3, 255, 255, 255, 255}
	m = append(m, requestDigest(m)...)
	m = append(m, make([]byte, 5)...)
	if _, e = DecodeSkillMove(m); e != nil {
		t.Fatal(e)
	}
	name := "DeleteMe"
	d := binary.LittleEndian.AppendUint32([]byte{2, 0}, uint32(len(name)))
	d = append(d, []byte(name)...)
	d = append(d, 0, 0)
	x, e := DecodeDeleteCharacter(d)
	if e != nil || x.Name != name || x.Slot != 2 {
		t.Fatal(x, e)
	}
	d[len(d)-1] = 1
	if _, e = DecodeDeleteCharacter(d); e == nil {
		t.Fatal("delete nonzero tail")
	}
}
