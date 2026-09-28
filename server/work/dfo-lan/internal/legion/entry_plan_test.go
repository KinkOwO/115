package legion

import (
	"dfolan/internal/catalog"
	"testing"
)

func TestApocalypseChoiceUsesZeroBasedNativeKey(t *testing.T) {
	c, e := catalog.LoadApocalypseCatalog("../../configs/apocalypse.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	clock, e := NewApocalypseClock(c)
	if e != nil {
		t.Fatal(e)
	}
	for _, choice := range []byte{0, 1, 2, 4} {
		p, e := PlanForChoice(c, clock, choice)
		if e != nil || p.OperationID != uint32(choice)+1 {
			t.Fatal(choice, p, e)
		}
	}
	for _, choice := range []byte{3, 5, 255} {
		if _, e := PlanForChoice(c, clock, choice); e == nil {
			t.Fatal("undeclared choice accepted", choice)
		}
	}
}

func TestApocalypseEntryPlanUsesSourceDestinations(t *testing.T) {
	c, e := catalog.LoadApocalypseCatalog("../../configs/apocalypse.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	dc, e := catalog.LoadDungeons("../../configs/dungeons.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	contents, e := catalog.LoadLegionContents("../../configs/legion-contents.generated.json", dc.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	// Use the small source dungeon fixture to verify destination selection and
	// validation without loading the300MB full snapshot in every unit test.
	donor := dc.Dungeons[3]
	content := contents.Contents["Apocalypse"]
	for _, row := range content.Dungeons {
		d := donor
		d.ID = row.Dungeon
		d.MinimumLevel = 115
		d.Mazes = nil
		for _, maze := range donor.Mazes {
			maze.Quest = 0
			d.Mazes = append(d.Mazes, maze)
		}
		dc.Dungeons[row.Dungeon] = d
	}
	p, e := BuildEntryPlan(contents, c, dc)
	if e != nil {
		t.Fatal(e)
	}
	if p.Destinations[0] != 100005112 || p.Destinations[5] != 100004994 || p.Waiting.X != 727 || p.MinimumLevel != 115 {
		t.Fatal(p)
	}
	delete(dc.Dungeons, content.Dungeons[2].Dungeon)
	if _, e = BuildEntryPlan(contents, c, dc); e == nil {
		t.Fatal("missing phase accepted")
	}
}

func TestApocalypseStageUsesPublishedDestinationIndex(t *testing.T) {
	p := EntryPlan{Destinations: [6]uint32{100005112, 100004995, 100005057, 100004918, 100005111, 100004994}}
	targets := [6]byte{0, 3, 255, 255, 255, 255}
	got, e := p.ResolveStageDestination(1, targets)
	if e != nil || got != 100004918 {
		t.Fatal("stage1 incorrectly treated as destination1", got, e)
	}
	for _, stage := range []uint32{2, 6, ^uint32(0)} {
		if _, e = p.ResolveStageDestination(stage, targets); e == nil {
			t.Fatal("unset/out-of-range stage authorized", stage)
		}
	}
}
