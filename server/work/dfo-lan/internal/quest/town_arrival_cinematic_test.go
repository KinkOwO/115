package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTownArrivalCinematicInitialProgressFollowsRuleAcrossQuestIDs(t *testing.T) {
	for _, id := range []uint32{21029, 12152, 32123} {
		d := catalog.QuestDefinition{ID: id, Kind: "[look cinematic]", ObjectiveCells: []pvf.Token{{Type: 0, Value: 21000}}, Script: catalog.ScriptRecord{Cells: []pvf.Token{
			{Type: 3, Text: "[move to dungeon]"}, {Type: 6, Text: "arrive in town"},
			{Type: 3, Text: "[int datas]"}, {Type: 0, Value: 39}, {Type: 0, Value: 1},
			{Type: 3, Text: "[dungeon]"}, {Type: 0, Value: 100004400}, {Type: 3, Text: "[/move to dungeon]"},
		}}}
		initial, model, err := InitialProgress(d)
		require.NoError(t, err)
		require.Equal(t, uint32(1), initial)
		require.Equal(t, TownArrivalCinematic, model)
		d.Script.Cells = nil
		initial, model, err = InitialProgress(d)
		require.NoError(t, err)
		require.Zero(t, initial)
		require.Equal(t, LookCinematic, model)
	}
}
