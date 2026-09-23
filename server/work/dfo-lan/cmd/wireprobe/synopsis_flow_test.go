package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSynopsisReadPreservesSaveAndEarlierReads(t *testing.T) {
	raw := json.RawMessage(`{"gold":12345,"inventory":{"items":[1,2]},"future_field":{"keep":true}}`)
	first, err := synopsisReadState(raw, 7)
	if err != nil {
		t.Fatal(err)
	}
	next, err := synopsisReadState(first, 2)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := synopsisReadState(next, 2)
	if err != nil || !bytes.Equal(next, replay) {
		t.Fatalf("replay changed state: %s %v", replay, err)
	}
	var before, after map[string]json.RawMessage
	json.Unmarshal(raw, &before)
	json.Unmarshal(next, &after)
	for k, v := range before {
		if !bytes.Equal(v, after[k]) {
			t.Fatalf("changed %s", k)
		}
	}
	p, err := synopsisRestore(replay)
	if err != nil || !bytes.Equal(p, []byte{0, 2, 0, 2, 0, 0, 0, 7, 0, 0, 0}) {
		t.Fatalf("restore=%x %v", p, err)
	}
	p, err = synopsisRestore(raw)
	if err != nil || !bytes.Equal(p, []byte{0, 0, 0}) {
		t.Fatalf("old save=%x %v", p, err)
	}
	for _, bad := range []string{`null`, `[]`, `{"synopsis_read":"bad"}`, `{"synopsis_read":[2147483648]}`} {
		if _, err := synopsisReadState([]byte(bad), 2); err == nil {
			t.Fatalf("accepted invalid state %s", bad)
		}
	}
}

func TestSynopsisRestoreAfterQuestLists(t *testing.T) {
	p, _ := synopsisRestore([]byte(`{"synopsis_read":[2]}`))
	packets := (entryPayloads{SynopsisRead: p}).packets()
	questAt, synopsisAt, locksAt := -1, -1, -1
	for i, packet := range packets {
		switch packet.Name {
		case "available_quests_restored":
			questAt = i
		case "synopsis_read_restored":
			synopsisAt = i
			if packet.Kind != 0 || packet.ID != 2310 || !bytes.Equal(packet.Payload, p) {
				t.Fatal("wrong synopsis packet")
			}
		case "skill_locks_restored":
			locksAt = i
		}
	}
	if questAt < 0 || synopsisAt <= questAt || locksAt <= synopsisAt {
		t.Fatal("wrong restore order")
	}
	if !observedGameRequest(2079) {
		t.Fatal("CMD2079 not retained")
	}
}
