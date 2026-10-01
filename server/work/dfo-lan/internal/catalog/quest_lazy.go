package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// These sections are the complete runtime availability/objective projection;
// presentation and opaque source blocks remain accessible through Definition.
var questRuntimeTags = map[string]bool{
	"[visible npc]": true, "[npc visivility not revert]": true,
	"[difficulty]": true, "[experience increasing point]": true, "[named monster]": true,
	"[type]": true, "[level]": true, "[job]": true, "[pre required quest]": true, "[collision quest]": true, "[int data]": true, "[tournament dungeon]": true,
	"[grade]": true, "[grow type]": true, "[target character]": true, "[sub type]": true, "[dungeon info]": true, "[monster reward item]": true, "[reward type]": true, "[reward int data]": true, "[reward select int data]": true, "[reward selection int data]": true, "[complete npc index]": true, "[npc]": true, "[NPC]": true, "[ignore level]": true, "[basic level penalty]": true, "[condition]": true, "[npc visibility]": true, "[/npc visibility]": true, "[visibility]": true,
}

func (c *QuestCatalog) EnableRuntimeDetails(a *pvf.Archive) error {
	if a == nil || c.Source.Checksum != a.Snapshot().Checksum {
		return fmt.Errorf("quest detail source mismatch")
	}
	if c.details != nil {
		return nil
	}
	refs := map[uint32]ScriptRecord{}
	for id, q := range c.Quests {
		if q.Script.Path != "" && len(q.Script.SHA256) == 64 {
			refs[id] = q.Script
		}
	}
	d, err := NewScriptDetails(a, refs, func(_ uint32, s ScriptRecord) ScriptRecord { return s }, ScriptBytes)
	if err != nil {
		return err
	}
	for id, q := range c.Quests {
		var cells []pvf.Token
		keep := false
		block := ""
		for _, t := range q.Script.Cells {
			if block == "" && t.Type == 3 && (t.Text == "[move to dungeon]" || t.Text == "[warp map condition]" || t.Text == "[npc visibility]" || t.Text == "[go guide]") {
				block = t.Text
			}
			if block != "" {
				cells = append(cells, t)
				if t.Type == 3 && t.Text == "[/"+block[1:] {
					block = ""
				}
				continue
			}
			if t.Type == 3 {
				keep = questRuntimeTags[t.Text]
			}
			if keep {
				cells = append(cells, t)
			}
		}
		q.Script.Cells = cells
		q.ObjectiveCells = append([]pvf.Token(nil), q.ObjectiveCells...)
		q.RewardCells = append([]pvf.Token(nil), q.RewardCells...)
		c.Quests[id] = q
	}
	c.details = d
	return nil
}
func (c QuestCatalog) Definition(id uint32) (QuestDefinition, error) {
	q, ok := c.Quests[id]
	if !ok {
		return q, fmt.Errorf("quest absent from source index: %d", id)
	}
	if c.details == nil || q.Script.Path == "" || len(q.Script.SHA256) != 64 {
		return q, nil
	}
	s, err := c.details.Get(id)
	if err != nil {
		return QuestDefinition{}, err
	}
	q.Script = s
	return q, nil
}
func (c QuestCatalog) Close() error {
	if c.details == nil {
		return nil
	}
	return c.details.Close()
}
