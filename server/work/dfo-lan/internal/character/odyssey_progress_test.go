package character

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestOdysseyJournalUsesOnlyConfirmedClears(t *testing.T) {
	s, r := odysseyGrowthFixture(t)
	r, err := s.ApplyOdysseyTarget(r, 115)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.OdysseyProgressPayload(r)
	if err != nil || !bytes.Equal(p, make([]byte, 280)) {
		t.Fatal("level alone unlocked journal", err)
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(r.State, &fields)
	fields["dungeon_best_times"] = json.RawMessage(`{"100004934:normal:solo":35000,"100004935:normal:solo":0,"3:normal:solo":100}`)
	fields["untouched"] = json.RawMessage(`42`)
	r.State, _ = json.Marshal(fields)
	r.State, err = s.saveOdysseyCompletion(r, 100004935)
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.OdysseyProgressPayload(r)
	if err != nil || len(p) != 280 || binary.LittleEndian.Uint32(p) != 100004934 || binary.LittleEndian.Uint32(p[4:]) != 100004935 || !bytes.Equal(p[8:], make([]byte, 272)) {
		t.Fatal("journal merge", p, err)
	}
	again, err := s.saveOdysseyCompletion(r, 100004935)
	if err != nil || !bytes.Equal(again, r.State) {
		t.Fatal("repeat changed progress", err)
	}
	json.Unmarshal(again, &fields)
	if string(fields["untouched"]) != "42" {
		t.Fatal("state lost")
	}
	if _, err := s.saveOdysseyCompletion(r, 3); err == nil {
		t.Fatal("non-source dungeon accepted")
	}
	r.ConfigVersion = "other"
	if _, err := s.OdysseyProgressPayload(r); err == nil {
		t.Fatal("wrong source accepted")
	}
	r.Request = nil
	if p, err := s.OdysseyProgressPayload(r); err != nil || p != nil {
		t.Fatal("ordinary role affected", err)
	}
}
