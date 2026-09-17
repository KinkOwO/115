package progression

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestSceneActorExperienceDoesNotRequireMonsterTables(t *testing.T) {
	for _, m := range []protocol.DungeonMonster{
		{Template: 55424, Rank: 5, APC: true, Level: 0, Team: 100},
		{Template: 109019135, Rank: 0, Level: 0, Team: 100},
	} {
		gain, e := MonsterGain(catalog.Progression{}, Rules{}, catalog.DungeonDefinition{}, m, 115, 2)
		if e != nil || gain != 0 {
			t.Fatal(gain, e)
		}
	}
}
