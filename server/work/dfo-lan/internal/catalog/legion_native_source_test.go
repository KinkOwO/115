package catalog

import (
	"os"
	"testing"
)

func TestLegionNativeCurrentSource(t *testing.T) {
	path := os.Getenv("DFO_REPAIR_PVF")
	if path == "" {
		t.Skip("current PVF required")
	}
	a, err := OpenTestArchiveCached(path, "")
	if err != nil {
		t.Fatal(err)
	}
	c, err := ImportLegionContents(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Dimension Cloister", "DuskyIsland"} {
		x := c.Contents[name]
		if x.NativeOperations == nil || len(x.NativeOperations.Definitions) == 0 {
			t.Fatal("missing native operations", name)
		}
		t.Logf("%s: phases=%d dungeons=%v modes=%v weekly=%v", name, x.LastPhase, x.Dungeons, x.NativeOperations.Modes, x.NativeOperations.WeeklyDungeons)
	}
	for _, name := range []string{"Venus Legion", "Apocalypse"} {
		t.Logf("%s rewards=%+v", name, c.Contents[name].Rewards)
	}
	op, err := ImportApocalypse(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range op.Operations {
		t.Logf("operation=%v type=%v reward=%+v", *row.Index, *row.Type, row.Reward)
	}
	venus := c.Contents["Venus Legion"].NativeOperations
	if venus == nil || len(venus.Definitions) != 4 {
		t.Fatal("Venus native operations missing", venus)
	}
}
