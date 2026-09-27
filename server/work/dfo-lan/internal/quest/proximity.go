package quest

import (
	"context"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"math"
	"os"
	"strconv"
)

// NPCLocator reports where a source NPC stands in the character's current
// area. The world service owns that geometry; this package only compares.
type NPCLocator func(npc uint32) ([2]uint16, bool)

// ProximityRadius is the local acceptance distance, in source map units,
// between the character and the NPC row's own coordinates. The native client
// gates its conversation on its own reach test, which is not recovered; this
// server-side distance is an explicit local policy, not an official value.
const ProximityRadius = 180

// NPCDistanceMultiplier expands server-side NPC objective proximity. Invalid
// values and values below one retain the source/default distances.
func NPCDistanceMultiplier() float64 {
	raw := os.Getenv("DFO_QUEST_NPC_DISTANCE_MULTIPLIER")
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 1 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 1
	}
	return value
}

func nearRadius(a, b uint16, radius int) bool {
	d := int(a) - int(b)
	if d < 0 {
		d = -d
	}
	return d <= radius
}

func nearNPCWithin(npc uint32, at storage.WorldPosition, locate NPCLocator, radius int) bool {
	if npc == 0 || locate == nil {
		return false
	}
	p, ok := locate(npc)
	limit := math.Min(65535, math.Ceil(float64(radius)*NPCDistanceMultiplier()))
	return ok && nearRadius(at.X, p[0], int(limit)) && nearRadius(at.Y, p[1], int(limit))
}

func nearNPC(npc uint32, at storage.WorldPosition, locate NPCLocator) bool {
	return nearNPCWithin(npc, at, locate, ProximityRadius)
}

// Local interpretation of the three-cell reach form: full width and height
// centered on the source NPC. The native client's boundary predicate has not
// yet been recovered; keep both source dimensions instead of a fixed radius.
func nearNPCReach(r NPCReachObjective, at storage.WorldPosition, locate NPCLocator) bool {
	if r.NPC == 0 || locate == nil || r.W <= 0 || r.H <= 0 {
		return false
	}
	p, ok := locate(r.NPC)
	if !ok {
		return false
	}
	dx := int64(at.X) - int64(p[0])
	dy := int64(at.Y) - int64(p[1])
	multiplier := NPCDistanceMultiplier()
	return math.Abs(float64(dx)) <= float64(r.W)*multiplier/2 &&
		math.Abs(float64(dy)) <= float64(r.H)*multiplier/2
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
func (s *Service) ProximityProgress(ctx context.Context, role storage.Character, at storage.WorldPosition, locate, phaseLocate NPCLocator) ([]uint16, error) {
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
		case ReachNPC:
			satisfied = nearNPCReach(en.NPCReach, at, locate)
			// Phase-map NPCs are only usable for accepted range objectives;
			// their source placement is not proof of a visible dialogue NPC.
			if !satisfied {
				satisfied = nearNPCReach(en.NPCReach, at, phaseLocate)
			}
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
