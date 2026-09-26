package catalog

import (
	"fmt"

	"dfolan/internal/catalog/pvf"
)

// DropGroupSource records which PVF script a drop group table was read from.
type DropGroupSource struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// DropWeight is one item row of a drop group. Weight 0 is preserved rather than
// rejected: the current table really does carry zero-weight rows, which means
// such an entry can never be selected. Treating that as malformed would reject
// a table the client ships.
type DropWeight struct {
	Template uint32 `json:"template"`
	Weight   uint32 `json:"weight"`
}

// DropGroup is one [group] block of etc/dungeondroptablebygroup.etc.
//
// The file is a flat stream of [table] blocks; each block holds exactly one
// [group], and the group header has no closing tag - the block ends where the
// table does. Both item sections are kept apart because the source names them
// differently and nothing establishes that they mean the same thing:
//
//	[drop item]       -> Explicit
//	[smart drop item] -> Smart
//
// [creation rate] carries two integers and [armor creation rate] one. Eight
// groups in the current table also declare [target] with a string cell
// ("weapon legacy"); it is preserved verbatim. What any of these gate is not
// established, so nothing here is given invented semantics.
type DropGroup struct {
	ID           uint32       `json:"id"`
	CreationRate []int32      `json:"creation_rate,omitempty"`
	ArmorRate    *int32       `json:"armor_creation_rate,omitempty"`
	Target       string       `json:"target,omitempty"`
	Explicit     []DropWeight `json:"drop_item,omitempty"`
	Smart        []DropWeight `json:"smart_drop_item,omitempty"`
}

type dropGroupParser struct {
	groups  []DropGroup
	cur     *DropGroup
	section string
	open    bool
	pending int32
	havePnd bool
	seen    map[uint32]bool
}

// ParseDropGroups reads the typed cells of etc/dungeondroptablebygroup.etc into
// group rows. It is deliberately strict: a table whose structure does not match
// the shape observed in the current client is refused instead of being silently
// half-read, because a half-read table would award the wrong items.
func ParseDropGroups(cells []pvf.Token) ([]DropGroup, error) {
	p := dropGroupParser{seen: map[uint32]bool{}}
	for _, t := range cells {
		if t.Type == 3 {
			if e := p.header(t.Text); e != nil {
				return nil, e
			}
			continue
		}
		switch t.Type {
		case 0:
			if e := p.number(t.Value); e != nil {
				return nil, e
			}
		case 6, 8:
			if e := p.label(t.Text, t.Reference); e != nil {
				return nil, e
			}
		default:
			return nil, fmt.Errorf("drop group cell type %d is not handled", t.Type)
		}
	}
	if p.open || p.cur != nil {
		return nil, fmt.Errorf("unterminated drop table")
	}
	if len(p.groups) == 0 {
		return nil, fmt.Errorf("drop table holds no group")
	}
	return p.groups, nil
}

func (p *dropGroupParser) header(text string) error {
	switch text {
	case "[table]":
		if p.open {
			return fmt.Errorf("nested drop table")
		}
		p.open = true
		p.section = ""
		return nil
	case "[/table]":
		if !p.open {
			return fmt.Errorf("drop table closed without opening")
		}
		if e := p.flush(); e != nil {
			return e
		}
		p.open = false
		return nil
	case "[group]":
		if !p.open {
			return fmt.Errorf("drop group outside a table")
		}
		if p.cur != nil {
			return fmt.Errorf("drop table holds more than one group")
		}
		p.cur = &DropGroup{}
		p.section = ""
		return nil
	}
	if p.cur == nil {
		return fmt.Errorf("drop table cell outside a group: %s", text)
	}
	if p.havePnd && p.section != "drop" && p.section != "smart" {
		return fmt.Errorf("drop group %d has a dangling item template", p.cur.ID)
	}
	switch text {
	case "[creation rate]":
		p.section = "creation"
	case "[armor creation rate]":
		p.section = "armor"
	case "[target]":
		p.section = "target"
	case "[drop item]":
		p.section, p.havePnd = "drop", false
	case "[smart drop item]":
		p.section, p.havePnd = "smart", false
	case "[/drop item]", "[/smart drop item]":
		want := "drop"
		if text == "[/smart drop item]" {
			want = "smart"
		}
		if p.section != want {
			return fmt.Errorf("drop group %d closes %s while %s is open", p.cur.ID, text, p.section)
		}
		if p.havePnd {
			return fmt.Errorf("drop group %d ends %s on a dangling item template", p.cur.ID, text)
		}
		p.section = ""
	default:
		return fmt.Errorf("unknown drop group section %s", text)
	}
	return nil
}

func (p *dropGroupParser) number(v int32) error {
	if p.cur == nil {
		return fmt.Errorf("drop group number outside a group")
	}
	switch p.section {
	case "":
		if p.cur.ID != 0 {
			return fmt.Errorf("unexpected second drop group id")
		}
		if v <= 0 {
			return fmt.Errorf("non-positive drop group id")
		}
		p.cur.ID = uint32(v)
	case "creation":
		if len(p.cur.CreationRate) >= 2 {
			return fmt.Errorf("drop group %d has more than two creation rates", p.cur.ID)
		}
		p.cur.CreationRate = append(p.cur.CreationRate, v)
	case "armor":
		if p.cur.ArmorRate != nil {
			return fmt.Errorf("drop group %d repeats armor creation rate", p.cur.ID)
		}
		rate := v
		p.cur.ArmorRate = &rate
	case "drop", "smart":
		if !p.havePnd {
			if v <= 0 {
				return fmt.Errorf("non-positive item template in drop group %d", p.cur.ID)
			}
			p.pending, p.havePnd = v, true
			return nil
		}
		if v < 0 {
			return fmt.Errorf("negative item weight in drop group %d", p.cur.ID)
		}
		w := DropWeight{Template: uint32(p.pending), Weight: uint32(v)}
		if p.section == "drop" {
			p.cur.Explicit = append(p.cur.Explicit, w)
		} else {
			p.cur.Smart = append(p.cur.Smart, w)
		}
		p.havePnd = false
	default:
		return fmt.Errorf("drop group %d has a number in section %s", p.cur.ID, p.section)
	}
	return nil
}

func (p *dropGroupParser) label(text, reference string) error {
	if p.cur == nil {
		return fmt.Errorf("drop group label outside a group")
	}
	if p.section != "target" {
		return fmt.Errorf("drop group %d has a string in section %s", p.cur.ID, p.section)
	}
	if p.cur.Target != "" {
		return fmt.Errorf("drop group %d repeats its target label", p.cur.ID)
	}
	if text == "" {
		text = reference
	}
	if text == "" {
		return fmt.Errorf("drop group %d has an empty target label", p.cur.ID)
	}
	p.cur.Target = text
	return nil
}

func (p *dropGroupParser) flush() error {
	if p.cur == nil {
		return fmt.Errorf("drop table without a group")
	}
	if p.havePnd {
		return fmt.Errorf("drop group %d ends on a dangling item template", p.cur.ID)
	}
	if len(p.cur.CreationRate) == 1 {
		return fmt.Errorf("drop group %d has a single creation rate", p.cur.ID)
	}
	if len(p.cur.Explicit) == 0 && len(p.cur.Smart) == 0 {
		return fmt.Errorf("drop group %d holds no item", p.cur.ID)
	}
	if p.seen[p.cur.ID] {
		return fmt.Errorf("duplicate drop group %d", p.cur.ID)
	}
	p.seen[p.cur.ID] = true
	p.groups = append(p.groups, *p.cur)
	p.cur = nil
	p.section = ""
	return nil
}

// DropGroupByID looks a group up in an imported catalog.
func (c LootCatalog) DropGroupByID(id uint32) (DropGroup, bool) {
	for _, g := range c.DropGroups {
		if g.ID == id {
			return g, true
		}
	}
	return DropGroup{}, false
}

// [normal group index] is deliberately left undecoded.
//
// The encoding of [normal group index] is NOT established. A sweep over every
// block in the shipped catalog (see TestNormalGroupIndexEncodingIsAmbiguous)
// finds 3419 sections: 2567 fit both a length-prefixed reading (a count followed
// by that many group ids) and a paired reading (count, group, count, ...), 758
// fit only the length-prefixed one and 94 only the paired one. Both readings
// have exclusive cases, so no decoder is shipped - picking either would award
// the wrong items on the sections the other one explains.
//
// DungeonDropSection is one header of a dungeon drop block with its cells
// verbatim. Values holds the numeric cells and Labels the string cells, in
// source order within each kind.
type DungeonDropSection struct {
	Header string   `json:"header"`
	Values []int32  `json:"values,omitempty"`
	Labels []string `json:"labels,omitempty"`
}

// DungeonDropBlock is one block of a dungeon's [difficulty dropitem group list].
// The source wraps blocks in either [group info] or [custom group info]; the
// wrapper is kept in Kind because the two are not interchangeable in the data.
type DungeonDropBlock struct {
	Kind     string               `json:"kind"`
	Sections []DungeonDropSection `json:"sections"`
}

const (
	dungeonDropListOpen    = "[difficulty dropitem group list]"
	dungeonDropListClose   = "[/difficulty dropitem group list]"
	dungeonGroupInfoOpen   = "[group info]"
	dungeonGroupInfoClose  = "[/group info]"
	dungeonCustomInfoOpen  = "[custom group info]"
	dungeonCustomInfoClose = "[/custom group info]"
)

// ParseDungeonDropBlocks reads a dungeon script's [difficulty dropitem group
// list]. A dungeon without the section yields nil and no error: most dungeons
// rely on the global drop tables instead.
func ParseDungeonDropBlocks(cells []pvf.Token) ([]DungeonDropBlock, error) {
	var spans [][2]int
	start := -1
	for i, c := range cells {
		if c.Type != 3 {
			continue
		}
		switch c.Text {
		case dungeonDropListOpen:
			if start >= 0 {
				return nil, fmt.Errorf("nested %s", dungeonDropListOpen)
			}
			start = i
		case dungeonDropListClose:
			if start < 0 {
				return nil, fmt.Errorf("%s without an opening tag", dungeonDropListClose)
			}
			spans = append(spans, [2]int{start, i})
			start = -1
		}
	}
	if start >= 0 {
		return nil, fmt.Errorf("unterminated %s", dungeonDropListOpen)
	}
	if len(spans) == 0 {
		return nil, nil
	}
	first := cells[spans[0][0]+1 : spans[0][1]]
	blocks, e := parseDungeonDropBlocks(first)
	if e != nil {
		return nil, e
	}
	for _, sp := range spans[1:] {
		// One dungeon in the shipped catalog (5410) repeats the whole list byte for
		// byte. An identical repeat adds nothing and is dropped; a second list that
		// differs is a shape nobody has looked at, so it is refused rather than merged
		// into the first - merging could double the awarded items.
		if !sameTokens(cells[sp[0]+1:sp[1]], first) {
			return nil, fmt.Errorf("%s repeats with different content", dungeonDropListOpen)
		}
	}
	return blocks, nil
}

func sameTokens(a, b []pvf.Token) bool {
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

func parseDungeonDropBlocks(cells []pvf.Token) ([]DungeonDropBlock, error) {
	var out []DungeonDropBlock
	var cur *DungeonDropBlock
	var section *DungeonDropSection
	flushSection := func() {
		if section == nil {
			return
		}
		if len(section.Values) == 0 && len(section.Labels) == 0 {
			section = nil
			return
		}
		cur.Sections = append(cur.Sections, *section)
		section = nil
	}
	flushBlock := func(kind string) error {
		if cur == nil {
			return fmt.Errorf("%s closed without opening", kind)
		}
		flushSection()
		// An empty wrapper is real data: the shipped catalog contains
		// [custom group info] immediately followed by its closing tag.
		out = append(out, *cur)
		cur = nil
		return nil
	}
	for _, c := range cells {
		if c.Type == 3 {
			switch c.Text {
			case dungeonGroupInfoOpen, dungeonCustomInfoOpen:
				if cur != nil {
					return nil, fmt.Errorf("nested %s", c.Text)
				}
				cur = &DungeonDropBlock{Kind: c.Text}
				section = nil
			case dungeonGroupInfoClose, dungeonCustomInfoClose:
				if cur == nil {
					return nil, fmt.Errorf("%s without any opening tag", c.Text)
				}
				if cur.Kind == "" {
					cur.Kind = openOf(c.Text)
				} else if cur.Kind != openOf(c.Text) {
					return nil, fmt.Errorf("%s closed by %s", cur.Kind, c.Text)
				}
				if e := flushBlock(cur.Kind); e != nil {
					return nil, e
				}
				section = nil
			default:
				if cur == nil {
					// One dungeon in the shipped catalog (100002889) omits the opening
					// [group info] of its first block. The closing tag still identifies the
					// wrapper, so the block is opened implicitly and its kind is taken from
					// that tag; an implicit block that never closes stays an error.
					cur = &DungeonDropBlock{}
				}
				flushSection()
				section = &DungeonDropSection{Header: c.Text}
			}
			continue
		}
		if cur == nil || section == nil {
			return nil, fmt.Errorf("drop list cell outside a section")
		}
		switch c.Type {
		case 0:
			section.Values = append(section.Values, c.Value)
		case 6, 8:
			label := c.Text
			if label == "" {
				label = c.Reference
			}
			if label == "" {
				return nil, fmt.Errorf("%s has an empty string cell", section.Header)
			}
			section.Labels = append(section.Labels, label)
		default:
			return nil, fmt.Errorf("drop list cell type %d is not handled", c.Type)
		}
	}
	if cur != nil {
		return nil, fmt.Errorf("unterminated %s", cur.Kind)
	}
	if len(out) == 0 {
		// A present but empty list is real: dungeon 100004448 ships the opening and
		// closing tags with nothing between them.
		return nil, nil
	}
	return out, nil
}

func openOf(close string) string {
	switch close {
	case dungeonGroupInfoClose:
		return dungeonGroupInfoOpen
	case dungeonCustomInfoClose:
		return dungeonCustomInfoOpen
	}
	return ""
}
