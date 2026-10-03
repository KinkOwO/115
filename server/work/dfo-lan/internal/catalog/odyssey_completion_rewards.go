package catalog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// This external reward mapping is explicitly maintained by the operator.
// Chapter membership, item attributes and box contents still come from PVF.
type OdysseyCompletionRewards struct {
	Version  int                        `json:"version"`
	Chapters []OdysseyCompletionChapter `json:"chapters"`
	Honor    OdysseyCompletionItem      `json:"honor"`
}

type OdysseyCompletionChapter struct {
	Number  uint8                   `json:"number"`
	Rewards []OdysseyCompletionItem `json:"rewards"`
}

type OdysseyCompletionItem struct {
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
	Name     string `json:"name"` // Documentation only; never used for item lookup.
}

func LoadOdysseyCompletionRewards(path string) (*OdysseyCompletionRewards, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	var c OdysseyCompletionRewards
	if err = d.Decode(&c); err != nil {
		return nil, err
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("Odyssey completion rewards have trailing data")
	}
	if c.Version != 1 || len(c.Chapters) != 7 || c.Honor.Template < 2 || c.Honor.Count != 1 {
		return nil, fmt.Errorf("invalid Odyssey completion rewards")
	}
	for i, ch := range c.Chapters {
		if ch.Number != uint8(i+1) || len(ch.Rewards) == 0 || len(ch.Rewards) > 11 {
			return nil, fmt.Errorf("invalid Odyssey completion chapter %d", ch.Number)
		}
		seen := map[uint32]bool{}
		for _, r := range ch.Rewards {
			if r.Template < 2 || r.Count == 0 || r.Count > 1000 || seen[r.Template] {
				return nil, fmt.Errorf("invalid Odyssey completion reward in chapter %d", ch.Number)
			}
			seen[r.Template] = true
		}
	}
	return &c, nil
}

func (c *OdysseyCompletionRewards) At(number uint8) []ChapterReward {
	if c == nil || number == 0 || int(number) > len(c.Chapters) {
		return nil
	}
	var out []ChapterReward
	for _, r := range c.Chapters[number-1].Rewards {
		out = append(out, ChapterReward{Template: r.Template, Count: r.Count})
	}
	return out
}

// ValidateItems requires every configured reward to be an indexed native box.
// Missing IDs fail startup; English labels are deliberately not compared.
func (c *OdysseyCompletionRewards) ValidateItems(index ItemIndex) error {
	if c == nil {
		return fmt.Errorf("Odyssey completion rewards missing")
	}
	items := []OdysseyCompletionItem{c.Honor}
	for _, ch := range c.Chapters {
		items = append(items, ch.Rewards...)
	}
	for _, r := range items {
		item, ok := index.Items[r.Template]
		if !ok || item.Kind != "stackable" || (item.StackableType != "[booster]" && item.StackableType != "[booster selection]") || (item.StackLimit > 0 && r.Count > item.StackLimit) {
			return fmt.Errorf("Odyssey completion reward %d missing or invalid in native item index", r.Template)
		}
	}
	return nil
}
