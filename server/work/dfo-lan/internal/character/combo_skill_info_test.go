package character

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/hex"
	"encoding/json"
	"testing"
)

const observedComboBody = "00067600052e006c000500080075007700007800007900007a00007b00000000"

func comboBody(t *testing.T) []byte {
	t.Helper()
	raw, err := hex.DecodeString(observedComboBody)
	if err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return raw
}

// The new state field must survive a skill-mutation merge untouched, and an
// explicit clear must actually remove it. mergeSkillState only writes the keys
// the new state marshals, so this is what the no-omitempty tag buys.
func TestComboSkillInfoSurvivesMergeAndClears(t *testing.T) {
	body := comboBody(t)

	state := State{Level: 1, SourceSHA256: "source", ComboSkillInfo: body}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var original map[string]json.RawMessage
	if err := json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	original["future_state"] = json.RawMessage(`{"keep":true}`)
	raw, err = json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := mergeSkillState(raw, state)
	if err != nil {
		t.Fatal(err)
	}
	var kept State
	if err := json.Unmarshal(merged, &kept); err != nil {
		t.Fatal(err)
	}
	if string(kept.ComboSkillInfo) != string(body) {
		t.Fatalf("body did not survive the merge: %x", kept.ComboSkillInfo)
	}

	var cleared State
	if err := json.Unmarshal(merged, &cleared); err != nil {
		t.Fatal(err)
	}
	cleared.ComboSkillInfo = nil
	next, err := mergeSkillState(merged, cleared)
	if err != nil {
		t.Fatal(err)
	}
	var back State
	if err := json.Unmarshal(next, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.ComboSkillInfo) != 0 {
		t.Fatalf("clear did not take: %x", back.ComboSkillInfo)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(next, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["future_state"]) != `{"keep":true}` {
		t.Fatal("clear lost unrelated save fields")
	}
}

func TestEntryComboSkillInfoReplaysTheStoredBody(t *testing.T) {
	body := comboBody(t)
	raw, err := json.Marshal(State{Level: 1, ComboSkillInfo: body})
	if err != nil {
		t.Fatal(err)
	}
	s := Service{}
	got, err := s.EntryComboSkillInfo(storage.Character{ID: 1, Profession: 9, State: raw})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("entry body = %x, want %x", got, body)
	}
}

// An absent body and an undecodable body both have to yield "nothing to send"
// rather than an entry failure: the pre-fix client never pushed one, and a
// stale save must not wedge character selection.
func TestEntryComboSkillInfoIsEmptyWhenNothingIsStored(t *testing.T) {
	s := Service{}
	for name, raw := range map[string]json.RawMessage{
		"no state":    nil,
		"empty state": json.RawMessage(`{}`),
		"null field":  json.RawMessage(`{"combo_skill_info":null}`),
		"undecodable": json.RawMessage(`{"combo_skill_info":"AQID"}`),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := s.EntryComboSkillInfo(storage.Character{ID: 1, Profession: 9, State: raw})
			if err != nil {
				t.Fatalf("entry: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("entry body = %x, want empty", got)
			}
		})
	}
}

func TestSaveComboSkillInfoRequiresAnOwnedCharacter(t *testing.T) {
	s := Service{}
	req, err := protocol.DecodeComboSkillInfo(comboBody(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SaveComboSkillInfo(context.Background(), storage.Character{ID: 1, AccountID: 1}, "key", req); err == nil {
		t.Fatal("save without a store succeeded")
	}
	if _, _, err := s.ClearComboSkillInfo(context.Background(), storage.Character{ID: 1, AccountID: 1}, "key"); err == nil {
		t.Fatal("clear without a store succeeded")
	}
}

func TestDarkKnightDefaultSkillRowsKeepAllSixComboSlots(t *testing.T) {
	c, err := catalog.LoadCharacters("../../configs/characters.skycastle-release.json")
	if err != nil {
		t.Fatal(err)
	}
	prof := c.Professions[9]
	s := Service{Catalog: c}
	state := State{Level: 1, SourceSHA256: prof.RawSHA256, InitialSkills: prof.InitialSkills}
	rows, err := s.skillRows(storage.Character{Profession: 9}, state, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[uint16]bool{}
	for _, r := range rows {
		if r.ID >= 118 && r.ID <= 123 {
			if r.Slot != r.ID-118 {
				t.Fatalf("combo %d projected to slot %d", r.ID, r.Slot)
			}
			seen[r.ID] = true
		} else if r.Slot < 6 {
			t.Fatalf("ordinary skill %d occupies combo slot %d", r.ID, r.Slot)
		}
	}
	if len(seen) != 6 {
		t.Fatalf("new dark knight has %d combo skills, want 6", len(seen))
	}
}
