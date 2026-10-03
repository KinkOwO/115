package inventory

import (
	"dfolan/internal/testfixture"
	"testing"
)

func TestCompleteHistoricalEnhancementFixtureFamilies(t *testing.T) {
	c, err := ReadEnhancementBaseline(testfixture.EnhancementsDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.ReinforcementTickets) != 1196 || len(c.AmplifyTickets) != 1629 || len(c.Grimoires.Grimoires) != 433 || len(c.Enchant.Beads) != 4846 || len(c.Gold.Levels) != 255 || len(c.Amplify.Levels) != 255 {
		t.Fatal("complete historical enhancement fixtures lost rows")
	}
}
