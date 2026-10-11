package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"strings"
	"testing"
)

func attunementPilotParsedDefinition(t *testing.T) catalog.DungeonDefinition {
	t.Helper()
	section := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	label := func(s string) pvf.Token { return pvf.Token{Type: 6, Text: s} }
	number := func(n int32) pvf.Token { return pvf.Token{Type: 0, Value: n} }
	d, err := catalog.ParseDungeon(17, catalog.ScriptRecord{
		Path: "Contents\\2026\\SkyofaThousandSeasOfBorder\\Dungeon\\source-test.dgn",
		Cells: []pvf.Token{section("[minimum required level]"), number(115), section("[basis level]"), number(145),
			section("[dungeon type]"), label("boundary of attunement"), section("[maze info]"),
			section("[size]"), number(1), number(1), section("[map specification]"), label("boss"), number(0), number(0), number(81), section("[/map specification]"),
			section("[start map]"), number(0), number(0), section("[/start map]"), section("[boss map]"), number(0), number(0), section("[/boss map]")}})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestAttunementElitePilotSourceAndModeBoundaries(t *testing.T) {
	w, _ := adventureTestWorld(t)
	sel := protocol.DungeonSelection{ID: 17, Party: 65535}
	s := &dungeon.Session{Definition: attunementPilotParsedDefinition(t)}
	for _, value := range []string{"", "all", "1"} {
		t.Setenv("DFO_ELITE_COMPANION_PILOT", value)
		p, err := w.attunementEliteEntryPackets(context.Background(), sel, s)
		if err != nil || len(p) != 0 {
			t.Fatal("disabled profile gained companions", value, err)
		}
	}
	t.Setenv("DFO_ELITE_COMPANION_PILOT", "attunement")
	for name, change := range map[string]func(*worldSession, *dungeon.Session, *protocol.DungeonSelection){
		"other source namespace": func(_ *worldSession, s *dungeon.Session, _ *protocol.DungeonSelection) {
			s.Definition.Script.Path = "contents/2025/another-abyss/source-test.dgn"
		},
		"ordinary source": func(_ *worldSession, s *dungeon.Session, _ *protocol.DungeonSelection) {
			s.Definition.Type = "normal"
		},
		"different dungeon": func(_ *worldSession, _ *dungeon.Session, sel *protocol.DungeonSelection) { sel.ID++ },
		"foreign party":     func(_ *worldSession, _ *dungeon.Session, sel *protocol.DungeonSelection) { sel.Party = 9 },
		"native channel":    func(w *worldSession, _ *dungeon.Session, _ *protocol.DungeonSelection) { w.channelType = 73 },
		"isolated channel":  func(w *worldSession, _ *dungeon.Session, _ *protocol.DungeonSelection) { w.channelWorldIsolated = true },
	} {
		t.Run(name, func(t *testing.T) {
			cw, cs, csel := *w, *s, sel
			change(&cw, &cs, &csel)
			p, err := cw.attunementEliteEntryPackets(context.Background(), csel, &cs)
			if err != nil || len(p) != 0 {
				t.Fatal("pilot escaped its source/solo boundary", err)
			}
		})
	}
	// A source-classified solo pilot must fail closed on an empty saved roster.
	companionTestRules(t, map[uint16][]uint32{2: {41}})
	if _, err := w.attunementEliteEntryPackets(context.Background(), sel, s); err == nil || !strings.Contains(err.Error(), "至少一个") {
		t.Fatal("enabled pilot accepted empty roster", err)
	}
}
