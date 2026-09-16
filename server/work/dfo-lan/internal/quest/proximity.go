package quest

import (
	"context"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
)

// NPCLocator reports where a source NPC stands in the character's current
// area. The world service owns that geometry; this package only compares.
type NPCLocator func(npc uint32) ([2]uint16, bool)

// ProximityRadius is the local acceptance distance, in source map units,
// between the character and the NPC row's own coordinates. The native client
// gates its conversation on its own reach test, which is not recovered; this
// server-side distance is an explicit local policy, not an official value.
const ProximityRadius = 180

func near(a, b uint16) bool {
	d := int(a) - int(b)
	if d < 0 {
		d = -d
	}
	return d <= ProximityRadius
}

func nearNPC(npc uint32, at storage.WorldPosition, locate NPCLocator) bool {
	if npc == 0 || locate == nil {
		return false
	}
	p, ok := locate(npc)
	return ok && near(at.X, p[0]) && near(at.Y, p[1])
}

// holds reports whether the bag already carries every required item. Quest
// items are counted where they actually sit; nothing is consumed here.
func holds(b inventory.Bag, need []ItemNeed) bool {
	if len(need) == 0 {
		return false
	}
	for _, n := range need {
		var have uint64
		for _, i := range b.Items {
			if i.Template == n.Template {
				have += uint64(i.Amount)
			}
		}
		for _, i := range b.Equipment {
			if i.Template == n.Template {
				have++
			}
		}
		for _, i := range b.Worn {
			if i.Template == n.Template {
				have++
			}
		}
		if have < uint64(n.Amount) {
			return false
		}
	}
	return true
}

// ProximityProgress settles the accepted objectives that are decided by where
// the character now stands: reaching a source rectangle, and standing at the
// NPC a quest names. The client's own conversation request (CMD33) remains
// the primary path for [meet npc]; this walk is what keeps a chain moving
// when that request is not observed, and it never invents a completion — the
// character has to actually be at the source coordinates.
func (s *Service) ProximityProgress(ctx context.Context, role storage.Character, at storage.WorldPosition, locate NPCLocator) ([]uint16, error) {
	states, e := s.Store.Quests(ctx, role.AccountID, role.ID)
	if e != nil {
		return nil, e
	}
	var bag inventory.Bag
	x := s.Index()
	var advanced []uint16
	for _, q := range states {
		if q.Status != "accepted" || q.Progress == 0 || q.ConfigVersion != x.Source {
			continue
		}
		en := x.Entries[uint32(q.ID)]
		if en == nil || !en.Implemented || q.ProgressModel != en.Model {
			continue
		}
		satisfied := false
		switch en.Model {
		case SingleMeetNPC:
			satisfied = nearNPC(en.NPC, at, locate)
		case SingleReachRange:
			satisfied = en.Range.Contains(at)
		case SeekAndMeetNPC:
			if bag.Version == "" {
				if bag, e = inventory.ReadBag(role.State); e != nil {
					return advanced, e
				}
			}
			satisfied = holds(bag, en.Seek.Items) && nearNPC(en.NPC, at, locate)
		}
		if !satisfied {
			continue
		}
		applied, e := s.Store.CompleteQuestObjective(ctx, role.AccountID, role.ID, q.ID, x.Source, en.Model)
		if e != nil {
			return advanced, e
		}
		if applied {
			advanced = append(advanced, q.ID)
		}
	}
	return advanced, nil
}
