package character

import (
	"encoding/json"
	"testing"
)

func TestBestTimePersistsAndSlowerClearDoesNotAnnounceRecord(t *testing.T) {
	raw := json.RawMessage(`{"inventory":{"gold":123},"level":5}`)
	raw, best, fresh, e := saveClearRecord(raw, 3, 60000)
	if e != nil || best != 60000 || !fresh {
		t.Fatal(best, fresh, e)
	}
	raw, best, fresh, e = saveClearRecord(raw, 3, 71000)
	if e != nil || best != 60000 || fresh {
		t.Fatal(best, fresh, e)
	}
	raw, best, fresh, e = saveClearRecord(raw, 3, 52000)
	if e != nil || best != 52000 || !fresh {
		t.Fatal(best, fresh, e)
	}
	raw, best, fresh, e = saveClearRecord(raw, 4, 99000)
	if e != nil || best != 99000 || !fresh {
		t.Fatal(best, fresh, e)
	}
	var state map[string]json.RawMessage
	json.Unmarshal(raw, &state)
	if string(state["inventory"]) != `{"gold":123}` {
		t.Fatal("inventory changed")
	}
}
