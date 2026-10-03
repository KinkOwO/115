package gamedata

import (
	"dfolan/internal/catalog"
	"os"
	"testing"
)

func TestOdysseyCompletionNativeStartup(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native read-only PVF required")
	}
	prior := catalog.OdysseySource
	defer catalog.SetOdysseySource(prior)
	c, err := PrepareCatalogs(CatalogInputs{
		Selection: "characters,odyssey-growth,odyssey-chapters", ArchivePath: path,
		IndexPath: "../../configs/items.index.json", ContentPolicyPath: "../../configs/pvf-mine-policy.json",
		CharacterPolicyPath: "../../configs/pvf-character-policy.json",
	}, CatalogAdapters{ValidatedSource: catalog.SetOdysseySource})
	if err != nil {
		t.Fatal(err)
	}
	if c.OdysseyCompletionRewards == nil || c.OdysseyGrowth.GraduateReward != c.OdysseyCompletionRewards.Honor.Template {
		t.Fatal("native honor mapping mismatch")
	}
	if err = c.OdysseyCompletionRewards.ValidateItems(*c.Items); err != nil {
		t.Fatal(err)
	}
	for _, ch := range c.OdysseyChapters.Chapters {
		r := c.OdysseyCompletionRewards.At(ch.Number)
		if len(r) != len(ch.Rewards) {
			t.Fatal("chapter mapping mismatch", ch.Number)
		}
		for i, item := range r {
			if item != ch.Rewards[i] {
				t.Fatal("native reward/quantity mismatch", ch.Number, item, ch.Rewards[i])
			}
		}
	}
}
