package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestAsrahanCombatIdentityFollowsSourceAndOwnsFields(t *testing.T) {
	symbol := pvf.Token{Type: 3, Text: "[VALUE]"}
	absent := pvf.Token{Type: 0, Value: -1}
	fields := []pvf.Token{symbol, absent, symbol, symbol, symbol, symbol, symbol, {Type: 0, Value: 41}, absent, absent, absent, absent, absent, symbol, absent, absent, absent}
	r := &AsrahanRaidRules{Symbols: map[string]uint32{"[VALUE]": 2}, Groups: []AsrahanSymbolGroup{{Header: []pvf.Token{{Type: 0, Value: 1}, {Type: 0, Value: 31}, absent}, Fields: fields}}}
	p := AsrahanRaidPhase{Dungeons: map[uint32]AsrahanDungeon{31: {}}}
	g, err := CompileAsrahanCombatGroups(r, p)
	if err != nil || g[0].Monster != 41 || g[0].Dungeon != 31 {
		t.Fatal(g, err)
	}
	fields[7].Value = 42
	if g[0].Fields[7].Value != 41 {
		t.Fatal("combat group retains mutable source cells")
	}
	g, err = CompileAsrahanCombatGroups(r, p)
	if err != nil || g[0].Monster != 42 {
		t.Fatal("monster identity did not follow changed source", g, err)
	}
	delete(p.Dungeons, 31)
	if _, err := CompileAsrahanCombatGroups(r, p); err == nil {
		t.Fatal("foreign dungeon acquired combat ownership")
	}
	p.Dungeons[31] = AsrahanDungeon{}
	fields[0] = pvf.Token{Type: 3, Text: "[FOREIGN]"}
	if _, err := CompileAsrahanCombatGroups(r, p); err == nil {
		t.Fatal("undeclared HP symbol acquired combat ownership")
	}
}
