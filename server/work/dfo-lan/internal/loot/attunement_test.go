package loot

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const attunementConfig = "../../configs/attunement-rewards.generated.json"

// loadAttunementDoc reads the shipped document, lets the caller break it, and
// writes the result to a temp file. Mutating the real document is how the
// invariant checks get tested at all: a hand-written fixture would only prove
// the fixture.
func loadAttunementDoc(t *testing.T, mutate func(doc map[string]any)) string {
	t.Helper()
	b, err := os.ReadFile(attunementConfig)
	if err != nil {
		t.Fatalf("read shipped table: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse shipped table: %v", err)
	}
	if mutate != nil {
		mutate(doc)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "attunement-rewards.json")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func attunementTable(t *testing.T, doc map[string]any, dungeon float64) map[string]any {
	t.Helper()
	for _, v := range doc["tables"].([]any) {
		tab := v.(map[string]any)
		if tab["dungeon"].(float64) == dungeon {
			return tab
		}
	}
	t.Fatalf("table for dungeon %g not found", dungeon)
	return nil
}

func TestAttunementRewardsLoadsTheShippedTable(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !a.Enabled() {
		t.Fatal("shipped table reports itself disabled")
	}
	want := []uint32{100005066, 100005067, 100005068, 100005014}
	got := a.Dungeons()
	if len(got) != len(want) {
		t.Fatalf("dungeons = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dungeons = %v, want %v", got, want)
		}
	}
	// The three difficulty files together name 94 distinct boxes (72 of them
	// epic); the guard is that the set is non-trivial and every id is a box,
	// not that the source never grows.
	if n := len(a.Templates()); n < 90 {
		t.Fatalf("templates = %d, want the full reward set", n)
	}
	if n := len(a.byDungeon[100005068].Fixed) + len(a.byDungeon[100005068].Additional); n < 17 {
		t.Fatalf("epic table carries only %d lists", n)
	}
	if a.byDungeon[100005068] == nil {
		t.Fatal("epic dungeon 100005068 is not bound")
	}
	if len(a.byDungeon[100005068].Hidden) == 0 {
		t.Fatal("epic table should carry hidden tables (mazes 1 and 2)")
	}
	if len(a.byDungeon[100005067].Hidden) != 0 {
		t.Fatal("legendary table carries no hidden table in the source")
	}
	// The endkeeper-of-order table (small abyss, 100005014) comes out of the
	// same generator run with -extra: one fixed list per maze and two additional
	// branches are the payable half, and five [coupon drop table] rows are
	// carried without a roll (see the coupon pin below).
	endkeeper := a.byDungeon[100005014]
	if endkeeper == nil {
		t.Fatal("endkeeper dungeon 100005014 is not bound")
	}
	if len(endkeeper.Fixed) != 2 || len(endkeeper.Additional) != 2 {
		t.Fatalf("endkeeper lists = %d fixed / %d additional, want 2/2",
			len(endkeeper.Fixed), len(endkeeper.Additional))
	}
	if len(endkeeper.Hidden) != 0 {
		t.Fatal("endkeeper table carries no hidden table in the source")
	}
}

func TestAttunementRewardsRefusesBrokenInvariants(t *testing.T) {
	dropFirstEntry := func(tab map[string]any) []any {
		return tab["additional"].([]any)[0].(map[string]any)["entries"].([]any)
	}
	cases := []struct {
		name   string
		mutate func(doc map[string]any)
	}{
		{"wrong model", func(doc map[string]any) { doc["model"] = "something-else" }},
		{"drop list weights off the million space", func(doc map[string]any) {
			tab := attunementTable(t, doc, 100005068)
			e := dropFirstEntry(tab)[0].(map[string]any)
			e["weight"] = e["weight"].(float64) - 1
		}},
		{"select prob off the million space", func(doc map[string]any) {
			tab := attunementTable(t, doc, 100005068)
			a := tab["additional"].([]any)[0].(map[string]any)
			a["selectProb"] = a["selectProb"].(float64) - 1
		}},
		{"zero weight", func(doc map[string]any) {
			tab := attunementTable(t, doc, 100005068)
			dropFirstEntry(tab)[0].(map[string]any)["weight"] = float64(0)
		}},
		{"empty item", func(doc map[string]any) {
			tab := attunementTable(t, doc, 100005068)
			dropFirstEntry(tab)[0].(map[string]any)["item"] = float64(0)
		}},
		{"no dungeon", func(doc map[string]any) {
			attunementTable(t, doc, 100005068)["dungeon"] = float64(0)
		}},
		{"duplicate dungeon", func(doc map[string]any) {
			attunementTable(t, doc, 100005067)["dungeon"] = float64(100005068)
		}},
		{"branch that draws nothing", func(doc map[string]any) {
			tab := attunementTable(t, doc, 100005068)
			tab["additional"].([]any)[0].(map[string]any)["dropCount"] = float64(0)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := LoadAttunementRewards(loadAttunementDoc(t, c.mutate)); err == nil {
				t.Fatal("broken table was accepted")
			}
		})
	}
}

// TestAttunementRollIsInertWithoutATable pins the property that makes the
// feature safe to ship switched on: a dungeon the table does not name must not
// consume the run's seed at all, so it cannot shift any other drop.
func TestAttunementRollIsInertWithoutATable(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	for _, dungeon := range []uint32{0, 100005066 - 1, 999999999} {
		awards, seed, err := a.Roll(0x12345678, dungeon, 0)
		if err != nil {
			t.Fatalf("dungeon %d: %v", dungeon, err)
		}
		if len(awards) != 0 {
			t.Fatalf("dungeon %d paid %v", dungeon, awards)
		}
		if seed != 0x12345678 {
			t.Fatalf("dungeon %d consumed the seed: %d", dungeon, seed)
		}
	}
}

func TestAttunementRollPaysTheFixedBoxAndTheAdditionalBranch(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	known := map[uint32]bool{}
	for _, id := range a.Templates() {
		known[id] = true
	}
	// maze 0 of the epic table: one fixed pick plus one branch of 3, 5, 10 or 1.
	// Both [drop list] sums are pinned to the million space, so a roll can never
	// come back empty - that is the difference between "pays nothing" and "this
	// feature is off", and it is the whole reason the reading is trusted.
	allowed := map[int]bool{1 + 3: true, 1 + 5: true, 1 + 10: true, 1 + 1: true}
	distinct := map[uint32]int{}
	for seed := uint32(1); seed <= 4000; seed++ {
		awards, next, err := a.Roll(seed, 100005068, 0)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if !allowed[len(awards)] {
			t.Fatalf("seed %d paid %d awards, want one of the branch sizes", seed, len(awards))
		}
		if next == seed {
			t.Fatalf("seed %d was not advanced", seed)
		}
		for _, aw := range awards {
			if !known[aw.Template] {
				t.Fatalf("seed %d paid unknown template %d", seed, aw.Template)
			}
			if aw.Amount != 1 {
				t.Fatalf("seed %d paid amount %d", seed, aw.Amount)
			}
			distinct[aw.Template]++
		}
	}
	// The fixed table is 91.43% epic / 8.57% primeval, so a real sample must
	// show both, and the rare one must stay rare.
	epic, primeval := distinct[10419728], distinct[10419729]
	if epic == 0 || primeval == 0 {
		t.Fatalf("fixed table paid only one face: %d/%d", epic, primeval)
	}
	ratio := float64(primeval) / float64(epic+primeval)
	if ratio < 0.075 || ratio > 0.096 {
		t.Fatalf("primeval share %.4f, want about 0.0857", ratio)
	}
}

// TestAttunementRollStaysInsideTheMazeItWasAskedFor pins the maze key: the epic
// table declares the same list for mazes 0,1,2, but the lookup must still be by
// maze rather than "first table wins", or a table that differentiates would be
// silently flattened.
func TestAttunementRollStaysInsideTheMazeItWasAskedFor(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	for _, maze := range []uint32{0, 1, 2} {
		if _, _, err := a.Roll(7, 100005068, maze); err != nil {
			t.Fatalf("maze %d: %v", maze, err)
		}
	}
	// The legendary table covers maze 0 only: a run parked on another maze gets
	// no fixed award, and that is data absence rather than an error.
	if awards, _, err := a.Roll(7, 100005067, 3); err != nil {
		t.Fatal(err)
	} else {
		for _, aw := range awards {
			if aw.Template == 10419725 || aw.Template == 10419726 || aw.Template == 10419727 {
				t.Fatalf("uncovered maze paid the fixed face %d", aw.Template)
			}
		}
	}
}

func TestAttunementValidateTemplates(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	templates := a.Templates()
	good := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{}}
	for _, id := range templates {
		good.Items[id] = catalog.LootItem{ID: id, Kind: "stackable", StackableType: "[booster]"}
	}
	if err := a.ValidateTemplates(good); err != nil {
		t.Fatalf("complete catalog refused: %v", err)
	}

	missing := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{}}
	for k, v := range good.Items {
		missing.Items[k] = v
	}
	delete(missing.Items, templates[0])
	if err := a.ValidateTemplates(missing); err == nil {
		t.Fatal("an unknown reward template was accepted")
	}

	wrongKind := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{}}
	for k, v := range good.Items {
		wrongKind.Items[k] = v
	}
	item := wrongKind.Items[templates[0]]
	item.Kind = "equipment"
	wrongKind.Items[templates[0]] = item
	if err := a.ValidateTemplates(wrongKind); err == nil {
		t.Fatal("a non-stackable reward template was accepted")
	}
}

// TestAttunementRolledTemplatesExcludeTheHiddenTables separates "the table names
// it" from "a clear can pay it". The hidden lists are parsed and kept, but
// nothing rolls them, and 48 of the wrappers they hold resolve to a reserved id
// with no script - so counting them as payable would make the reward set look
// twice its real size and hide an unresolved slot behind it.
func TestAttunementRolledTemplatesExcludeTheHiddenTables(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	all := map[uint32]bool{}
	for _, id := range a.Templates() {
		all[id] = true
	}
	rolled := a.RolledTemplates()
	if len(rolled) == 0 {
		t.Fatal("no rolled template at all")
	}
	if len(rolled) >= len(a.Templates()) {
		t.Fatalf("rolled %d of %d: the hidden tables are being counted as payable", len(rolled), len(a.Templates()))
	}
	rolledSet := map[uint32]bool{}
	for _, id := range rolled {
		if !all[id] {
			t.Fatalf("rolled template %d is not part of the table", id)
		}
		if rolledSet[id] {
			t.Fatalf("rolled template %d listed twice", id)
		}
		rolledSet[id] = true
	}
	// The epic hidden tables (mazes 1 and 2) name these; none may be payable.
	for _, id := range []uint32{10410314, 10410326, 10410338, 10410350} {
		if !all[id] {
			t.Fatalf("hidden-only wrapper %d missing from Templates()", id)
		}
		if rolledSet[id] {
			t.Fatalf("hidden-only wrapper %d counted as payable", id)
		}
	}
}

// TestAttunementCouponRowsAreTheOmenStages pins the read of the
// endkeeper-of-order [coupon drop table] rows: they are the omen stages.
//
// A row carries two probabilities and a drop list, and their sums miss the
// million space every other list in the file sums to (obtain 400000, drop
// 2330000 on the only table we have). That used to be read as "no invariant
// separates the readings"; it is in fact the strongest evidence *for* the
// reading now implemented in omen.go, where the two columns are the first two
// branches of a three-way choice and "no change" takes the remainder - a
// three-way choice is exactly what does not need them to sum to the space.
//
// The rows are paid by AdvanceOmen, per accumulated stage, never by Roll: one
// reward set belongs to a single clear, the other to a player's run of clears.
func TestAttunementCouponRowsAreTheOmenStages(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	tab := a.byDungeon[100005014]
	if tab == nil {
		t.Fatal("endkeeper dungeon 100005014 is not bound")
	}
	if len(tab.Coupons) != 5 {
		t.Fatalf("coupon rows = %d, want 5", len(tab.Coupons))
	}
	var obtain, drop uint64
	for i, c := range tab.Coupons {
		obtain += uint64(c.ObtainProb)
		drop += uint64(c.DropProb)
		if c.ObtainProb > attunementWeightSpace || c.DropProb > attunementWeightSpace {
			t.Fatalf("coupon %d prob outside the space: %d/%d", i, c.ObtainProb, c.DropProb)
		}
	}
	// These two sums are the load-bearing observation behind the reading: every
	// other list in the file lands on the space and these do not. If that ever
	// changes, the three-way model has to be re-argued.
	if obtain == attunementWeightSpace || drop == attunementWeightSpace {
		t.Fatal("coupon probabilities now sum to the million space; the three-way reading in omen.go needs re-arguing")
	}
	all := map[uint32]bool{}
	for _, id := range a.Templates() {
		all[id] = true
	}
	rolled := map[uint32]bool{}
	for _, id := range a.RolledTemplates() {
		rolled[id] = true
	}
	payable := map[uint32]bool{}
	for _, id := range a.payableTemplates() {
		payable[id] = true
	}
	// Named by a coupon row and by nothing else: paid per accumulated omen
	// stage, not by one clear's fixed/additional roll.
	for _, id := range []uint32{10416150, 10417543, 10417544, 10417545, 10417546, 10417547, 10417552, 10417554, 10417571} {
		if !all[id] {
			t.Fatalf("coupon wrapper %d missing from Templates()", id)
		}
		if rolled[id] {
			t.Fatalf("coupon wrapper %d counted in RolledTemplates(): AdvanceOmen pays it, not a single clear", id)
		}
		if !payable[id] {
			t.Fatalf("coupon wrapper %d is not part of what a clear can pay", id)
		}
	}
	// The fixed and additional lists of this dungeon are payable, primeval
	// included - that is the whole point of wiring the exclusive table.
	for _, id := range []uint32{10416103, 10416110, 10416752, 10416141, 10416144} {
		if !rolled[id] {
			t.Fatalf("endkeeper wrapper %d is not payable", id)
		}
	}
}

// TestAttunementValidateBoxes pins both halves of the startup gate: an
// unresolvable wrapper is refused (it would drop as a jar nobody can open), and
// a slot the item catalog does not know is reported rather than invented.
func TestAttunementValidateBoxes(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	// ValidateBoxes walks payableTemplates (fixed + additional + omen stages),
	// so the complete source has to cover the omen wrappers too.
	rolled := a.payableTemplates()

	good := fakeBoxes{boxes: map[uint32]RewardBox{}, items: map[uint32]bool{11: true}}
	for _, id := range rolled {
		good.boxes[id] = RewardBox{Pools: []RewardBoxPool{{
			Draws:      1,
			Candidates: []RewardBoxCandidate{{Template: 11, Weight: 1, Count: 1}},
		}}}
	}
	empties, unopenable, err := a.ValidateBoxes(good)
	if err != nil {
		t.Fatalf("complete source refused: %v", err)
	}
	if len(empties) != 0 || len(unopenable) != 0 {
		t.Fatalf("empties = %v, unopenable = %v, want none", empties, unopenable)
	}

	missing := fakeBoxes{boxes: map[uint32]RewardBox{}, items: map[uint32]bool{11: true}}
	for k, v := range good.boxes {
		missing.boxes[k] = v
	}
	delete(missing.boxes, rolled[0])
	if _, _, err := a.ValidateBoxes(missing); err == nil {
		t.Fatal("a wrapper the box catalog cannot open was accepted")
	}

	// Every pooled entry names an id no catalog knows: that is the source's
	// empty face, and it must come back listed instead of being paid.
	sparse := fakeBoxes{boxes: map[uint32]RewardBox{}, items: map[uint32]bool{}}
	for _, id := range rolled {
		sparse.boxes[id] = RewardBox{Pools: []RewardBoxPool{{
			Draws:      1,
			Candidates: []RewardBoxCandidate{{Template: 12, Weight: 1, Count: 1}},
		}}}
	}
	empties, unopenable, err = a.ValidateBoxes(sparse)
	if err != nil {
		t.Fatal(err)
	}
	if len(empties) != 1 || empties[0] != 12 {
		t.Fatalf("empties = %v, want [12]", empties)
	}
	if len(unopenable) != 0 {
		t.Fatalf("unopenable = %v, want none", unopenable)
	}

	// A box the catalog cannot open is reported, not treated as a prize: the
	// walk has to record it whether it sits at the top or further down.
	boxed := fakeBoxes{boxes: map[uint32]RewardBox{}, items: map[uint32]bool{}, containers: map[uint32]bool{903: true}}
	for _, id := range rolled {
		boxed.boxes[id] = RewardBox{Pools: []RewardBoxPool{{
			Draws:      1,
			Candidates: []RewardBoxCandidate{{Template: 903, Weight: 1, Count: 1}},
		}}}
	}
	empties, unopenable, err = a.ValidateBoxes(boxed)
	if err != nil {
		t.Fatal(err)
	}
	if len(empties) != 0 || len(unopenable) != 1 || unopenable[0] != 903 {
		t.Fatalf("empties = %v, unopenable = %v, want [903] and no empty face", empties, unopenable)
	}

	if _, _, err := a.ValidateBoxes(nil); err != nil {
		t.Fatalf("a nil source must be inert, got %v", err)
	}
}
