package boostup

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBoostGuideProfessionSourceOverrides(t *testing.T) {
	c := &Catalog{Steps: []Step{{Number: 1, Guide: "dungeon", Dungeon: 10, GuideCells: []pvf.Token{
		{Type: 3, Text: "[specific dungeon index]"}, {Type: 6, Text: "[priest]"}, {Type: 0, Value: 1}, {Type: 0, Value: 1}, {Type: 0, Value: 20}, {Type: 3, Text: "[/specific dungeon index]"},
	}}}}
	for _, v := range []struct {
		variant uint32
		want    uint32
	}{{0, 10}, {1, 20}} {
		id, e := c.GuideDungeon(Training{Step: 1}, "[priest]", 1, v.variant)
		if e != nil || id != v.want {
			t.Fatal(id, e)
		}
	}
	if _, e := c.GuideDungeon(Training{Step: 1, Phase: 2}, "[priest]", 1, 1); e == nil {
		t.Fatal("claimed guide entered again")
	}
}
func TestBoostActualTrainingDungeonSelections(t *testing.T) {
	root := os.Getenv("US115_TEST_BOOST_SOURCE")
	path := os.Getenv("US115_TEST_BOOST_DUNGEONS")
	if root == "" || path == "" {
		t.Skip("explicit source tokens/full dungeon export required")
	}
	read := func(name string) []pvf.Token {
		b, e := os.ReadFile(filepath.Join(root, name))
		if e != nil {
			t.Fatal(e)
		}
		var out []pvf.Token
		if e = json.Unmarshal(b, &out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	c, e := Parse(read("01-boostup.evt.tokens.json"), read("00-eventgift.evt.tokens.json"))
	if e != nil {
		t.Fatal(e)
	}
	cat, e := catalog.LoadDungeons(path)
	if e != nil {
		t.Fatal(e)
	}
	seen := map[uint32]bool{}
	for _, step := range c.Steps {
		if step.Guide != "dungeon" {
			continue
		}
		seen[step.Dungeon] = true
		v := values(step.GuideCells, "[specific dungeon index]")
		for i := 0; i < len(v); i += 4 {
			id, e := c.GuideDungeon(Training{Step: step.Number}, v[i].Text, byte(v[i+1].Value), uint32(v[i+2].Value))
			if e != nil {
				t.Fatal(e)
			}
			seen[id] = true
		}
	}
	for id := range seen {
		d, ok := cat.Dungeons[id]
		if !ok {
			t.Errorf("source guide%d absent", id)
			continue
		}
		run, e := dungeon.SelectTutorial(cat, id)
		if e != nil {
			t.Errorf("guide%d tutorial=%t: %v", id, d.Tutorial, e)
			continue
		}
		if len(run.Maze.Rooms) == 0 {
			t.Fatal("empty source guide")
		}
		t.Logf("guide%d rooms=%d noFatigue=%t", id, len(run.Maze.Rooms), d.NoFatigue)
	}
	t.Logf("unique profession-specific guide routes=%d", len(seen))
}
