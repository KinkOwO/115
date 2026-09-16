package protocol

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestQuestTriggerNativeRestore(t *testing.T) {
	b, err := os.ReadFile("testdata/native_quest_triggers.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Payload  string     `json:"payload_hex"`
		Triggers [][]uint32 `json:"triggers"`
		Reset    bool       `json:"reset"`
		Consumed int        `json:"consumed"`
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatal("missing pending/ready native fixtures")
	}
	for _, row := range rows {
		if !row.Reset || len(row.Triggers) != 1 || len(row.Triggers[0]) != 2 {
			t.Fatal("native effect missing")
		}
		p, err := QuestTriggers([]ActiveQuest{{uint16(row.Triggers[0][0]), row.Triggers[0][1]}})
		if err != nil || hex.EncodeToString(p) != row.Payload || len(p) != row.Consumed {
			t.Fatal("native trigger restore mismatch", err)
		}
	}
	if _, err = QuestTriggers([]ActiveQuest{{3145, 0}, {3145, 1}}); err == nil {
		t.Fatal("duplicate quest restore accepted")
	}
}
