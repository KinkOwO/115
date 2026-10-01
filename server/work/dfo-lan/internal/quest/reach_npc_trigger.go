package quest

import (
	"context"
	"dfolan/internal/character"
	"fmt"
)

// ReachNPCFromClient advances a subtype-0 NPC range objective after the world
// handler has matched the native CMD33 and a source-guided temporary NPC.
func (s *Service) ReachNPCFromClient(ctx context.Context, role character.Character, id uint16, npc uint32) (bool, error) {
	d, ok := s.Catalog.Quests[uint32(id)]
	if !ok {
		return false, fmt.Errorf("unknown quest")
	}
	r, valid := ReachNPCObjective(d)
	if !valid || r.NPC != npc {
		return false, fmt.Errorf("NPC does not match range objective")
	}
	return s.Store.CompleteQuestObjective(ctx, role.AccountID, role.ID, id, s.Catalog.Source.SaveIdentity(), ReachNPC)
}
