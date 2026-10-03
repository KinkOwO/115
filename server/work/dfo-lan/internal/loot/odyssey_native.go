package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// These probabilities and activation flags are server strategy, independently
// retained from the operator-approved configuration. Source IDs and final
// dungeons for chapter drops are deliberately absent.
type OdysseyChapterDropPolicy struct {
	Chapter uint8  `json:"chapter"`
	Rate    uint32 `json:"rate"`
	Enabled bool   `json:"enabled"`
}

type OdysseyCurrencyPolicy struct {
	Denominator uint32    `json:"denominator"`
	Rates       [4]uint32 `json:"rates_by_rank"`
	Templates   [4]uint32 `json:"templates_by_rank"`
}

func ImportOdysseyChapterDrop(chapters *catalog.OdysseyChapters, index catalog.ItemIndex, policy []OdysseyChapterDropPolicy) (*OdysseyChapterDrop, error) {
	if chapters == nil || chapters.Source.Checksum != catalog.OdysseySource || index.Source.Checksum != catalog.OdysseySource || len(policy) != len(chapters.Chapters) {
		return nil, fmt.Errorf("invalid Odyssey chapter drop source/policy")
	}
	c := OdysseyChapterDrop{Model: OdysseyChapterDropModel, Source: chapters.Source, JournalPath: chapters.JournalPath, DefinitionSHA: chapters.DefinitionSHA}
	for i, chapter := range chapters.Chapters {
		p := policy[i]
		if p.Chapter != chapter.Number || p.Rate > 10000 {
			return nil, fmt.Errorf("invalid Odyssey chapter policy %d", p.Chapter)
		}
		line := ChapterDropLine{Chapter: chapter.Number, Final: chapter.Final, Rate: p.Rate, Enabled: p.Enabled}
		for _, reward := range chapter.Rewards {
			entry, ok := index.Items[reward.Template]
			if !ok {
				return nil, fmt.Errorf("chapter reward %d missing source index", reward.Template)
			}
			if entry.StackableType == "[booster selection]" {
				line.Template = reward.Template
				break
			}
		}
		if chapter.Number <= 6 && line.Template == 0 {
			return nil, fmt.Errorf("chapter %d missing source selection box", chapter.Number)
		}
		c.Drops = append(c.Drops, line)
	}
	return NewOdysseyChapterDrop(c)
}

func ImportOdysseyCurrency(a *pvf.Archive, index catalog.ItemIndex, policy OdysseyCurrencyPolicy) (*OdysseyCurrency, error) {
	if a == nil || a.Snapshot().Checksum != catalog.OdysseySource || index.Source.Checksum != catalog.OdysseySource {
		return nil, fmt.Errorf("Odyssey currency source mismatch")
	}
	c := OdysseyCurrency{Source: a.Snapshot().Checksum, Model: "operator-odyssey-coins-v1", Denominator: policy.Denominator, Rates: policy.Rates, Templates: policy.Templates, Items: map[uint32]catalog.LootItem{}}
	for _, id := range policy.Templates {
		if _, exists := c.Items[id]; exists {
			continue
		}
		entry, ok := index.Items[id]
		if !ok {
			return nil, fmt.Errorf("Odyssey currency %d missing source index", id)
		}
		item, err := catalog.ImportStackableItem(a, entry)
		if err != nil {
			return nil, err
		}
		c.Items[id] = item
	}
	return NewOdysseyCurrency(c)
}
