package character

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"testing"
)

func TestAvatarDetailCandidateIsolation(t *testing.T) {
	state := json.RawMessage(`{"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100},"inventory":{"worn":[{"slot":3,"template":40601,"durability":9},{"slot":12,"template":101000013},{"slot":47,"template":100610096}]}}`)
	before := append([]byte(nil), state...)
	role := storage.Character{WireID: 503, State: state}
	service := Service{}
	baseline, e := service.EntryAddition(role)
	if e != nil {
		t.Fatal(e)
	}
	service.DetailedWornCandidate = true
	modified, e := service.EntryAddition(role)
	if e != nil {
		t.Fatal(e)
	}
	block, e := protocol.DetailedEquipment([]protocol.DetailedWorn{{Slot: 3, Template: 40601, Durability: 9}})
	if e != nil {
		t.Fatal(e)
	}
	if len(modified)-len(baseline) != 135 || !bytes.Contains(modified, block) {
		t.Fatal("avatar-only initialization missing")
	}
	if !bytes.Equal(state, before) {
		t.Fatal("read projection mutated stored items")
	}
	service.DetailedWornCandidate = false
	restored, e := service.EntryAddition(role)
	if e != nil || !bytes.Equal(restored, baseline) {
		t.Fatal("disabled candidate changed original packet")
	}
}
