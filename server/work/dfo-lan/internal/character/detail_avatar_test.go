package character

import (
	"bytes"
	"dfolan/internal/game/protocol"

	"encoding/binary"
	"encoding/json"
	"testing"
)

func binary32(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

func TestAvatarDetailCandidateIsolation(t *testing.T) {
	state := json.RawMessage(`{"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100},"inventory":{"worn":[{"slot":3,"template":40601,"durability":9},{"slot":12,"template":101000013},{"slot":26,"template":500991361},{"slot":47,"template":100610096}]}}`)
	before := append([]byte(nil), state...)
	role := Character{WireID: 503, State: state}
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
	// The worn projection keeps avatar slot 3, now also carries the creature
	// body slot 26 and the ordinary weapon slot 12. Slot 47 stays excluded
	// from this projection; its separate worn-container restore remains intact.
	block, e := protocol.DetailedEquipment([]protocol.DetailedWorn{{Slot: 3, Template: 40601, Durability: 9}, {Slot: 12, Template: 101000013}, {Slot: 26, Template: 500991361}})
	if e != nil {
		t.Fatal(e)
	}
	// Delta over the empty 14-byte equipment block: avatar row 135 +
	// ordinary row 127 + creature row 132.
	if len(modified)-len(baseline) != 394 || !bytes.Contains(modified, block) {
		t.Fatal("avatar+weapon+creature mode1 projection mismatch")
	}
	if bytes.Contains(modified, binary32(100610096)) {
		t.Fatal("unsupported slot-47 row leaked into mode1")
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

func TestAvatarDetailProjectsCoexistingCloneAndAppearance(t *testing.T) {
	state := json.RawMessage(`{"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100},"inventory":{"worn":[{"slot":1,"template":517560000},{"slot":1,"template":517562678,"group":1}]}}`)
	s := Service{DetailedWornCandidate: true}
	packet, err := s.EntryAddition(Character{WireID: 503, State: state})
	if err != nil {
		t.Fatal(err)
	}
	block, err := protocol.DetailedEquipment([]protocol.DetailedWorn{{
		Slot: 1, Template: 517560000,
		HeaderTemplateA: 517562678,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(packet, block) {
		t.Fatal("entry detail did not carry clear-avatar base and ordinary-look override")
	}
}
