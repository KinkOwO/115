package boostup

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBoostUpActualSelectedSource(t *testing.T) {
	dir := os.Getenv("US115_TEST_BOOST_SOURCE")
	if dir == "" {
		t.Skip("explicit read-only exported source tokens required")
	}
	read := func(name string) []pvf.Token {
		b, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		var c []pvf.Token
		if e = json.Unmarshal(b, &c); e != nil {
			t.Fatal(e)
		}
		return c
	}
	cells, gifts := read("01-boostup.evt.tokens.json"), read("00-eventgift.evt.tokens.json")
	c, e := Parse(cells, gifts)
	if e != nil {
		t.Fatal(e)
	}
	if c.GoalLevel != 115 || c.UsableLevel != 115 || c.Town != 222 || len(c.Steps) != 11 || len(c.Gifts) != 2 || len(c.Presets) < 69 {
		t.Fatal("actual source identity", c.GoalLevel, c.Town, len(c.Steps), len(c.Gifts), len(c.Presets))
	}
	if c.ReservedMail == nil || c.ReservedMail.Item != 590015954 || c.ReservedMail.Count != 1 {
		t.Fatal("source reserved mail lost", c.ReservedMail)
	}
	// [capsule info] 里的 [quest clear item]：直升后补发的三张清券（各清一条
	// [grade] [side] 墙任务，见 docs/protocol/boostup662-story-skip-20261006.md）。
	if len(c.QuestClearItems) != 3 || c.QuestClearItems[0] != 10327301 || c.QuestClearItems[1] != 10327302 || c.QuestClearItems[2] != 10327303 {
		t.Fatal("source quest clear items lost", c.QuestClearItems)
	}
	if len(c.JournalDiscounts) != 1 || c.JournalDiscounts[0] != (JournalDiscount{Step: 10, Phase: 2, Condition: 0, Gold: 100, Material: 0}) {
		t.Fatal("source journal discount lost", c.JournalDiscounts)
	}
	if e = c.ValidateWearMissions(); e != nil {
		t.Fatal(e)
	}
	for _, step := range c.Steps {
		if step.BlockEnchantBead != (step.Number == 9) {
			t.Fatal("source bead restriction scope", step.Number, step.BlockEnchantBead)
		}
		if step.AutoOpen != (step.Number == 9 || step.Number == 11) {
			t.Fatal("source auto-open scope", step.Number, step.AutoOpen)
		}
	}
	enchanted, err := c.Steps[5].WearRequirement()
	if err != nil || enchanted == nil || !enchanted.Enchanted || len(enchanted.Kinds) != 12 {
		t.Fatal("source enchanted equipment mission", enchanted, err)
	}
	point, err := c.Steps[0].PointRequirement()
	if err != nil || point.Minimum != 1265 || len(point.Contracts) != 4 || point.Contracts[0] != 22 || point.Contracts[1] != 27 || point.Contracts[2] != 79 || point.Contracts[3] != 92 {
		t.Fatal("source contract/point requirements", point, err)
	}
	for _, n := range []int{2, 7, 8, 10} {
		r, err := c.Steps[n-1].WearRequirement()
		if err != nil || r == nil {
			t.Fatalf("step%d: %v", n, err)
		}
		if n == 8 && (r.Count != 11 || len(r.Groups) != 7) {
			t.Fatal("all repeated group indexes must survive", r)
		}
		if n == 10 && r.Count != 1 {
			t.Fatal("journal OR wear fallback source", r)
		}
	}
	if c.Steps[5].Mission != "equip enchanted item" || c.Steps[8].Mission != "disjoint" || c.Steps[9].Mission != "transform equip journal or equip item" {
		t.Fatal("mission semantics lost")
	}
	if c.Steps[0].Rewards[0].Item != 590015877 || c.Steps[10].Rewards[0].Item != 590015965 || c.Gifts[0].Items[0] != 590015870 {
		t.Fatal("reward source lost")
	}
	if !c.Gifts[0].FirstPopup || c.Gifts[1].FirstPopup || !c.Gifts[0].Direct {
		t.Fatal("gift defaults overrode explicit source")
	}
	if len(c.Steps[10].GuideCells) == 0 || len(c.Steps[5].MissionCells) == 0 {
		t.Fatal("unmodeled source conditions discarded")
	}
	t.Logf("read-only source: steps=%d gifts=%d preset alternatives=%d", len(c.Steps), len(c.Gifts), len(c.Presets))
	for _, code := range c.Presets {
		p, e := DecodePreset(code.Code)
		if e != nil {
			t.Fatalf("preset job%d grow%d: %v", code.Job, code.Grow, e)
		}
		if p.Job != code.Job || p.Grow != code.Grow || p.Level != 115 || len(p.QuickSlots) != 14 {
			t.Fatalf("preset identity/slots mismatch: %+v", p)
		}
	}
}
func TestBoostSourcePairsAndBoundaries(t *testing.T) {
	n := func(v int32) pvf.Token { return pvf.Token{Type: 0, Value: v} }
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	if _, e := rewards([]pvf.Token{tag("[reward]"), n(1)}, "[reward]"); e == nil {
		t.Fatal("odd reward accepted")
	}
	if _, e := sections([]pvf.Token{tag("[step info]")}, "[step info]", "[/step info]"); e == nil {
		t.Fatal("truncated source accepted")
	}
	if _, e := Parse(nil, nil); e == nil {
		t.Fatal("missing content accepted")
	}
}
