package database

import (
	"encoding/json"
	"testing"
)

func TestAdvanceStoryDigestState(t *testing.T) {
	initial := json.RawMessage(`{"cinematic_skipped":[7],"custom":{"keep":true}}`)
	advanced, changed, err := advanceStoryDigestState(initial, 115)
	if err != nil || !changed {
		t.Fatalf("advance: changed=%v err=%v", changed, err)
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(advanced, &state); err != nil {
		t.Fatal(err)
	}
	if string(state["story_digest_level"]) != "115" || string(state["cinematic_skipped"]) != "[7]" || string(state["custom"]) != `{"keep":true}` {
		t.Fatalf("other character state changed: %s", advanced)
	}
	for _, level := range []uint32{100, 115} {
		got, changed, err := advanceStoryDigestState(advanced, level)
		if err != nil || changed || string(got) != string(advanced) {
			t.Fatalf("regressed at %d: state=%s changed=%v err=%v", level, got, changed, err)
		}
	}
}
