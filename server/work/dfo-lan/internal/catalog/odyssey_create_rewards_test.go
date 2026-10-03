package catalog

import (
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func creationSourceFixture() (ItemIndex, ScriptRecord, map[string]ScriptRecord) {
	tokens := func(tag string, nums ...int32) []pvf.Token {
		out := []pvf.Token{{Type: 3, Text: tag}}
		for _, n := range nums {
			out = append(out, pvf.Token{Type: 0, Value: n})
		}
		return out
	}
	fixed := func() []pvf.Token {
		c := tokens("[booster selection num]", 0)
		return append(c, tokens("[booster select category]", 0, 0)...)
	}
	def := ScriptRecord{Cells: append(append(tokens("[create reward]"), tokens("[reward data]", 100)...), tokens("[/create reward]")...)}
	root := fixed()
	root = append(root, tokens("[stackable]", 101, 1, 102, 1, 103, 7)...)
	root = append(root, tokens("[/stackable]")...)
	root = append(root, tokens("[/booster select category]")...)
	armor := fixed()
	armor = append(armor, tokens("[equipment]", 201, 1, 202, 2)...)
	armor = append(armor, tokens("[/equipment]")...)
	armor = append(armor, tokens("[/booster select category]")...)
	weapon := tokens("[booster selection num]", 1)
	weapon = append(weapon, tokens("[booster select category]", 0, 0)...)
	weapon = append(weapon, tokens("[equipment]", 203, 1)...)
	weapon = append(weapon, tokens("[/equipment]")...)
	weapon = append(weapon, tokens("[/booster select category]")...)
	idx := ItemIndex{Source: pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}, Items: map[uint32]ItemIndexEntry{}}
	for _, id := range []uint32{100, 101, 102, 103} {
		idx.Items[id] = ItemIndexEntry{ID: id, Path: map[uint32]string{100: "root", 101: "weapon", 102: "armor", 103: "potion"}[id], Kind: "stackable", StackableType: "[booster selection]"}
	}
	e := idx.Items[103]
	e.StackableType = "[waste]"
	idx.Items[103] = e
	for _, id := range []uint32{201, 202, 203} {
		idx.Items[id] = ItemIndexEntry{ID: id, Path: "equipment", Kind: "equipment"}
	}
	return idx, def, map[string]ScriptRecord{"root": {Cells: root}, "weapon": {Cells: weapon}, "armor": {Cells: armor}, "potion": {}}
}
func TestOdysseyCreationFollowsETCAndPackageReferences(t *testing.T) {
	idx, def, scripts := creationSourceFixture()
	read := func(p string) (ScriptRecord, error) {
		s, ok := scripts[p]
		if !ok {
			t.Fatalf("unexpected source %s", p)
		}
		return s, nil
	}
	r, e := parseOdysseyCreateRewards(idx, def, read)
	if e != nil {
		t.Fatal(e)
	}
	if r.Template != 100 || r.Weapon.Template != 101 || r.ArmorBox != 102 || len(r.Armor) != 2 || r.Armor[1].Count != 2 || r.Supplies[0].Template != 103 || r.Supplies[0].Count != 7 {
		t.Fatalf("hardcoded creation rewards: %+v", r)
	}
	changed := scripts["root"]
	for i, c := range changed.Cells {
		if c.Type == 0 && c.Value == 103 {
			changed.Cells[i+1].Value = 19
		}
	}
	scripts["root"] = changed
	r, e = parseOdysseyCreateRewards(idx, def, read)
	if e != nil || r.Supplies[0].Count != 19 {
		t.Fatal("source count ignored", r, e)
	}
}
func TestOdysseyCreationRejectsUnsupportedOrMissingBindings(t *testing.T) {
	for _, kind := range []string{"missing armor", "wrong equipment kind", "invalid count", "extra supplies"} {
		t.Run(kind, func(t *testing.T) {
			idx, def, scripts := creationSourceFixture()
			switch kind {
			case "missing armor":
				delete(idx.Items, 102)
			case "wrong equipment kind":
				idx.Items[201] = ItemIndexEntry{ID: 201, Kind: "avatar"}
			case "invalid count":
				s := scripts["root"]
				for i, c := range s.Cells {
					if c.Value == 103 {
						s.Cells[i+1].Value = -1
					}
				}
				scripts["root"] = s
			case "extra supplies":
				s := scripts["root"]
				for i, c := range s.Cells {
					if c.Text == "[/stackable]" {
						s.Cells = append(s.Cells[:i], append([]pvf.Token{{Type: 0, Value: 104}, {Type: 0, Value: 1}}, s.Cells[i:]...)...)
						break
					}
				}
				scripts["root"] = s
				idx.Items[104] = ItemIndexEntry{ID: 104, Path: "potion", Kind: "stackable", StackableType: "[waste]"}
			}
			read := func(p string) (ScriptRecord, error) { return scripts[p], nil }
			if _, e := parseOdysseyCreateRewards(idx, def, read); e == nil {
				t.Fatal("unsupported source accepted")
			}
		})
	}
}
