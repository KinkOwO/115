package main

import (
	"bytes"
	"context"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

// Exercise the actual committed settlement and its packet plan in temporary
// SQLite. Combo restoration must not grant rewards twice or rewrite the layout.
func TestSettlementRestoresStoredDarkKnightCombo(t *testing.T) {
	for _, tc := range []struct {
		name  string
		job   byte
		combo []byte
		want  bool
	}{
		{"stored", 9, []byte{0, 1, 118, 0, 1, 46, 0}, true},
		{"legal_empty_cells", 9, []byte{0, 1, 118, 0, 0}, true},
		{"old_invalid", 9, []byte{255}, false},
		{"absent", 9, nil, false},
		{"other_profession", 0, []byte{0, 1, 118, 0, 1, 46, 0}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, f := robotSettlementFixture(t)
			w := c.worldState
			role := w.role
			role.ID, role.Profession, role.Name = 0, tc.job, "ComboResult"
			var state map[string]json.RawMessage
			if err := json.Unmarshal(role.State, &state); err != nil {
				t.Fatal(err)
			}
			state["combo_skill_info"], _ = json.Marshal(tc.combo)
			state["skill_preset"], _ = json.Marshal(make([]byte, protocol.SkillPresetConfigBytes))
			state["integration_unknown"] = json.RawMessage(`{"keep":true}`)
			role.State, _ = json.Marshal(state)
			var err error
			role, err = w.store.CreateCharacter(context.Background(), role, 10)
			if err != nil {
				t.Fatal(err)
			}
			role.WireID = 2
			w.role = role
			w.peer.roleID, c.selectedCharacterID = role.ID, role.ID
			chars := *f.manager.characters
			// The small settlement fixture has one synthetic profession. Reuse its
			// source attributes for the dark-knight wire/storage behavior under test.
			chars.Catalog.Professions[9] = chars.Catalog.Professions[0]
			chars.Store = w.store
			w.characters = &chars
			plan, err := w.dungeonResult(robotLiveResults(t)[0])
			if err != nil {
				t.Fatal(err)
			}
			skills, preset, combo := -1, -1, -1
			for i, packet := range plan {
				if packet.Kind != 0 {
					continue
				}
				switch packet.ID {
				case 19:
					skills = i
				case 2758:
					preset = i
				case 433:
					combo = i
				}
			}
			if skills < 0 || preset <= skills {
				t.Fatalf("skill restoration missing: %+v", plan)
			}
			if tc.want {
				if combo <= preset {
					t.Fatalf("combo must follow skills/preset: skills=%d preset=%d combo=%d", skills, preset, combo)
				}
				info, err := protocol.DecodeComboSkillInfo(tc.combo)
				if err != nil {
					t.Fatal(err)
				}
				want, err := protocol.EncodeComboSkillInfoNotify(info)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(plan[combo].Payload, want) {
					t.Fatal("not the saved S2C combo layout")
				}
			} else if combo >= 0 {
				t.Fatal("invalid/absent/other-profession layout replayed")
			}
			var after character.State
			if err := json.Unmarshal(w.role.State, &after); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after.ComboSkillInfo, tc.combo) {
				t.Fatal("settlement changed stored combo")
			}
			if !bytes.Contains(w.role.State, []byte(`"integration_unknown"`)) {
				t.Fatal("unknown save field lost")
			}
			settled := bytes.Clone(w.role.State)
			if _, err = w.dungeonResult(robotLiveResults(t)[0]); err != nil || !bytes.Equal(settled, w.role.State) {
				t.Fatalf("duplicate settlement changed save: %v", err)
			}
			w.resultSent = true
			if again, err := w.dungeonResult(robotLiveResults(t)[0]); err != nil || len(again) != 0 {
				t.Fatal("sent result duplicated", err)
			}
		})
	}
}
