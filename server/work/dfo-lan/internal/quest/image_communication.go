package quest

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/storage"
	"fmt"
)

// These pairs come from etc/imagecommunication.etc in the same PVF snapshot
// used for quests.generated.json. Refuse another catalog version until its
// device resource is checked, since a changed mapping could summon an NPC for
// the wrong quest.
// The first quest is absent from that quest catalog, so it cannot match until
// its quest definition is imported.
const imageCommunicationSourceChecksum = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"

var imageCommunicationTargets = []struct {
	quest uint16
	npc   uint32
}{
	{3734, 2000},
	{3741, 2001},
}

// ImageCommunicationTarget selects only a pending, accepted meet-NPC quest
// whose objective agrees with the native PVF device configuration. Using the
// device does not itself complete the objective; conversation does that later.
func (s *Service) ImageCommunicationTarget(ctx context.Context, role storage.Character) (uint16, uint32, error) {
	if s == nil || s.Store == nil || role.ID == 0 || role.AccountID == 0 {
		return 0, 0, fmt.Errorf("image communication requires an owned character")
	}
	states, err := s.Store.Quests(ctx, role.AccountID, role.ID)
	if err != nil {
		return 0, 0, err
	}
	return imageCommunicationTarget(s.Catalog, states)
}

func imageCommunicationTarget(c catalog.QuestCatalog, states []storage.QuestState) (uint16, uint32, error) {
	if c.Source.Checksum != imageCommunicationSourceChecksum {
		return 0, 0, fmt.Errorf("image communication resource and quest catalog versions differ")
	}
	for _, target := range imageCommunicationTargets {
		d, ok := c.Quests[uint32(target.quest)]
		if !ok || d.Kind != "[meet npc]" || len(d.ObjectiveCells) != 1 ||
			d.ObjectiveCells[0].Type != 0 || d.ObjectiveCells[0].Value != int32(target.npc) {
			continue
		}
		_, model, err := InitialProgress(d)
		if err != nil || model != SingleMeetNPC {
			continue
		}
		for _, state := range states {
			if state.ID == target.quest && state.Status == "accepted" && state.Progress != 0 &&
				state.ConfigVersion == c.Source.Checksum && state.ProgressModel == model {
				return target.quest, target.npc, nil
			}
		}
	}
	return 0, 0, nil
}
