package legion

import (
	"fmt"
	"strings"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
)

// Exercises the source reader and the actual reward consumer together. The
// shared COS reader retaining repeated party blocks must not stop solo clear.
func TestApocalypseBasicRewardSelectsSoloFromRepeatedCOSBlocks(t *testing.T) {
	for _, values := range [][2]int64{{17, 3}, {29, 7}} {
		text := fmt.Sprintf("[legion data set]\n[contents] `Apocalypse`\n"+
			"[reward data]\n[party type] `party`\n[data]\n`normal` `basic` 500000 80 30\n`expert` `basic` 1000000 81 40\n[/data]\n[/reward data]\n"+
			"[reward data]\n[party type] `solo`\n[data]\n`normal` `basic` 1000000 %d %d\n`expert` `basic` 1000000 %d %d\n`normal` `pcroom` 1000000 82 50\n[/data]\n[/reward data]\n"+
			"[/legion data set]\n", values[0], values[1], values[0]+1, values[1]+1)
		parsed, err := catalog.ParseLegionContents(text)
		if err != nil {
			t.Fatal(err)
		}
		if len(parsed["Apocalypse"].Rewards) != 5 {
			t.Fatal("party source rows were discarded")
		}
		source := strings.Repeat("b", 64)
		contents := catalog.LegionContents{Source: pvf.ArchiveSnapshot{Checksum: source}, SHA256: strings.Repeat("c", 64), Contents: parsed}
		for i, diff := range []string{"normal", "expert"} {
			item, err := GuaranteedBasicClearReward(contents, source, diff)
			if err != nil || item.Template != uint32(values[0])+uint32(i) || item.Amount != uint32(values[1])+uint32(i) {
				t.Fatal("source solo reward not selected", diff, item, err)
			}
		}
		c := contents.Contents["Apocalypse"]
		c.Rewards = append(c.Rewards, c.Rewards[2])
		contents.Contents["Apocalypse"] = c
		if _, err := GuaranteedBasicClearReward(contents, source, "normal"); err == nil {
			t.Fatal("ambiguous duplicate solo basic reward accepted")
		}
		c.Rewards = c.Rewards[:2]
		contents.Contents["Apocalypse"] = c
		if _, err := GuaranteedBasicClearReward(contents, source, "expert"); err == nil {
			t.Fatal("party-only reward substituted for missing solo reward")
		}
	}
}
