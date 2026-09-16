package protocol

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestQuestFinishCurrentNativeNoItemCursor(t *testing.T) {
	b, e := os.ReadFile("testdata/native_quest_finish_empty.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Payload  string `json:"payload_hex"`
		Consumed int
	}
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	p, e := QuestFinishedNoItems(3145, 1200)
	if e != nil || p[0] != 1 || hex.EncodeToString(p[1:]) != f.Payload || len(p) != f.Consumed+1 {
		t.Fatal("quest completion native cursor mismatch", e)
	}
	for _, ids := range [][]uint32{{3145, 3145}, {0}, {40000}} {
		if _, e = CompletedQuests(ids); e == nil {
			t.Fatal("invalid completion bitmap", ids)
		}
	}
}

func TestCompletedQuestsNativeBitmap(t *testing.T) {
	b, e := os.ReadFile("testdata/native_completed_quests.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Payload   string `json:"payload_hex"`
		Consumed  int
		IDs       []uint32 `json:"completed_ids"`
		Truncated string   `json:"truncation_failure"`
	}
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	p, e := CompletedQuests(f.IDs)
	if e != nil || hex.EncodeToString(p) != f.Payload || len(p) != f.Consumed || f.Truncated == "" {
		t.Fatal("current completion bitmap mismatch", e)
	}
}
