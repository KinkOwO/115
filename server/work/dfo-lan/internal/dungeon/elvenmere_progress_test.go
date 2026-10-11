package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

func TestElvenmereProgressPreservesUnknownFields(t *testing.T) {
	raw := json.RawMessage(`{"other":{"keep":42},"elvenmere_progress":{"version":1,"max_cleared":5,"future":{"keep":99},"weekly":{"5":{"event_key":"old"}}}}`)
	p, exists, err := ReadElvenmereProgress(raw)
	if err != nil || !exists {
		t.Fatal(p, err)
	}
	p.MaxCleared = 6
	after, err := WriteElvenmereProgress(raw, p)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(after, &fields); err != nil {
		t.Fatal(err)
	}
	var ledger map[string]json.RawMessage
	if err := json.Unmarshal(fields["elvenmere_progress"], &ledger); err != nil {
		t.Fatal(err)
	}
	if string(fields["other"]) != `{"keep":42}` || string(ledger["future"]) != `{"keep":99}` {
		t.Fatal("lost unknown state", string(after))
	}
}

func TestElvenmereConsumedCompletionRequiresFreshLoad(t *testing.T) {
	s := &Session{Definition: catalog.DungeonDefinition{ID: 100003126}, Loaded: true}
	s.MarkSceneCompleted()
	s.AdvanceElvenmereFloor(5, 6)
	if s.Completed() || s.Loaded || s.completionTarget != 0 {
		t.Fatal("completion not consumed")
	}
	request := protocol.BossCheckRequest{Actor: 1, Target: 99}
	if err := s.BossCheck(request, 1); err == nil {
		t.Fatal("old boss report accepted without a new load")
	}
	s.Loaded = true
	if err := s.BossCheck(request, 1); err != nil || !s.Completed() {
		t.Fatal("new owned load could not complete", err)
	}
}
