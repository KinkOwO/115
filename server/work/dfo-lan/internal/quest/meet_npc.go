package quest

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/storage"
	"fmt"
)

// AllowsRemoteNPCInteraction reports whether a [meet npc] quest decides its
// target from the explicit conversation request (CMD33) alone, without the
// target having to stand in the character's current area.
//
// [sub type] 1 marks that form. The native client, for these quests, overrides
// the object it was handed with the quest's own target NPC (14546260e), then
// matches identity and hands the request to the objective/CMD33 chain
// (14546267f -> 145468150). It never asks whether that NPC is in the current
// map, so a server-side positional test refuses requests the client has
// already made: in town40/area2 the raw objective NPC 28 is present, while the
// [alternative npc index] target 100001447 lives only in town139, and the
// three extended-slot quests are exactly this shape.
//
// The positive-NPC restriction is deliberate. The 13 subtype-1 quests whose
// objective NPC is -1 resolve their real target from conditions this package
// does not model yet, so they stay on the positional path and keep being
// refused when we cannot name a target.
//
// Note that this server does not apply the [alternative npc index] rule at
// all: the objective NPC is taken as written. The exemption below is what
// keeps those quests working from either map, so it must not be dropped when
// alternative resolution is added later.
func AllowsRemoteNPCInteraction(d catalog.QuestDefinition) bool {
	if len(d.Pending) != 0 || d.Kind != "[meet npc]" ||
		len(d.ObjectiveCells) != 1 || d.ObjectiveCells[0].Type != 0 {
		return false
	}
	if d.ObjectiveCells[0].Value <= 0 {
		return false
	}
	subtype := cells(d.Script.Cells, "[sub type]")
	return len(subtype) == 1 && subtype[0].Type == 0 && subtype[0].Value == 1
}

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
	tag, e := s.Store.DB.Exec(ctx, `UPDATE character_quests q SET progress=0 FROM characters c WHERE c.id=q.character_id AND c.account_id=$1 AND c.id=$2 AND c.deleted_at IS NULL AND q.quest_id=$3 AND q.status='accepted' AND q.config_version=$4 AND q.progress_model=$5 AND q.progress IN (0,1)`, role.AccountID, role.ID, id, s.Catalog.Source.SaveIdentity(), model)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("NPC quest is not active for this owner")
	}
	return nil
}
