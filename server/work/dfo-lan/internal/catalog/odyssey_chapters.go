package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
)

// OdysseyChaptersModel is the model tag of odyssey-chapters-release.json, the
// chapter journal exported from
// contents/2026/aradodyssey/etc/aradodysseyjournal.cos by cmd/odysseychapterimport.
const OdysseyChaptersModel = "source-odyssey-chapters-v1"

// OdysseyChaptersJournalSHA pins the journal this catalog was exported from.
// A different build must be re-exported rather than silently accepted: the
// chapter split drives which rewards a player is owed.
const OdysseyChaptersJournalSHA = "d4654fa9a50ddd582077f5f7a0a19835ec4fb6b777f032d1a66be7f288eff67e"

// ChapterReward is one line of a chapter's [reward] block. Count is almost
// always 1; chapter 4 line "10419743 2" and chapter 5 line "10419744 2" are the
// two exceptions in the 115 source.
type ChapterReward struct {
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
}

// OdysseyChapter is one [chapter] block: its dungeons in source order, the
// final one, and the rewards the journal lists for it.
type OdysseyChapter struct {
	Number   uint8           `json:"number"`
	Dungeons []uint32        `json:"dungeons"`
	Final    uint32          `json:"final"`
	Rewards  []ChapterReward `json:"rewards"`
}

// OdysseyChapters is the loaded chapter journal. It answers the two questions
// the reward and drop paths need: which chapter a dungeon belongs to, and
// whether a dungeon is its chapter's final lord.
type OdysseyChapters struct {
	Model          string              `json:"model"`
	Source         pvf.ArchiveSnapshot `json:"source"`
	JournalPath    string              `json:"journal_path"`
	DefinitionSHA  string              `json:"definition_sha256"`
	ChapterCount   int                 `json:"chapter_count"`
	DungeonCount   int                 `json:"dungeon_count"`
	RewardTemplate int                 `json:"reward_templates"`
	Chapters       []OdysseyChapter    `json:"chapters"`

	chapterOf map[uint32]uint8
	byNumber  map[uint8]OdysseyChapter
}

// LoadOdysseyChapters reads and validates the exported chapter journal. It
// refuses a catalog that disagrees with the source shape in any way that would
// change who gets paid: wrong model/source/journal, not seven chapters, not
// fifty dungeons, a duplicated dungeon, a Final that is not the chapter's last
// dungeon, or an empty reward list.
func LoadOdysseyChapters(path string) (*OdysseyChapters, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c OdysseyChapters
	if e = json.Unmarshal(b, &c); e != nil {
		return nil, e
	}
	return NewOdysseyChapters(c)
}

// NewOdysseyChapters shares validation and runtime index construction across native and JSON sources.
func NewOdysseyChapters(c OdysseyChapters) (*OdysseyChapters, error) {
	if c.Model != OdysseyChaptersModel || c.Source.Checksum != OdysseySource || c.DefinitionSHA != OdysseyChaptersJournalSHA {
		return nil, fmt.Errorf("Odyssey chapter source mismatch")
	}
	if len(c.Chapters) != 7 || c.ChapterCount != 7 {
		return nil, fmt.Errorf("Odyssey journal must have 7 chapters, got %d", len(c.Chapters))
	}
	c.chapterOf = map[uint32]uint8{}
	c.byNumber = map[uint8]OdysseyChapter{}
	total := 0
	for i, ch := range c.Chapters {
		if ch.Number != uint8(i+1) || len(ch.Dungeons) == 0 || len(ch.Rewards) == 0 {
			return nil, fmt.Errorf("invalid Odyssey chapter %d", ch.Number)
		}
		if ch.Final != ch.Dungeons[len(ch.Dungeons)-1] {
			return nil, fmt.Errorf("Odyssey chapter %d final is not its last dungeon", ch.Number)
		}
		for _, id := range ch.Dungeons {
			if id == 0 {
				return nil, fmt.Errorf("Odyssey chapter %d has a zero dungeon", ch.Number)
			}
			if prev, dup := c.chapterOf[id]; dup {
				return nil, fmt.Errorf("Odyssey dungeon %d in chapters %d and %d", id, prev, ch.Number)
			}
			c.chapterOf[id] = ch.Number
		}
		for _, r := range ch.Rewards {
			if r.Template == 0 || r.Count == 0 {
				return nil, fmt.Errorf("Odyssey chapter %d has an invalid reward line", ch.Number)
			}
		}
		c.byNumber[ch.Number] = ch
		total += len(ch.Dungeons)
	}
	if total != 50 || c.DungeonCount != 50 {
		return nil, fmt.Errorf("Odyssey journal must have 50 dungeons, got %d", total)
	}
	return &c, nil
}

// ChapterOf reports which chapter a dungeon belongs to.
func (c *OdysseyChapters) ChapterOf(dungeon uint32) (uint8, bool) {
	if c == nil {
		return 0, false
	}
	n, ok := c.chapterOf[dungeon]
	return n, ok
}

// At returns a chapter by number.
func (c *OdysseyChapters) At(number uint8) (OdysseyChapter, bool) {
	if c == nil {
		return OdysseyChapter{}, false
	}
	ch, ok := c.byNumber[number]
	return ch, ok
}

// IsFinal reports whether the dungeon is a chapter's final lord. Both the
// chapter reward and the chapter drop hinge on it.
func (c *OdysseyChapters) IsFinal(dungeon uint32) bool {
	if c == nil {
		return false
	}
	n, ok := c.chapterOf[dungeon]
	if !ok {
		return false
	}
	return c.byNumber[n].Final == dungeon
}
