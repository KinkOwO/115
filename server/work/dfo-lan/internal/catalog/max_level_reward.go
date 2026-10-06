package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// MaxLevelRewardPath names the source table that decides what a character
// receives on reaching the level cap. The block is not Odyssey-specific: every
// character that hits the cap gets it, an Odyssey graduate included.
const MaxLevelRewardPath = "etc/titlebook.etc"

// The mail wording is bound through the source's own hardcoded text tag list,
// so the server sends what the source names instead of an invented sentence.
const (
	maxLevelRewardSection  = "[maxlevel reward]"
	maxLevelRewardTitleTag = "86_levelup_maxlevel_title"
	maxLevelRewardTextTag  = "86_levelup_maxlevel_text"
	hardcodeTextTagPath    = "etc/hardcodetexttag.etc"
)

// MaxLevelReward is the parsed [maxlevel reward] rule: one stackable template
// with its count, plus the mail title and body resolved from the source.
type MaxLevelReward struct {
	Source     string       `json:"source"`
	Definition ScriptRecord `json:"definition"`
	Template   uint32       `json:"template"`
	Count      uint32       `json:"count"`
	Title      string       `json:"title"`
	Text       string       `json:"text"`
}

// ImportMaxLevelReward reads the reward pair and its mail wording from the
// current archive. An unrecognised block shape is refused rather than guessed.
func ImportMaxLevelReward(a *pvf.Archive, index ItemIndex) (*MaxLevelReward, error) {
	if a == nil {
		return nil, fmt.Errorf("max level reward requires PVF")
	}
	definition, err := ReadScript(a, MaxLevelRewardPath)
	if err != nil {
		return nil, err
	}
	cells := sectionCells(definition.Cells, maxLevelRewardSection)
	if len(cells) != 2 || cells[0].Type != 0 || cells[1].Type != 0 {
		return nil, fmt.Errorf("max level reward block must hold exactly one item/count pair, got %d cells", len(cells))
	}
	reward := MaxLevelReward{Source: a.Snapshot().Checksum, Definition: definition, Template: uint32(cells[0].Value), Count: uint32(cells[1].Value)}
	if reward.Template < 2 || reward.Count == 0 {
		return nil, fmt.Errorf("max level reward template %d count %d is unusable", reward.Template, reward.Count)
	}
	entry, ok := index.Items[reward.Template]
	if !ok || entry.Kind != "stackable" {
		return nil, fmt.Errorf("max level reward template %d absent from the stackable source index", reward.Template)
	}
	if reward.Title, err = hardcodeTextTag(a, maxLevelRewardTitleTag); err != nil {
		return nil, err
	}
	if reward.Text, err = hardcodeTextTag(a, maxLevelRewardTextTag); err != nil {
		return nil, err
	}
	return &reward, nil
}

// hardcodeTextTag follows a tag in etc/hardcodetexttag.etc to its localization
// reference and resolves the text the current client table carries.
func hardcodeTextTag(a *pvf.Archive, tag string) (string, error) {
	script, err := ReadScript(a, hardcodeTextTagPath)
	if err != nil {
		return "", err
	}
	for i, c := range script.Cells {
		if c.Type != 6 || c.Text != tag || i+1 >= len(script.Cells) {
			continue
		}
		ref, ok := pvf.ParseLocalizedRef(script.Cells[i+1].Reference)
		if !ok {
			return "", fmt.Errorf("hardcoded text tag %s has no localization reference", tag)
		}
		text, found, err := a.LocalizedText(ref)
		if err != nil {
			return "", err
		}
		if !found || text == "" {
			return "", fmt.Errorf("hardcoded text tag %s absent from the source string tables", tag)
		}
		return text, nil
	}
	return "", fmt.Errorf("hardcoded text tag %s absent from %s", tag, hardcodeTextTagPath)
}
