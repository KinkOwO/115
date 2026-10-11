package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func asrahanIntegrityFixture() (ScriptRecord, ScriptRecord) {
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	num := func(n int32) pvf.Token { return pvf.Token{Type: 0, Value: n} }
	text := func(s string) pvf.Token { return pvf.Token{Type: 6, Text: s} }
	phase := []pvf.Token{tag("[PHASE COIN LIMIT]"), num(5), tag("[MINIMAL DUNGEON CLEAR COUNT]"), num(1),
		tag("[DUNGEON INFO]"), tag("[INDEX]"), num(9), tag("[INIT STATE]"), text("open"), tag("[PARTY MAX]"), num(1), tag("[/DUNGEON INFO]"),
		tag("[TRIGGER]"), tag("[IF]"), tag("[INIT]"), num(0), tag("[/TRIGGER]"),
		tag("[BEHAVIOR]"), tag("[SET]"), tag("[INIT]"), num(1), tag("[/BEHAVIOR]")}
	s := ScriptRecord{Path: "fixture.etc", Cells: []pvf.Token{tag("[HARD RAID PARTY MAX]"), num(1), tag("[HARD LIMIT FAME]"), num(1), tag("[PHASE TIME OVER]"), num(100), tag("[HARD PHASE TIME OVER]"), num(90), tag("[PHASE]")}}
	s.Cells = append(s.Cells, phase...)
	s.Cells = append(s.Cells, tag("[/PHASE]"), tag("[RAID PHASE OF HARDMODE]"))
	s.Cells = append(s.Cells, phase...)
	s.Cells = append(s.Cells, tag("[/RAID PHASE OF HARDMODE]"))
	sym := ScriptRecord{Path: "fixture.symbol", Cells: []pvf.Token{num(1), tag("[INIT]"), num(2), tag("[STATE]"), tag("[SYMBOL TABLE]"), tag("[SYMBOL GROUP]"), num(1), num(1), tag("[INDEX]"), num(9), tag("[/SYMBOL GROUP]"), tag("[/SYMBOL TABLE]")}}
	return s, sym
}

func TestAsrahanSourceRejectsAliasedSymbolIdentity(t *testing.T) {
	for _, id := range []int32{0, 1} {
		s, sym := asrahanIntegrityFixture()
		sym.Cells[2].Value = id
		if _, err := ParseAsrahanRaidRules(s, sym); err == nil {
			t.Fatalf("accepted ambiguous symbol id %d", id)
		}
	}
}

func TestAsrahanSourceRejectsUnpairedEventBlocks(t *testing.T) {
	for _, bad := range [][]pvf.Token{
		{{Type: 3, Text: "[BEHAVIOR]"}, {Type: 3, Text: "[SET]"}, {Type: 3, Text: "[INIT]"}, {Type: 0, Value: 0}, {Type: 3, Text: "[/BEHAVIOR]"}},
		{{Type: 3, Text: "[/TRIGGER]"}},
		{{Type: 3, Text: "[/BEHAVIOR]"}},
	} {
		s, sym := asrahanIntegrityFixture()
		for i, token := range s.Cells {
			if token.Text == "[/PHASE]" {
				s.Cells = append(append(append([]pvf.Token{}, s.Cells[:i]...), bad...), s.Cells[i:]...)
				break
			}
		}
		if _, err := ParseAsrahanRaidRules(s, sym); err == nil {
			t.Fatalf("silently discarded malformed source event %+v", bad)
		}
	}
}

func TestAsrahanSourcePreservesUnknownTypedExpression(t *testing.T) {
	s, sym := asrahanIntegrityFixture()
	for i, tok := range s.Cells {
		if tok.Text == "[/BEHAVIOR]" {
			s.Cells = append(append(append([]pvf.Token{}, s.Cells[:i]...), pvf.Token{Type: 10, Value: 77, Text: "source-expression"}), s.Cells[i:]...)
			break
		}
	}
	r, err := ParseAsrahanRaidRules(s, sym)
	if err != nil {
		t.Fatal(err)
	}
	b := r.Normal.Events[0].Behavior
	if b[len(b)-1].Type != 10 || b[len(b)-1].Text != "source-expression" || len(r.Hard.Events[0].Behavior) != 3 {
		t.Fatal("unknown expression lost or changed another mode")
	}
}
