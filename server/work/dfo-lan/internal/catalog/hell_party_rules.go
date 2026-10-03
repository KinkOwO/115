package catalog

import (
	"fmt"
	"math"
	"strings"

	"dfolan/internal/catalog/pvf"
)

// HellPartyDifficulty preserves the five source columns. Native 147339B10
// stores them as u8/u16/u8/u8/u8; their server reward meaning is not assumed.
type HellPartyDifficulty struct {
	Key     string
	Columns [5]uint32
}

type HellPartyActor struct {
	Template   uint32
	EntityType byte // retain the native byte; 0/1 identify monster/APC
}

type HellPartyGroup struct {
	ID         uint16
	Difficulty string
	Actors     []HellPartyActor
}

type HellPartyRules struct {
	Path, SHA256 string
	Difficulties []HellPartyDifficulty
	Groups       map[uint16]HellPartyGroup
	Actors       map[HellPartyActor]HellPartyActorSource
}

type HellPartyActorSource struct {
	Path, SHA256 string
	Level        byte // APC's own [minimum info] level; monsters use dungeon basis
	HellMonster  bool
	Unavailable  string
}

// ImportHellPartyRules keeps each MOB/AIC binding in its own LIST namespace.
// Unknown modern entity types remain in Groups and cannot become legacy actors.
func ImportHellPartyRules(a *pvf.Archive) (*HellPartyRules, error) {
	if a == nil {
		return nil, fmt.Errorf("Hell Party requires native PVF")
	}
	s, err := ResolveScript(a, "etc/hellparty.etc")
	if err != nil {
		return nil, err
	}
	r, err := ParseHellPartyRules(s)
	if err != nil {
		return nil, err
	}
	r.Actors = map[HellPartyActor]HellPartyActorSource{}
	for kind, listing := range []string{"list/monster.lst", "list/aicharacter.lst"} {
		index, err := ResolveScript(a, listing)
		if err != nil {
			return nil, err
		}
		rows, err := ParseIndex(index.Cells)
		if err != nil {
			return nil, err
		}
		paths := map[uint32]string{}
		for _, row := range rows {
			paths[row.ID] = row.Path
		}
		for _, group := range r.Groups {
			for _, actor := range group.Actors {
				if actor.EntityType != byte(kind) {
					continue
				}
				if _, exists := r.Actors[actor]; exists {
					continue
				}
				p, exists := paths[actor.Template]
				if !exists {
					r.Actors[actor] = HellPartyActorSource{Unavailable: "template absent from " + listing}
					continue
				}
				if kind == 0 {
					p = monsterItemScriptPath(a, p)
				} else if _, found := a.FindFile(p); !found && !strings.HasPrefix(p, "aicharacter/") {
					p = "aicharacter/" + p
				}
				script, err := ResolveScript(a, p)
				if err != nil {
					r.Actors[actor] = HellPartyActorSource{Path: p, Unavailable: err.Error()}
					continue
				}
				source, err := ParseHellPartyActorSource(script, byte(kind))
				if err != nil {
					source.Unavailable = err.Error()
				}
				r.Actors[actor] = source
			}
		}
	}
	return &r, nil
}

// The APC level projection matches S4's [minimum info] reader. Its use as
// the server spawn level is an explicitly approved compatibility policy.
func ParseHellPartyActorSource(s ScriptRecord, kind byte) (HellPartyActorSource, error) {
	out := HellPartyActorSource{Path: s.Path, SHA256: s.SHA256}
	if len(s.SHA256) != 64 {
		return out, fmt.Errorf("Hell actor without provenance")
	}
	cells := sectionCells(s.Cells, "[hell monster]")
	if len(cells) != 0 {
		if len(cells) != 1 || cells[0].Type != 0 || cells[0].Value < 0 || cells[0].Value > 1 {
			return out, fmt.Errorf("invalid [hell monster]")
		}
		out.HellMonster = cells[0].Value == 1
	}
	switch kind {
	case 0:
		if !strings.HasSuffix(s.Path, ".mob") {
			return out, fmt.Errorf("Hell monster without MOB")
		}
	case 1:
		cells := sectionCells(s.Cells, "[minimum info]")
		if !strings.HasSuffix(s.Path, ".aic") || len(cells) < 6 || cells[5].Type != 0 || cells[5].Value < 1 || cells[5].Value > 255 {
			return out, fmt.Errorf("APC source level unavailable")
		}
		out.Level = byte(cells[5].Value)
	default:
		return out, fmt.Errorf("unsupported Hell entity type %d", kind)
	}
	return out, nil
}

// ParseHellPartyRules follows the current 115 importHellPartyScript reader
// 147339B10. Basic/random-party values are outside this projection. Keeping
// source keys and columns does not authorize a legacy reward formula.
func ParseHellPartyRules(s ScriptRecord) (HellPartyRules, error) {
	out := HellPartyRules{Path: s.Path, SHA256: s.SHA256, Groups: map[uint16]HellPartyGroup{}}
	if s.Path != "etc/hellparty.etc" || len(s.SHA256) != 64 {
		return out, fmt.Errorf("Hell Party rules without source provenance")
	}
	c := sectionCells(s.Cells, "[difficulty]")
	if len(c) < 1 || c[0].Type != 0 || c[0].Value < 1 || c[0].Value > 6 || len(c) != 1+int(c[0].Value)*6 {
		return out, fmt.Errorf("%s: invalid Hell difficulty rows", s.Path)
	}
	keys := map[string]bool{}
	for i := 1; i < len(c); i += 6 {
		key := c[i]
		if key.Type != 6 || len(key.Text) != 1 || key.Text[0] < 'A' || key.Text[0] > 'F' || keys[key.Text] {
			return out, fmt.Errorf("%s: invalid Hell difficulty key at cell %d", s.Path, i)
		}
		keys[key.Text] = true
		row := HellPartyDifficulty{Key: key.Text}
		for j, limit := range [5]int32{math.MaxUint8, math.MaxUint16, math.MaxUint8, math.MaxUint8, math.MaxUint8} {
			v := c[i+1+j]
			if v.Type != 0 || v.Value < 0 || v.Value > limit {
				return out, fmt.Errorf("%s: invalid Hell difficulty %s column %d", s.Path, key.Text, j+1)
			}
			row.Columns[j] = uint32(v.Value)
		}
		out.Difficulties = append(out.Difficulties, row)
	}
	// This section has nested type-3 tags, so sectionCells cannot extract it.
	i := -1
	for n, cell := range s.Cells {
		if cell.Type == 3 && cell.Text == "[hellparty monster group]" {
			if i != -1 {
				return out, fmt.Errorf("%s: duplicate Hell group section", s.Path)
			}
			i = n + 1
		}
	}
	if i == -1 {
		return out, fmt.Errorf("%s: Hell groups absent", s.Path)
	}
	tag := func(name string) bool { return i < len(s.Cells) && s.Cells[i].Type == 3 && s.Cells[i].Text == name }
	for !tag("[/hellparty monster group]") {
		if !tag("[group index]") {
			return out, fmt.Errorf("%s: expected Hell group index at cell %d", s.Path, i)
		}
		i++
		if i >= len(s.Cells) || s.Cells[i].Type != 0 || s.Cells[i].Value <= 0 || s.Cells[i].Value > math.MaxUint16 {
			return out, fmt.Errorf("%s: invalid Hell group index", s.Path)
		}
		id := uint16(s.Cells[i].Value)
		i++
		if _, exists := out.Groups[id]; exists {
			return out, fmt.Errorf("%s: duplicate Hell group %d", s.Path, id)
		}
		if !tag("[group]") {
			return out, fmt.Errorf("%s: Hell group %d body absent", s.Path, id)
		}
		i++
		if i >= len(s.Cells) || s.Cells[i].Type != 6 || !keys[s.Cells[i].Text] {
			return out, fmt.Errorf("%s: Hell group %d difficulty absent", s.Path, id)
		}
		group := HellPartyGroup{ID: id, Difficulty: s.Cells[i].Text}
		i++
		for !tag("[/group]") {
			if i+1 >= len(s.Cells) || s.Cells[i].Type != 0 || s.Cells[i].Value <= 0 || s.Cells[i+1].Type != 0 || s.Cells[i+1].Value < 0 || s.Cells[i+1].Value > math.MaxUint8 || len(group.Actors) >= 16 {
				return out, fmt.Errorf("%s: invalid Hell group %d actor at cell %d", s.Path, id, i)
			}
			group.Actors = append(group.Actors, HellPartyActor{Template: uint32(s.Cells[i].Value), EntityType: byte(s.Cells[i+1].Value)})
			i += 2
		}
		if len(group.Actors) == 0 {
			return out, fmt.Errorf("%s: empty Hell group %d", s.Path, id)
		}
		out.Groups[id] = group
		i++
	}
	if len(out.Groups) == 0 {
		return out, fmt.Errorf("%s: empty Hell groups", s.Path)
	}
	return out, nil
}

type HellPartyMapChoice struct {
	Group  uint16
	Weight uint32
	Order  uint16
}

// The [hellparty] delimiters inside a [special passive object] are type-6
// strings, not section tags. Native 1471E3A90 reads triples and rejects
// orders outside 1..9. Preserve the object's section offset as its identity;
// unrelated trap/item payloads need not be interpreted to extract choices.
type HellPartyPillar struct {
	SectionOffset int
	Object        uint32
	Choices       []HellPartyMapChoice
}

func ParseHellPartyPillars(s ScriptRecord) ([]HellPartyPillar, error) {
	var out []HellPartyPillar
	c := sectionCells(s.Cells, "[special passive object]")
	for i := 0; i < len(c); i++ {
		if c[i].Type != 6 || c[i].Text != "[hellparty]" {
			continue
		}
		if i < 5 {
			return nil, fmt.Errorf("%s: Hell pillar header absent", s.Path)
		}
		start := i - 5
		for j := start; j < i; j++ {
			if c[j].Type != 0 {
				return nil, fmt.Errorf("%s: invalid Hell pillar header", s.Path)
			}
		}
		if c[start].Value <= 0 {
			return nil, fmt.Errorf("%s: invalid Hell pillar object", s.Path)
		}
		pillar := HellPartyPillar{SectionOffset: start, Object: uint32(c[start].Value)}
		i++
		for i < len(c) && !(c[i].Type == 6 && c[i].Text == "[/hellparty]") {
			if i+2 >= len(c) || c[i].Type != 0 || c[i+1].Type != 0 || c[i+2].Type != 0 || c[i].Value <= 0 || c[i].Value > math.MaxUint16 || c[i+1].Value < 0 || c[i+2].Value < 1 || c[i+2].Value > 9 {
				return nil, fmt.Errorf("%s: invalid Hell pillar choice at cell %d", s.Path, i)
			}
			pillar.Choices = append(pillar.Choices, HellPartyMapChoice{Group: uint16(c[i].Value), Weight: uint32(c[i+1].Value), Order: uint16(c[i+2].Value)})
			i += 3
		}
		if i >= len(c) || len(pillar.Choices) == 0 {
			return nil, fmt.Errorf("%s: empty or unterminated Hell pillar", s.Path)
		}
		out = append(out, pillar)
	}
	return out, nil
}
