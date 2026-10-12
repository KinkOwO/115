package catalog

import (
	"os"
	"strings"
	"testing"
)

func TestLegionRuntimeSourceContract(t *testing.T) {
	p := os.Getenv("DFO_REPAIR_PVF")
	if p == "" {
		t.Skip("native PVF required")
	}
	a, err := OpenTestArchiveCached(p, "")
	if err != nil {
		t.Fatal(err)
	}
	c, err := ImportLegionContents(a)
	if err != nil {
		t.Fatal(err)
	}
	text, e := a.ReadText(LegionContentsPath)
	if e != nil {
		t.Fatal(e)
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.Contains(line, "total probability rate") && i+1 < len(lines) {
			t.Logf("COS probability rate=%s %s", line, lines[i+1])
		}
	}
	for _, name := range []string{"Dimension Cloister", "DuskyIsland"} {
		x := c.Contents[name]
		if x.NativeOperations == nil || x.LastPhase != 2 {
			t.Fatalf("missing three-phase source: %s", name)
		}
		t.Logf("%s source=%s content=%+v", name, a.Snapshot().Checksum, x)
		s, err := ReadScript(a, x.NativeOperations.Path)
		if err != nil {
			t.Fatal(err)
		}
		for _, tag := range []string{"[extra operation select pool table]", "[week base time]"} {
			t.Logf("%s %s=%+v", name, tag, sectionCells(s.Cells, tag))
		}
		for _, id := range x.NativeOperations.Modes["normal"] {
			d := x.NativeOperations.Definitions[id]
			t.Logf("%s op%d type%d pool%d weight%d member=%s ranges=%+v formula=%+v", name, id, d.Type, d.SelectPool, d.Probability, d.MemberLimit, d.Ranges, d.Fields["[operation formula]"])
		}
		if x.RetryCount == nil || x.ReselectCount == nil {
			t.Fatal("source retry/reselection rules missing", name)
		}
		t.Logf("%s retry=%d reselect=%d", name, *x.RetryCount, *x.ReselectCount)
		for mode, ids := range x.NativeOperations.Modes {
			for _, id := range ids {
				d := x.NativeOperations.Definitions[id]
				t.Logf("%s/%s operation%d rewards=%+v", name, mode, id, d.Rewards)
				if d.Probability == 0 {
					continue
				}
				one := &NativeLegionOperations{Modes: map[string][]uint16{mode: {id}}, Definitions: map[uint16]NativeLegionOperation{id: d}}
				for _, last := range []bool{false, true} {
					draw := func(n int64) (int64, error) {
						if last {
							return n - 1, nil
						}
						return 0, nil
					}
					var card NativeLegionCard
					if name == "DuskyIsland" {
						card, err = one.DrawDuskyCard(mode, "party", "normal", d.SelectPool, draw)
					} else {
						card, err = one.DrawCard(mode, "party", "normal", d.SelectPool, draw)
					}
					if err != nil {
						t.Fatalf("%s/%s operation%d: %v", name, mode, id, err)
					}
					if card.Index != id {
						t.Fatal("source operation redirected", card)
					}
				}
			}
		}
		t.Logf("%s global clear rewards=%+v", name, x.NativeOperations.ClearRewards)
	}
}
