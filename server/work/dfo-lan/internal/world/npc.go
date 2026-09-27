package world

import (
	"dfolan/internal/catalog"
	"dfolan/internal/storage"
)

func (s *Service) HasNPC(p storage.WorldPosition, id uint32) bool {
	_, ok := s.NPCPosition(p, id)
	return ok
}

// HasPhaseNPC checks NPC rows exported from source [phase] maps of this area.
// Quest-state visibility is checked separately before such a row can authorize
// an interaction; a phase row alone does not mean that phase is active.
func (s *Service) HasPhaseNPC(p storage.WorldPosition, id uint32) bool {
	area, ok := s.Catalog.Areas[catalog.AreaKey(p.Town, p.Area)]
	if !ok || id == 0 {
		return false
	}
	for _, row := range area.PhaseNPCs {
		if row.ID == id {
			return true
		}
	}
	return false
}

// NPCPosition returns where a source NPC stands in this area. A source [NPC]
// row is five cells: identity, facing tag, x, y and a trailing flag — read
// against town38/area0 NPC1 "[left] 1227 164 0" and town40/area0 NPC358
// "[left] 2249 148 0". Only complete, well-formed rows are accepted.
func (s *Service) NPCPosition(p storage.WorldPosition, id uint32) ([2]uint16, bool) {
	a, ok := s.Catalog.Areas[catalog.AreaKey(p.Town, p.Area)]
	if !ok || id == 0 {
		return [2]uint16{}, false
	}
	for _, script := range append([]catalog.ScriptRecord{a.Map}, a.ImportedScripts...) {
		c := script.Cells
		for i := 0; i < len(c); i++ {
			if c[i].Type != 3 || c[i].Text != "[NPC]" {
				continue
			}
			for i++; i < len(c) && c[i].Type != 3; i += 5 {
				if i+4 >= len(c) || c[i].Type != 0 || c[i+1].Type != 6 || c[i+2].Type != 0 || c[i+3].Type != 0 || c[i+4].Type != 0 {
					break
				}
				if c[i].Value > 0 && uint32(c[i].Value) == id {
					x, y := c[i+2].Value, c[i+3].Value
					if x < 0 || y < 0 || x > 65535 || y > 65535 {
						return [2]uint16{}, false
					}
					return [2]uint16{uint16(x), uint16(y)}, true
				}
			}
		}
	}
	return [2]uint16{}, false
}
