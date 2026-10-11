package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// The current native loader (144cf2bba -> 1477332b0) reads this COS.
// Its indices and positions are content, not a second Go monster table.
const asrahanMonsterSourcePath = "contents/2024/asrahanraid/etc/asrahanmonster.cos"

type AsrahanMonsterSource struct {
	Path, SHA256 string
	Monsters     map[string]AsrahanMonster
}

type AsrahanMonster struct {
	Kind     string
	Template uint32
	X, Y     int32
	Normal   bool
}

func ParseAsrahanMonsterSource(s ScriptRecord) (*AsrahanMonsterSource, error) {
	if s.Path == "" || s.SHA256 == "" {
		return nil, fmt.Errorf("Asrahan monster source identity missing")
	}
	parents := scriptWarpBlocks(s.Cells, "[MONSTERS]")
	if len(parents) != 1 {
		return nil, fmt.Errorf("Asrahan monster source requires one MONSTERS table")
	}
	out := &AsrahanMonsterSource{Path: s.Path, SHA256: s.SHA256, Monsters: map[string]AsrahanMonster{}}
	ids := map[uint32]bool{}
	for _, block := range scriptWarpBlocks(parents[0], "[MONSTER INFO]") {
		kind := sectionCells(block, "[MONSTER TYPE]")
		id := sectionCells(block, "[MONSTER INDEX]")
		position := sectionCells(block, "[POSITION]")
		if len(kind) != 1 || kind[0].Type != 6 || kind[0].Text == "" || len(id) != 1 || id[0].Type != 0 || id[0].Value < 0 || len(position) != 2 {
			return nil, fmt.Errorf("invalid Asrahan monster identity/position")
		}
		for _, v := range position {
			if v.Type != 0 {
				return nil, fmt.Errorf("Asrahan monster position is not an integer")
			}
		}
		m := AsrahanMonster{Kind: kind[0].Text, Template: uint32(id[0].Value), X: position[0].Value, Y: position[1].Value}
		if _, dup := out.Monsters[m.Kind]; dup {
			return nil, fmt.Errorf("duplicate Asrahan monster kind %s", m.Kind)
		}
		if m.Template != 0 {
			if ids[m.Template] {
				return nil, fmt.Errorf("ambiguous Asrahan monster template %d", m.Template)
			}
			ids[m.Template] = true
		}
		if normal := sectionCells(block, "[IS NORMAL MONSTER]"); len(normal) > 0 {
			if len(normal) != 1 || normal[0].Type != 0 || (normal[0].Value != 0 && normal[0].Value != 1) {
				return nil, fmt.Errorf("invalid Asrahan normal monster flag")
			}
			m.Normal = normal[0].Value == 1
		}
		out.Monsters[m.Kind] = m
	}
	if len(out.Monsters) == 0 {
		return nil, fmt.Errorf("empty Asrahan monster source")
	}
	return out, nil
}

// Resolve a phase-owned group against its own COS monster index. Dungeon TYPE
// is not a boss identity: current Terminus dungeons have TYPE mu. Template zero is a
// source placeholder (Lopes in the current COS), never permission to invent a
// template or reuse another boss's coordinates.
func (r *AsrahanRaidRules) MonsterForGroup(p AsrahanRaidPhase, index uint32) (AsrahanCombatGroup, AsrahanMonster, error) {
	var empty AsrahanCombatGroup
	var absent AsrahanMonster
	if r == nil || r.MonsterSource == nil {
		return empty, absent, fmt.Errorf("Asrahan monster COS missing")
	}
	g, err := r.CombatGroup(p, index)
	if err != nil {
		return empty, absent, err
	}
	var found *AsrahanMonster
	for _, m := range r.MonsterSource.Monsters {
		if m.Template != g.Monster || m.Template == 0 {
			continue
		}
		if found != nil {
			return empty, absent, fmt.Errorf("ambiguous Asrahan group monster index")
		}
		copy := m
		found = &copy
	}
	if found == nil || found.Normal {
		return empty, absent, fmt.Errorf("Asrahan group %d has no matching source boss spawn (monster %d)", index, g.Monster)
	}
	return g, *found, nil
}

// A map-native boss can own a group without a dynamic COS spawn record.
// Resolve group ownership first; do not let a zero COS placeholder replace
// a real source-map actor or borrow another group's coordinates.
func (r *AsrahanRaidRules) CombatGroup(p AsrahanRaidPhase, index uint32) (AsrahanCombatGroup, error) {
	groups, err := CompileAsrahanCombatGroups(r, p)
	if err != nil {
		return AsrahanCombatGroup{}, err
	}
	for _, g := range groups {
		if g.Index == index {
			return g, nil
		}
	}
	return AsrahanCombatGroup{}, fmt.Errorf("Asrahan group %d is not owned by this phase", index)
}

func importAsrahanMonsterSource(a *pvf.Archive) (*AsrahanMonsterSource, error) {
	raw, err := a.ReadRaw(asrahanMonsterSourcePath)
	if err != nil {
		return nil, err
	}
	// Reuse the existing native UTF-16 COS lexer, including comment handling.
	// COS text is not a five-byte-cell ETC script.
	cells, err := readBakalCOSTokens(a, asrahanMonsterSourcePath)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(raw)
	return ParseAsrahanMonsterSource(ScriptRecord{Path: asrahanMonsterSourcePath, SHA256: fmt.Sprintf("%x", hash), Cells: cells})
}
