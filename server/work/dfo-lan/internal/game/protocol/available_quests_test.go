package protocol

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestAvailableQuestsNativeFields(t *testing.T) {
	b, e := os.ReadFile("testdata/native_available_quests.json")
	if e != nil {
		t.Fatal(e)
	}
	var r struct {
		Payload string `json:"payload_hex"`
	}
	if e = json.Unmarshal(b, &r); e != nil {
		t.Fatal(e)
	}
	p, e := AvailableQuests(5, []uint32{4873, 3146})
	if e != nil || hex.EncodeToString(p) != r.Payload {
		t.Fatal("current quest schema mismatch", e)
	}
}
