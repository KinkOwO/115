package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"dfolan/internal/catalog/pvf"
)

func loadDropGroupFixture(t *testing.T) []DropGroup {
	t.Helper()
	b, e := os.ReadFile("testdata/droptablebygroup_sample.json")
	if e != nil {
		t.Fatal(e)
	}
	var doc struct {
		Source       string      `json:"source"`
		SourceSHA256 string      `json:"source_sha256"`
		Cells        []pvf.Token `json:"cells"`
	}
	if e = json.Unmarshal(b, &doc); e != nil {
		t.Fatal(e)
	}
	if len(doc.SourceSHA256) != 64 {
		t.Fatalf("fixture lost its provenance: %q", doc.SourceSHA256)
	}
	groups, e := ParseDropGroups(doc.Cells)
	if e != nil {
		t.Fatal(e)
	}
	return groups
}

// The fixture is a slice of the real cell stream of
// etc/dungeondroptablebygroup.etc, so these expectations are the client's own
// numbers rather than values chosen to fit the parser. The table is not a stored
// artifact yet, which is why a slice is committed instead of the whole file.
func TestParseDropGroupsSample(t *testing.T) {
	groups := loadDropGroupFixture(t)
	if len(groups) != 10 {
		t.Fatalf("groups: got %d want 10", len(groups))
	}
	byID := map[uint32]DropGroup{}
	for _, g := range groups {
		byID[g.ID] = g
	}
	for _, tc := range []struct {
		id            uint32
		creation      []int32
		armor         int32
		hasArmor      bool
		target        string
		explicit      []DropWeight
		explicitCount int
		smart         []DropWeight
		smartCount    int
	}{
		{id: 1, creation: []int32{12000, 0}, smartCount: 73,
			smart: []DropWeight{{109040488, 300}, {109000713, 300}}},
		{id: 2, creation: []int32{12000, 0}, smartCount: 73, armor: 432, hasArmor: true,
			smart: []DropWeight{{104030802, 300}, {114030285, 300}}},
		{id: 21251, creation: []int32{0, 0}, smartCount: 11,
			smart: []DropWeight{{100051304, 10}, {100101187, 10}}},
		{id: 21279, explicitCount: 12,
			explicit: []DropWeight{{10401449, 1}, {10401450, 1}}},
		{id: 21289, creation: []int32{1200, 0}, target: "weapon legacy", smartCount: 216,
			smart: []DropWeight{{101001151, 30}, {101011317, 30}}},
		{id: 21310, explicitCount: 1, explicit: []DropWeight{{10336310, 1}}},
		{id: 21600, explicitCount: 1, explicit: []DropWeight{{10420063, 1}}},
		{id: 21601, explicitCount: 1, explicit: []DropWeight{{10420064, 1}}},
		{id: 21602, explicitCount: 1, explicit: []DropWeight{{10420065, 1}}},
		{id: 21603, explicitCount: 1, explicit: []DropWeight{{10404804, 1}}},
	} {
		g, ok := byID[tc.id]
		if !ok {
			t.Fatalf("group %d missing", tc.id)
		}
		if len(g.CreationRate) != len(tc.creation) {
			t.Fatalf("group %d creation rate: got %v want %v", tc.id, g.CreationRate, tc.creation)
		}
		for i, v := range tc.creation {
			if g.CreationRate[i] != v {
				t.Fatalf("group %d creation rate: got %v want %v", tc.id, g.CreationRate, tc.creation)
			}
		}
		if tc.hasArmor {
			if g.ArmorRate == nil || *g.ArmorRate != tc.armor {
				t.Fatalf("group %d armor rate: got %v want %d", tc.id, g.ArmorRate, tc.armor)
			}
		} else if g.ArmorRate != nil {
			t.Fatalf("group %d unexpected armor rate %d", tc.id, *g.ArmorRate)
		}
		if g.Target != tc.target {
			t.Fatalf("group %d target: got %q want %q", tc.id, g.Target, tc.target)
		}
		if len(g.Explicit) != tc.explicitCount {
			t.Fatalf("group %d explicit rows: got %d want %d", tc.id, len(g.Explicit), tc.explicitCount)
		}
		for i, w := range tc.explicit {
			if g.Explicit[i] != w {
				t.Fatalf("group %d explicit[%d]: got %+v want %+v", tc.id, i, g.Explicit[i], w)
			}
		}
		if len(g.Smart) != tc.smartCount {
			t.Fatalf("group %d smart rows: got %d want %d", tc.id, len(g.Smart), tc.smartCount)
		}
		for i, w := range tc.smart {
			if g.Smart[i] != w {
				t.Fatalf("group %d smart[%d]: got %+v want %+v", tc.id, i, g.Smart[i], w)
			}
		}
	}
}

// A half-read table would award the wrong items, so every structural deviation
// the source can express has to be refused rather than tolerated.
func TestParseDropGroupsRejectsMalformed(t *testing.T) {
	h := func(text string) pvf.Token { return pvf.Token{Type: 3, Text: text} }
	n := func(v int32) pvf.Token { return pvf.Token{Type: 0, Value: v} }
	good := func() []pvf.Token {
		return []pvf.Token{
			h("[table]"), h("[group]"), n(7), h("[creation rate]"), n(12000), n(0),
			h("[smart drop item]"), n(1001), n(300), h("[/smart drop item]"), h("[/table]"),
		}
	}
	if _, e := ParseDropGroups(good()); e != nil {
		t.Fatalf("well-formed table refused: %v", e)
	}
	for _, tc := range []struct {
		name   string
		mutate func([]pvf.Token) []pvf.Token
	}{
		{"group outside table", func(c []pvf.Token) []pvf.Token { return c[1:] }},
		{"nested table", func(c []pvf.Token) []pvf.Token {
			return append(c[:1], append([]pvf.Token{h("[table]")}, c[1:]...)...)
		}},
		{"second group in one table", func(c []pvf.Token) []pvf.Token {
			out := append([]pvf.Token{}, c[:3]...)
			out = append(out, h("[group]"), n(8))
			return append(out, c[3:]...)
		}},
		{"no item section", func(c []pvf.Token) []pvf.Token {
			return []pvf.Token{h("[table]"), h("[group]"), n(7), h("[/table]")}
		}},
		{"odd creation rate", func(c []pvf.Token) []pvf.Token {
			out := append([]pvf.Token{}, c[:5]...)
			return append(out, c[6:]...)
		}},
		{"dangling item template", func(c []pvf.Token) []pvf.Token {
			out := append([]pvf.Token{}, c[:8]...)
			return append(out, c[9:]...)
		}},
		{"unknown section", func(c []pvf.Token) []pvf.Token {
			out := append([]pvf.Token{}, c[:7]...)
			out = append(out, h("[nonsense]"))
			return append(out, c[7:]...)
		}},
		{"string in an item section", func(c []pvf.Token) []pvf.Token {
			out := append([]pvf.Token{}, c...)
			out[7] = pvf.Token{Type: 6, Text: "weapon legacy"}
			return out
		}},
		{"zero group id", func(c []pvf.Token) []pvf.Token {
			out := append([]pvf.Token{}, c...)
			out[2] = n(0)
			return out
		}},
		{"duplicate group id", func(c []pvf.Token) []pvf.Token {
			return append(c, h("[table]"), h("[group]"), n(7),
				h("[drop item]"), n(1001), n(1), h("[/drop item]"), h("[/table]"))
		}},
		{"mismatched item section close", func(c []pvf.Token) []pvf.Token {
			out := append([]pvf.Token{}, c...)
			out[9] = h("[/drop item]")
			return out
		}},
		{"unterminated table", func(c []pvf.Token) []pvf.Token { return c[:len(c)-1] }},
	} {
		if _, e := ParseDropGroups(tc.mutate(good())); e == nil {
			t.Fatalf("%s accepted", tc.name)
		}
	}
}

// The per-dungeon side is checked against the shipped catalog, so the numbers
// below are the ones the current client actually carries. Note that the string
// cell of [special setinfo reward] is only visible at cell level - the textual
// PVF dump drops it, which is why this reads the catalog instead.
func TestParseDungeonDropBlocksBorderOfAttunement(t *testing.T) {
	d := loadDungeon(t, 100005068)
	blocks, e := ParseDungeonDropBlocks(d.Script.Cells)
	if e != nil {
		t.Fatal(e)
	}
	if len(blocks) != 3 {
		t.Fatalf("blocks: got %d want 3", len(blocks))
	}
	for i, tc := range []struct {
		item     int32
		groups   []int32
		multiple []int32
	}{
		{item: 10420247, groups: []int32{1, 21251, 1, 21600}, multiple: []int32{3, 1}},
		{item: 10420248, groups: []int32{1, 21251, 1, 21601}, multiple: []int32{3, 6}},
		{item: 10420249, groups: []int32{1, 21251, 1, 21602}, multiple: []int32{3, 12}},
	} {
		b := blocks[i]
		if b.Kind != "[group info]" {
			t.Fatalf("block %d kind: got %q", i, b.Kind)
		}
		var headers []string
		for _, s := range b.Sections {
			headers = append(headers, s.Header)
		}
		want := []string{
			"[item index]", "[normal group index]", "[fame info]",
			"[special setinfo reward]", "[special setinfo reward]",
			"[special setinfo reward]", "[special setinfo reward]",
			"[reward multiple info]",
		}
		if len(headers) != len(want) {
			t.Fatalf("block %d sections: got %v want %v", i, headers, want)
		}
		for j := range want {
			if headers[j] != want[j] {
				t.Fatalf("block %d section %d: got %q want %q", i, j, headers[j], want[j])
			}
		}
		if items := b.Sections[0].Values; len(items) != 4 || items[2] != tc.item {
			t.Fatalf("block %d item index: got %v", i, items)
		}
		if got := b.Sections[1].Values; !equalInt32(got, tc.groups) {
			t.Fatalf("block %d normal group index: got %v want %v", i, got, tc.groups)
		}
		for r, sec := range b.Sections[3:7] {
			if len(sec.Labels) != 1 || len(sec.Values) != 4 {
				t.Fatalf("block %d setinfo %d: labels=%v values=%v", i, r, sec.Labels, sec.Values)
			}
			if sec.Values[3] != []int32{21279, 21468, 21470, 21310}[r] {
				t.Fatalf("block %d setinfo %d group: got %v", i, r, sec.Values)
			}
		}
		if b.Sections[3].Labels[0] != "<2::SetEquipmentReward>" ||
			b.Sections[6].Labels[0] != "<2::WeaponEquipmentReward>" {
			t.Fatalf("block %d setinfo labels: got %q / %q", i,
				b.Sections[3].Labels[0], b.Sections[6].Labels[0])
		}
		m := b.Sections[7].Values
		if !equalInt32(m, tc.multiple) {
			t.Fatalf("block %d reward multiple: got %v want %v", i, m, tc.multiple)
		}
	}
}

// Every dungeon must parse, including the three source irregularities the sweep
// turned up: an empty [custom group info] placeholder (100004177), a first block
// whose opening [group info] tag is missing (100002889), a list repeated verbatim
// (5410) and an empty list (100004448).
func TestParseDungeonDropBlocksAcrossCatalog(t *testing.T) {
	c, e := LoadDungeons("../../configs/dungeons.full.json")
	if e != nil {
		t.Fatal(e)
	}
	blocks, custom, empty, sections := 0, 0, 0, 0
	for id, d := range c.Dungeons {
		got, e := ParseDungeonDropBlocks(d.Script.Cells)
		if e != nil {
			t.Fatalf("dungeon %d: %v", id, e)
		}
		for _, b := range got {
			blocks++
			if b.Kind == "[custom group info]" {
				custom++
			}
			if len(b.Sections) == 0 {
				empty++
			}
			sections += len(b.Sections)
		}
	}
	if blocks == 0 || sections == 0 {
		t.Fatalf("catalog sweep found nothing: blocks=%d sections=%d", blocks, sections)
	}
	t.Logf("blocks=%d custom=%d empty=%d sections=%d dungeons=%d",
		blocks, custom, empty, sections, len(c.Dungeons))
}

// [normal group index] has two plausible readings and neither explains the whole
// catalog, so no decoder is shipped. This test pins that ambiguity: if a future
// change makes one reading fit everything, it will fail and the conclusion can be
// revisited on purpose instead of by accident.
func TestNormalGroupIndexEncodingIsAmbiguous(t *testing.T) {
	c, e := LoadDungeons("../../configs/dungeons.full.json")
	if e != nil {
		t.Fatal(e)
	}
	const header = "[normal group index]"
	var both, prefixOnly, pairsOnly, neither, total int
	var firstNeither string
	for id, d := range c.Dungeons {
		got, e := ParseDungeonDropBlocks(d.Script.Cells)
		if e != nil {
			t.Fatal(e)
		}
		for _, b := range got {
			for _, s := range b.Sections {
				if s.Header != header {
					continue
				}
				total++
				p := fitsLengthPrefix(s.Values)
				q := len(s.Values) > 0 && len(s.Values)%2 == 0
				switch {
				case p && q:
					both++
				case p:
					prefixOnly++
				case q:
					pairsOnly++
				default:
					neither++
					if firstNeither == "" {
						firstNeither = fmt.Sprintf("%d %v", id, s.Values)
					}
				}
			}
		}
	}
	t.Logf("%s sections=%d both=%d prefixOnly=%d pairsOnly=%d neither=%d first neither=%s",
		header, total, both, prefixOnly, pairsOnly, neither, firstNeither)
	// Both readings need exclusive cases for the ambiguity to hold. If one day a
	// single reading fits all 3419 sections, this fails and the conclusion in
	// droptable.go can be revisited on purpose rather than by accident.
	if prefixOnly == 0 || pairsOnly == 0 {
		t.Fatalf("the two readings no longer disagree (%d/%d/%d/%d) - revisit the "+
			"conclusion in droptable.go before shipping a decoder",
			both, prefixOnly, pairsOnly, neither)
	}
	if total < 3000 {
		t.Fatalf("%s sections=%d - the sweep is no longer reading the catalog", header, total)
	}
}

func fitsLengthPrefix(v []int32) bool {
	i := 0
	for i < len(v) {
		n := int(v[i])
		if n <= 0 || i+1+n > len(v) {
			return false
		}
		i += 1 + n
	}
	return len(v) > 0
}

func equalInt32(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func loadDungeon(t *testing.T, id uint32) DungeonDefinition {
	t.Helper()
	c, e := LoadDungeons("../../configs/dungeons.full.json")
	if e != nil {
		t.Fatal(e)
	}
	d, ok := c.Dungeons[id]
	if !ok {
		t.Fatalf("dungeon %d absent from the catalog", id)
	}
	return d
}

// Catalogs written before the projection existed carry neither key and must keep
// loading; the shipped next25 catalog is exactly that case.
func TestLoadLootAcceptsCatalogWithoutDropGroups(t *testing.T) {
	c, e := LoadLoot("../../configs/loot.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	if len(c.DropGroups) != 0 {
		t.Fatalf("catalog unexpectedly carries %d drop groups", len(c.DropGroups))
	}
	if _, ok := c.DropGroupByID(21600); ok {
		t.Fatal("empty projection answered a lookup")
	}
}

func TestLoadLootRejectsIncompleteDropGroups(t *testing.T) {
	src := DropGroupSource{Path: "etc/dungeondroptablebygroup.etc",
		SHA256: "2590d9378a81de11e66e3dcc11f6d05124becac627259f6b088df275b2bcdf51"}
	base := LootCatalog{
		Source:       pvf.ArchiveSnapshot{Checksum: "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"},
		MaximumGrade: 1,
		Rules:        map[string]ScriptRecord{"a": {}, "b": {}, "c": {}, "d": {}},
	}
	write := func(t *testing.T, c LootCatalog) string {
		t.Helper()
		b, e := json.Marshal(c)
		if e != nil {
			t.Fatal(e)
		}
		p := t.TempDir() + "/loot.json"
		if e = os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		return p
	}
	noSource := base
	noSource.DropGroups = []DropGroup{{ID: 1, Explicit: []DropWeight{{1, 1}}}}
	if _, e := LoadLoot(write(t, noSource)); e == nil {
		t.Fatal("drop groups without provenance accepted")
	}
	noGroups := base
	noGroups.DropGroupSource = src
	if _, e := LoadLoot(write(t, noGroups)); e == nil {
		t.Fatal("provenance without groups accepted")
	}
	badItem := base
	badItem.DropGroupSource = src
	badItem.DropGroups = []DropGroup{{ID: 1, Explicit: []DropWeight{{0, 1}}}}
	if _, e := LoadLoot(write(t, badItem)); e == nil {
		t.Fatal("zero item template accepted")
	}
	dup := base
	dup.DropGroupSource = src
	dup.DropGroups = []DropGroup{
		{ID: 1, Explicit: []DropWeight{{1, 1}}},
		{ID: 1, Explicit: []DropWeight{{2, 1}}},
	}
	if _, e := LoadLoot(write(t, dup)); e == nil {
		t.Fatal("duplicate drop group accepted")
	}
}
