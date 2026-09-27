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

	// RandomOptionLevel is the payload of [create random option level]: three
	// integers in all thirteen groups that carry the section. The shipped table
	// gained this section after the parser was written, and because the parser
	// refuses what it does not know, the whole table became unreadable - group
	// 21250/21251 (the two [smart drop group id] targets of the endkeeper
	// carriers 10362480/10362481) could not be read at all. The numbers are kept
	// verbatim: what they mean is not established, and a guess here would award
	// the wrong items.
	RandomOptionLevel []int32 `json:"create_random_option_level,omitempty"`

	// ZeroPriceDrop records [zero price drop], which carries no payload at all
	// (thirteen groups, each immediately followed by [smart drop item]). It is a
	// flag of unknown effect, so only its presence is recorded.
	ZeroPriceDrop bool `json:"zero_price_drop,omitempty"`
}

// randomOptionLevelCount is the payload width of [create random option level] as
// the shipped table writes it: thirteen occurrences, three numbers each. A
// different width is refused rather than half-read.
const randomOptionLevelCount = 3

type dropGroupParser struct {
	groups  []DropGroup
	cur     *DropGroup
	section string
	open    bool
	pending int32
	havePnd bool
	// items records that the group declared an item section at all. A section
	// that opens and closes with nothing between it is real data (group 20363),
	// but a group with no item section is a misread and stays an error.
	items bool
	// strict turns a group this file cannot read into an error (the default).
	// It is switched off only around the single section that ends mid-pair, so
	// the group is skipped instead of taking the whole table down with it.
	strict     bool
	unreadable []uint32
	skipped    map[uint32]bool
	seen       map[uint32]bool
	first      map[uint32]DropGroup
	// dropped records that the group just read was skipped, so the table that
	// held it can still close normally.
	dropped bool
}

// ParseDropGroups reads the typed cells of etc/dungeondroptablebygroup.etc into
// group rows. It is deliberately strict: a table whose structure does not match
// the shape observed in the current client is refused instead of being silently
// half-read, because a half-read table would award the wrong items.
//
// One exception is deliberate and narrow. A single group in the shipped table
// (21469) declares [drop item] with an odd number of numbers - eleven bare
// templates and no weights - which no reading explains: pairs leave a dangling
// template, and a bare list would need a weight the source never wrote. Its
// neighbours all write explicit (template, 1) pairs, so this is an irregularity
// rather than a shape. Such a group is skipped and its id comes back in
// unreadable; refusing the whole table instead (the previous behaviour) made
// every group unreadable, and guessing a weight would award the wrong items.
func ParseDropGroups(cells []pvf.Token) ([]DropGroup, []uint32, error) {
	p := dropGroupParser{
		seen:   map[uint32]bool{},
		first:  map[uint32]DropGroup{},
		strict: true,
	}
	for _, t := range cells {
		if t.Type == 3 {
			if e := p.header(t.Text); e != nil {
				return nil, nil, e
			}
			continue
		}
		switch t.Type {
		case 0:
			if e := p.number(t.Value); e != nil {
				return nil, nil, e
			}
		case 6, 8:
			if e := p.label(t.Text, t.Reference); e != nil {
				return nil, nil, e
			}
		default:
			return nil, nil, fmt.Errorf("drop group cell type %d is not handled", t.Type)
		}
	}
	if p.open || p.cur != nil {
		return nil, nil, fmt.Errorf("unterminated drop table")
	}
	if len(p.groups) == 0 {
		return nil, nil, fmt.Errorf("drop table holds no group")
	}
	return p.groups, p.unreadable, nil
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
		p.section, p.havePnd, p.items = "drop", false, true
	case "[smart drop item]":
		p.section, p.havePnd, p.items = "smart", false, true
	case "[create random option level]":
		p.section = "randomoption"
	case "[zero price drop]":
		p.section = "zeroprice"
		p.cur.ZeroPriceDrop = true
	case "[/drop item]", "[/smart drop item]":
		want := "drop"
		if text == "[/smart drop item]" {
			want = "smart"
		}
		if p.section != want {
			return fmt.Errorf("drop group %d closes %s while %s is open", p.cur.ID, text, p.section)
		}
		if p.havePnd {
			// The one irregularity in the shipped table: eleven bare templates
			// with no weights. The group is dropped, loudly.
			p.skip(p.cur.ID)
			return nil
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
	case "randomoption":
		if len(p.cur.RandomOptionLevel) >= randomOptionLevelCount {
			return fmt.Errorf("drop group %d has more than %d random option levels",
				p.cur.ID, randomOptionLevelCount)
		}
		p.cur.RandomOptionLevel = append(p.cur.RandomOptionLevel, v)
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
		// A table whose only group was skipped still closes normally.
		if p.dropped {
			p.dropped = false
			return nil
		}
		return fmt.Errorf("drop table without a group")
	}
	if p.havePnd {
		p.skip(p.cur.ID)
		return nil
	}
	if len(p.cur.CreationRate) == 1 {
		return fmt.Errorf("drop group %d has a single creation rate", p.cur.ID)
	}
	if n := len(p.cur.RandomOptionLevel); n != 0 && n != randomOptionLevelCount {
		return fmt.Errorf("drop group %d has %d random option levels, want %d",
			p.cur.ID, n, randomOptionLevelCount)
	}
	if len(p.cur.Explicit) == 0 && len(p.cur.Smart) == 0 && !p.items {
		return fmt.Errorf("drop group %d declares no item section", p.cur.ID)
	}
	if p.seen[p.cur.ID] {
		// One group in the shipped table is repeated byte for byte (50013). An
		// identical repeat adds nothing, so it is dropped - the same policy the
		// dungeon drop list uses. A repeat that *differs* makes the id ambiguous,
		// so it is skipped loudly instead of one of the two winning by order.
		if sameDropGroup(p.first[p.cur.ID], *p.cur) {
			p.cur = nil
			p.section = ""
			p.items = false
			return nil
		}
		p.skip(p.cur.ID)
		return nil
	}
	p.seen[p.cur.ID] = true
	p.first[p.cur.ID] = *p.cur
	p.groups = append(p.groups, *p.cur)
	p.cur = nil
	p.section = ""
	p.items = false
	return nil
}

// skip drops the group being read and records its id. Only used for data this
// file cannot decode; the caller sees the id instead of a guessed distribution.
func (p *dropGroupParser) skip(id uint32) {
	if p.cur == nil {
		return
	}
	// A skipped id is tracked separately from seen: whether it may appear again as
	// a well-formed group is a different question from whether it was already
	// skipped.
	if id != 0 && !p.skipped[id] {
		if p.skipped == nil {
			p.skipped = map[uint32]bool{}
		}
		p.skipped[id] = true
		p.unreadable = append(p.unreadable, id)
	}
	p.cur = nil
	p.section = ""
	p.pending, p.havePnd = 0, false
	p.items = false
	p.dropped = true
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

// sameDropGroup reports whether two groups carry the same rows. It is what makes
// an identical duplicate harmless and a differing one ambiguous.
func sameDropGroup(a, b DropGroup) bool {
	if a.ID != b.ID || a.Target != b.Target || !equalInt32Slice(a.CreationRate, b.CreationRate) {
		return false
	}
	if (a.ArmorRate == nil) != (b.ArmorRate == nil) {
		return false
	}
	if a.ArmorRate != nil && *a.ArmorRate != *b.ArmorRate {
		return false
	}
	return equalDropWeights(a.Explicit, b.Explicit) && equalDropWeights(a.Smart, b.Smart)
}

func equalInt32Slice(a, b []int32) bool {
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

func equalDropWeights(a, b []DropWeight) bool {
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
