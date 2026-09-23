package main

import (
	"dfolan/internal/game/protocol"
	"testing"
)

// A skill cost names the store it is paid from by slot: the ordinary bag's
// material cells are 121..176 and the account-shared store's fixed cells are
// 363..379. Routing the request to the wrong store would charge the wrong
// stack, so the split is checked before anything is committed.
func TestSkillMaterialStorageRouting(t *testing.T) {
	for _, c := range []struct {
		name string
		rows []protocol.MaterialDelete
		want bool
	}{
		{"ordinary bag material cell", []protocol.MaterialDelete{{Slot: 121, Template: 3037, Count: 1}}, false},
		{"ordinary bag upper cell", []protocol.MaterialDelete{{Slot: 176, Template: 3037, Count: 1}}, false},
		{"shared store cell", []protocol.MaterialDelete{{Slot: 367, Template: 3037, Count: 15}}, true},
		{"shared store first cell", []protocol.MaterialDelete{{Slot: 363, Template: 3033, Count: 1}}, true},
		{"shared store last cell", []protocol.MaterialDelete{{Slot: 379, Template: 10361516, Count: 1}}, true},
		{"several shared cells", []protocol.MaterialDelete{
			{Slot: 367, Template: 3037, Count: 1},
			{Slot: 368, Template: 3262, Count: 1},
		}, true},
	} {
		got, err := skillMaterialStorageRows(c.rows)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: storage = %v, want %v", c.name, got, c.want)
		}
	}

	for _, c := range []struct {
		name string
		rows []protocol.MaterialDelete
	}{
		{"shared cell with another template", []protocol.MaterialDelete{{Slot: 367, Template: 3033, Count: 1}}},
		{"shared cell with a bag-only template", []protocol.MaterialDelete{{Slot: 363, Template: 3037, Count: 1}}},
		{"bag cell and shared cell together", []protocol.MaterialDelete{
			{Slot: 121, Template: 3037, Count: 1},
			{Slot: 367, Template: 3037, Count: 1},
		}},
	} {
		if _, err := skillMaterialStorageRows(c.rows); err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}
}
