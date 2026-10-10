package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
)

// Asrahan owns an independent script tree. A parsed source is not a
// Bakal executor, and unknown behavior operands remain typed and visible.
type AsrahanRaidRules struct {
	MonsterSource                          *AsrahanMonsterSource
	Path, SHA256, SymbolPath, SymbolSHA256 string
	Normal, Hard                           AsrahanRaidPhase
	Symbols                                map[string]uint32
	Groups                                 []AsrahanSymbolGroup
	HardPartyMax, HardLimitFame            uint32
}

type AsrahanRaidPhase struct {
	TimeoutSeconds, CoinLimit, MinimumClear uint32
	Dungeons                                map[uint32]AsrahanDungeon
	Events                                  []AsrahanSourceEvent
}
type AsrahanDungeon struct {
	Kind, InitialState string
	PartyMax           uint32
	Fields             map[string][]pvf.Token
}
type AsrahanSourceEvent struct{ Trigger, Behavior []pvf.Token }
type AsrahanSymbolGroup struct {
	Header []pvf.Token
	Fields []pvf.Token
}

func asrahanScalar(ts []pvf.Token, tag string) (uint32, error) {
	v := sectionCells(ts, tag)
	if len(v) != 1 || v[0].Type != 0 || v[0].Value < 0 {
		return 0, fmt.Errorf("invalid Asrahan source%s", tag)
	}
	return uint32(v[0].Value), nil
}

func asrahanPhase(ts []pvf.Token, timeout uint32) (AsrahanRaidPhase, error) {
	p := AsrahanRaidPhase{TimeoutSeconds: timeout, Dungeons: map[uint32]AsrahanDungeon{}}
	var err error
	p.CoinLimit, err = asrahanScalar(ts, "[PHASE COIN LIMIT]")
	if err != nil {
		return p, err
	}
	p.MinimumClear, err = asrahanScalar(ts, "[MINIMAL DUNGEON CLEAR COUNT]")
	if err != nil {
		return p, err
	}
	for _, block := range scriptWarpBlocks(ts, "[DUNGEON INFO]") {
		id, err := asrahanScalar(block, "[INDEX]")
		if err != nil || id == 0 {
			return p, fmt.Errorf("invalid Asrahan dungeon identity")
		}
		if _, dup := p.Dungeons[id]; dup {
			return p, fmt.Errorf("duplicate Asrahan dungeon")
		}
		d := AsrahanDungeon{Fields: map[string][]pvf.Token{}}
		for _, tag := range []string{"[TYPE]", "[INIT STATE]"} {
			v := sectionCells(block, tag)
			// The source's neutral dungeon intentionally has no TYPE.
			// Preserve absence rather than assign a boss category.
			if tag == "[TYPE]" && len(v) == 0 {
				continue
			}
			if len(v) != 1 || v[0].Type != 6 || v[0].Text == "" {
				return p, fmt.Errorf("invalid Asrahan dungeon%s", tag)
			}
			if tag == "[TYPE]" {
				d.Kind = v[0].Text
			} else {
				d.InitialState = v[0].Text
			}
		}
		d.PartyMax, err = asrahanScalar(block, "[PARTY MAX]")
		if err != nil || d.PartyMax == 0 {
			return p, fmt.Errorf("invalid Asrahan dungeon party limit")
		}
		for i, t := range block {
			if t.Type != 3 || len(t.Text) < 2 || t.Text[1] == '/' {
				continue
			}
			j := i + 1
			for j < len(block) && block[j].Type != 3 {
				j++
			}
			if _, dup := d.Fields[t.Text]; dup {
				return p, fmt.Errorf("duplicate Asrahan dungeon field")
			}
			d.Fields[t.Text] = append([]pvf.Token(nil), block[i+1:j]...)
		}
		p.Dungeons[id] = d
	}
	for i := 0; i < len(ts); i++ {
		if ts[i].Type != 3 {
			continue
		}
		switch ts[i].Text {
		case "[BEHAVIOR]", "[/BEHAVIOR]", "[/TRIGGER]":
			return p, fmt.Errorf("unpaired Asrahan event block at cell %d", i)
		case "[TRIGGER]":
		default:
			continue
		}
		start := i + 1
		for i++; i < len(ts) && !(ts[i].Type == 3 && ts[i].Text == "[/TRIGGER]"); i++ {
			if ts[i].Type == 3 && (ts[i].Text == "[TRIGGER]" || ts[i].Text == "[BEHAVIOR]" || ts[i].Text == "[/BEHAVIOR]") {
				return p, fmt.Errorf("nested or unclosed Asrahan trigger")
			}
		}
		if i >= len(ts) || i+1 >= len(ts) || ts[i+1].Type != 3 || ts[i+1].Text != "[BEHAVIOR]" {
			return p, fmt.Errorf("Asrahan trigger without paired behavior")
		}
		trigger := append([]pvf.Token(nil), ts[start:i]...)
		start = i + 2
		for i = start; i < len(ts) && !(ts[i].Type == 3 && ts[i].Text == "[/BEHAVIOR]"); i++ {
			if ts[i].Type == 3 && (ts[i].Text == "[TRIGGER]" || ts[i].Text == "[/TRIGGER]" || ts[i].Text == "[BEHAVIOR]") {
				return p, fmt.Errorf("nested or unclosed Asrahan behavior")
			}
		}
		if i >= len(ts) {
			return p, fmt.Errorf("unclosed Asrahan behavior")
		}
		p.Events = append(p.Events, AsrahanSourceEvent{trigger, append([]pvf.Token(nil), ts[start:i]...)})
	}
	if timeout == 0 || len(p.Dungeons) == 0 || len(p.Events) == 0 {
		return p, fmt.Errorf("incomplete Asrahan phase source")
	}
	return p, nil
}

func ParseAsrahanRaidRules(s, symbol ScriptRecord) (*AsrahanRaidRules, error) {
	r := &AsrahanRaidRules{Path: s.Path, SHA256: s.SHA256, SymbolPath: symbol.Path, SymbolSHA256: symbol.SHA256, Symbols: map[string]uint32{}}
	var err error
	r.HardPartyMax, err = asrahanScalar(s.Cells, "[HARD RAID PARTY MAX]")
	if err != nil {
		return nil, err
	}
	r.HardLimitFame, err = asrahanScalar(s.Cells, "[HARD LIMIT FAME]")
	if err != nil {
		return nil, err
	}
	normal, hard := scriptWarpBlocks(s.Cells, "[PHASE]"), scriptWarpBlocks(s.Cells, "[RAID PHASE OF HARDMODE]")
	if len(normal) != 1 || len(hard) != 1 {
		return nil, fmt.Errorf("Asrahan requires separate normal and hard phase trees")
	}
	for _, mode := range []struct {
		ts     []pvf.Token
		tag    string
		target *AsrahanRaidPhase
	}{{normal[0], "[PHASE TIME OVER]", &r.Normal}, {hard[0], "[HARD PHASE TIME OVER]", &r.Hard}} {
		timeout, err := asrahanScalar(s.Cells, mode.tag)
		if err != nil {
			return nil, err
		}
		*mode.target, err = asrahanPhase(mode.ts, timeout)
		if err != nil {
			return nil, err
		}
	}
	ids := map[uint32]bool{}
	for i := 0; i < len(symbol.Cells); i += 2 {
		if symbol.Cells[i].Text == "[SYMBOL TABLE]" {
			break
		}
		if i+1 >= len(symbol.Cells) || symbol.Cells[i].Type != 0 || symbol.Cells[i].Value <= 0 || symbol.Cells[i].Value > math.MaxInt32 || symbol.Cells[i+1].Type != 3 {
			return nil, fmt.Errorf("invalid Asrahan symbol identity")
		}
		name, id := symbol.Cells[i+1].Text, uint32(symbol.Cells[i].Value)
		if _, dup := r.Symbols[name]; dup || ids[id] {
			return nil, fmt.Errorf("duplicate Asrahan symbol")
		}
		r.Symbols[name] = id
		ids[id] = true
	}
	for _, block := range scriptWarpBlocks(symbol.Cells, "[SYMBOL GROUP]") {
		end := 0
		for end < len(block) && block[end].Type != 3 {
			end++
		}
		if end < 2 || block[0].Type != 0 || block[1].Type != 0 || block[0].Value <= 0 || block[1].Value <= 0 {
			return nil, fmt.Errorf("invalid Asrahan symbol group header")
		}
		r.Groups = append(r.Groups, AsrahanSymbolGroup{append([]pvf.Token(nil), block[:end]...), append([]pvf.Token(nil), block[end:]...)})
	}
	if len(r.Symbols) == 0 || len(r.Groups) == 0 {
		return nil, fmt.Errorf("missing Asrahan symbols")
	}
	return r, nil
}

func importAsrahanRaidRules(a *pvf.Archive, path string) (*AsrahanRaidRules, error) {
	s, err := ResolveScript(a, path)
	if err != nil {
		return nil, err
	}
	symbol, err := ResolveScript(a, path+".symbol")
	if err != nil {
		return nil, err
	}
	for i, t := range s.Cells {
		if t.Type == 10 {
			s.Cells[i].Text = a.ResolveString(int(t.Value))
			if s.Cells[i].Text == "" {
				return nil, fmt.Errorf("unresolved Asrahan source expression")
			}
		}
	}
	r, err := ParseAsrahanRaidRules(s, symbol)
	if err != nil {
		return nil, err
	}
	r.MonsterSource, err = importAsrahanMonsterSource(a)
	if err != nil {
		return nil, err
	}
	return r, nil
}
