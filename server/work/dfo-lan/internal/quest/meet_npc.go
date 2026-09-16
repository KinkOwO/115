package quest

import (
	"context"
	"dfolan/internal/storage"
	"fmt"
)

// The world handler supplies a source NPC present in the owned town area.
// A request never chooses its own progress, reward, or arbitrary NPC identity.
func (s *Service) MeetNPC(ctx context.Context, role storage.Character, id uint16, npc uint32) error {
	d, ok := s.Catalog.Quests[uint32(id)]
	if !ok {
		return fmt.Errorf("unknown quest")
	}
	_, model, e := InitialProgress(d)
	if e != nil || model != SingleMeetNPC || uint32(d.ObjectiveCells[0].Value) != npc {
		return fmt.Errorf("NPC does not match quest objective")
	}
	tag, e := s.Store.DB.Exec(ctx, `UPDATE character_quests q SET progress=0 FROM characters c WHERE c.id=q.character_id AND c.account_id=$1 AND c.id=$2 AND c.deleted_at IS NULL AND q.quest_id=$3 AND q.status='accepted' AND q.config_version=$4 AND q.progress_model=$5 AND q.progress IN (0,1)`, role.AccountID, role.ID, id, s.Catalog.Source.Checksum, model)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("NPC quest is not active for this owner")
	}
	return nil
}
