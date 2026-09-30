package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"os"
)

// OdysseyChapterDropModel is the model tag of
// odyssey-chapter-drop-release.json (see cmd/odysseychapterimport).
const OdysseyChapterDropModel = "source-odyssey-chapter-drop-v1"

// ChapterDropLine is the box a chapter's final lord drops. Template 0 means the
// journal gives that chapter no equipment selection box (chapter 7 — both of
// its reward boxes are plain [booster] items).
type ChapterDropLine struct {
	Chapter  uint8  `json:"chapter"`
	Final    uint32 `json:"final"`
	Template uint32 `json:"template"`
	Rate     uint32 `json:"rate"`
	Enabled  bool   `json:"enabled"`
}

// OdysseyChapterDrop holds the per-chapter final-lord box drops.
//
// It ships disabled: every line is rate 0 / enabled false until an operator
// profile turns it on. A disabled line is inert — it does not even consume a
// roll seed, so enabling it later cannot shift any other drop.
type OdysseyChapterDrop struct {
	Model         string              `json:"model"`
	Source        pvf.ArchiveSnapshot `json:"source"`
	JournalPath   string              `json:"journal_path"`
	DefinitionSHA string              `json:"definition_sha256"`
	Drops         []ChapterDropLine   `json:"drops"`

	byDungeon map[uint32]ChapterDropLine
}

func LoadOdysseyChapterDrop(path string) (*OdysseyChapterDrop, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var d OdysseyChapterDrop
	if e = json.Unmarshal(b, &d); e != nil {
		return nil, e
	}
	return NewOdysseyChapterDrop(d)
}

// NewOdysseyChapterDrop shares validation and runtime index construction across native and JSON sources.
func NewOdysseyChapterDrop(d OdysseyChapterDrop) (*OdysseyChapterDrop, error) {
	if d.Model != OdysseyChapterDropModel || d.Source.Checksum != catalog.OdysseySource || d.DefinitionSHA != catalog.OdysseyChaptersJournalSHA {
		return nil, fmt.Errorf("Odyssey chapter drop source mismatch")
	}
	if len(d.Drops) != 7 {
		return nil, fmt.Errorf("Odyssey chapter drop must cover 7 chapters, got %d", len(d.Drops))
	}
	d.byDungeon = map[uint32]ChapterDropLine{}
	for i, line := range d.Drops {
		if line.Chapter != uint8(i+1) || line.Final == 0 {
			return nil, fmt.Errorf("invalid Odyssey chapter drop row %d", i)
		}
		if line.Enabled && (line.Template == 0 || line.Rate == 0 || line.Rate > 10000) {
			return nil, fmt.Errorf("enabled Odyssey chapter drop %d needs a template and a rate in 1..10000", line.Chapter)
		}
		if _, dup := d.byDungeon[line.Final]; dup {
			return nil, fmt.Errorf("Odyssey chapter drop duplicates final %d", line.Final)
		}
		d.byDungeon[line.Final] = line
	}
	return &d, nil
}

// ValidateBoxes refuses an enabled line whose template is not really an
// equipment selection box. A disabled line is not checked: it may legitimately
// carry template 0.
func (d *OdysseyChapterDrop) ValidateBoxes(boxes *catalog.SelectionBoxes) error {
	if d == nil {
		return nil
	}
	for _, line := range d.Drops {
		if !line.Enabled {
			continue
		}
		if boxes == nil {
			return fmt.Errorf("Odyssey chapter drop enabled without a selection box catalog")
		}
		if _, ok := boxes.ByTemplate(line.Template); !ok {
			return fmt.Errorf("Odyssey chapter drop %d template %d is not a selection box", line.Chapter, line.Template)
		}
	}
	return nil
}

// Enabled reports whether any chapter drop is turned on.
func (d *OdysseyChapterDrop) Enabled() bool {
	if d == nil {
		return false
	}
	for _, line := range d.Drops {
		if line.Enabled && line.Template != 0 {
			return true
		}
	}
	return false
}

// DropFor returns the drop line for a dungeon that is some chapter's final lord.
func (d *OdysseyChapterDrop) DropFor(dungeon uint32) (ChapterDropLine, bool) {
	if d == nil {
		return ChapterDropLine{}, false
	}
	line, ok := d.byDungeon[dungeon]
	return line, ok
}

// Roll rolls the chapter box for a confirmed final-lord kill.
//
// A disabled (or non-final) dungeon returns no award *and the seed unchanged*,
// so the line is truly inert: turning it on later cannot retroactively shift
// any other drop in the run.
func (d *OdysseyChapterDrop) Roll(seed, dungeon uint32) ([]Award, uint32, error) {
	line, ok := d.DropFor(dungeon)
	if !ok || !line.Enabled || line.Template == 0 || line.Rate == 0 {
		return nil, seed, nil
	}
	if line.Rate > 10000 {
		return nil, seed, fmt.Errorf("invalid Odyssey chapter drop rate")
	}
	rng := RNG{seed}
	roll := rng.Next(10000)
	if roll < line.Rate {
		return []Award{{line.Template, 1}}, rng.Seed, nil
	}
	return nil, rng.Seed, nil
}

// StorageCatalog overlays the chapter box templates onto the pickup catalog so
// the awarded box can actually be granted. The boxes already exist in the item
// index the server supplements from, so this only re-points the item record.
func (d *OdysseyChapterDrop) StorageCatalog(c catalog.LootCatalog) catalog.LootCatalog {
	if d == nil {
		return c
	}
	items := make(map[uint32]catalog.LootItem, len(c.Items)+len(d.Drops))
	for id, v := range c.Items {
		items[id] = v
	}
	for _, line := range d.Drops {
		if line.Template == 0 {
			continue
		}
		if _, ok := items[line.Template]; ok {
			continue
		}
		items[line.Template] = catalog.LootItem{
			ID: line.Template, Kind: "stackable", Grade: 1, Rarity: 2,
			StackableType: "[booster selection]", StackLimit: 1,
		}
	}
	c.Items = items
	return c
}

// BagRules makes sure a chapter box has a slot range to land in.
func (d *OdysseyChapterDrop) BagRules(b inventory.BagRules) inventory.BagRules {
	if d == nil || !d.Enabled() {
		return b
	}
	slots := make(map[string][2]uint16, len(b.Slots)+1)
	for k, v := range b.Slots {
		slots[k] = v
	}
	slots["[booster selection]"] = [2]uint16{65, 120}
	b.Slots = slots
	return b
}

// Stackable reports whether a template is one of the chapter boxes. Scene rows
// carry an amount (not durability) for those.
func (d *OdysseyChapterDrop) Stackable(id uint32) bool {
	if d == nil {
		return false
	}
	for _, line := range d.Drops {
		if line.Template != 0 && line.Template == id {
			return true
		}
	}
	return false
}
