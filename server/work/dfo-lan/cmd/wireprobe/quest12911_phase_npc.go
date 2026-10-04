package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/world"
)

// A Lull has no quest-local NPC visibility rule. Native CMD33 for 12911 was
// observed in Chest Town 80/0, where its target is placed by the destroyed
// Chest Town phase map rather than the base map. Keep this authorization
// confined to that observed quest/area; it does not infer active town phases.
// MeetNPC still checks the accepted quest, account, model and source version.
func allowsALullPhaseNPCInteraction(service *world.Service, id uint16, npc uint32, at database.WorldPosition, d catalog.QuestDefinition, quests catalog.QuestCatalog) bool {
	if service == nil || id != 12911 || d.ID != uint32(id) || npc != 100000670 || at.Town != 80 || at.Area != 0 ||
		len(quests.Source.Checksum) != 64 || service.Catalog.Source.Checksum != quests.Source.Checksum ||
		len(d.Pending) != 0 || d.Kind != "[meet npc]" || len(d.ObjectiveCells) != 1 ||
		d.ObjectiveCells[0].Type != 0 || d.ObjectiveCells[0].Value != int32(npc) || !questCompletionNPCMatches(d, npc) {
		return false
	}
	for i, c := range d.Script.Cells {
		if c.Type != 3 || c.Text != "[sub type]" {
			continue
		}
		if i+1 >= len(d.Script.Cells) || d.Script.Cells[i+1].Type != 0 || d.Script.Cells[i+1].Value != -1 ||
			(i+2 < len(d.Script.Cells) && d.Script.Cells[i+2].Type != 3) {
			return false
		}
		_, found := service.PhaseNPCPosition(at, npc)
		return found
	}
	return false
}
