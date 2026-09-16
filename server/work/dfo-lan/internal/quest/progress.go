package quest

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"errors"
	"fmt"
)

const SingleClearMap = "single-clear-map-remaining-v1"
const SingleMeetNPC = "single-meet-npc-remaining-v1"
const SingleReachRange = "single-reach-range-remaining-v1"
const SeekAndMeetNPC = "seek-items-and-meet-npc-remaining-v1"
const LookCinematic = "look-cinematic-client-gated-v1"

// RangeObjective is a six-integer [reach the range] objective: a town, an
// area and an axis-aligned rectangle. Origin plus extent is the only reading
// consistent with every current low-level source row (3156 "38 3 445 115 400
// 300", 3160 "40 3 1034 180 100 100", 21094 "40 3 132 160 900 200"); a
// centre-and-half-extent reading puts 3156 at a negative Y. This has not been
// confirmed against the native handler, so the decode stays strict and any
// other shape remains unimplemented rather than guessed.
type RangeObjective struct {
	Town, Area uint32
	X, Y, W, H int32
}

func ReachRange(d catalog.QuestDefinition) (RangeObjective, bool) {
	c := d.ObjectiveCells
	if len(d.Pending) != 0 || d.Kind != "[reach the range]" || len(c) != 6 {
		return RangeObjective{}, false
	}
	for _, t := range c {
		if t.Type != 0 || t.Value < 0 {
			return RangeObjective{}, false
		}
	}
	if c[0].Value == 0 || c[4].Value <= 0 || c[5].Value <= 0 {
		return RangeObjective{}, false
	}
	return RangeObjective{uint32(c[0].Value), uint32(c[1].Value), c[2].Value, c[3].Value, c[4].Value, c[5].Value}, true
}

func (r RangeObjective) Contains(p storage.WorldPosition) bool {
	return p.Town == r.Town && p.Area == r.Area &&
		int32(p.X) >= r.X && int32(p.X) <= r.X+r.W &&
		int32(p.Y) >= r.Y && int32(p.Y) <= r.Y+r.H
}

type ItemNeed struct{ Template, Amount uint32 }

// SeekObjective is an odd-length [seek n meet npc] objective: repeated
// (item, amount) pairs followed by the NPC identity. Source 3173
// "10164787 1 10164789 1 29" collects the two items awarded by its own
// prerequisites 3169 and 3171 and then reports to NPC 29.
type SeekObjective struct {
	Items []ItemNeed
	NPC   uint32
}

func SeekMeet(d catalog.QuestDefinition) (SeekObjective, bool) {
	c := d.ObjectiveCells
	if len(d.Pending) != 0 || d.Kind != "[seek n meet npc]" || len(c) < 3 || len(c)%2 == 0 {
		return SeekObjective{}, false
	}
	for _, t := range c {
		if t.Type != 0 || t.Value <= 0 {
			return SeekObjective{}, false
		}
	}
	var out SeekObjective
	for i := 0; i+2 < len(c); i += 2 {
		out.Items = append(out.Items, ItemNeed{uint32(c[i].Value), uint32(c[i+1].Value)})
	}
	out.NPC = uint32(c[len(c)-1].Value)
	return out, len(out.Items) > 0
}

var ErrObjectiveIncomplete = errors.New("quest still requires a verified map clear")
var ErrRewardPending = errors.New("quest reward application is not implemented")

// Native addAcceptQuest144f38b60 and updateQuest144f69770 treat zero as
// ready to submit. For the supported single map objective, one means pending.
// Broader condition encodings must be recovered before accepting other types.
func InitialProgress(d catalog.QuestDefinition) (uint32, string, error) {
	if len(d.Pending) == 0 && d.Kind == "[meet npc]" && len(d.ObjectiveCells) == 1 && d.ObjectiveCells[0].Type == 0 && d.ObjectiveCells[0].Value > 0 {
		return 1, SingleMeetNPC, nil
	}
	if len(d.Pending) == 0 && d.Kind == "[clear map]" && len(d.ObjectiveCells) == 1 && d.ObjectiveCells[0].Type == 0 && d.ObjectiveCells[0].Value > 0 {
		return 1, SingleClearMap, nil
	}
	if _, ok := ReachRange(d); ok {
		return 1, SingleReachRange, nil
	}
	if _, ok := SeekMeet(d); ok {
		return 1, SeekAndMeetNPC, nil
	}
	// A [look cinematic] objective has no server-verifiable condition: the
	// client plays the cutscene locally and only then lets the player submit.
	// Progress 0 means the server accepts that submit; the single cell is the
	// cinematic id, kept only to validate the source shape. This is the sole
	// completion path a cinematic has, and it mints nothing beyond the quest's
	// own configured reward. Without it the main story stalls at the first
	// cutscene beat (quest 21029, level 22).
	if len(d.Pending) == 0 && d.Kind == "[look cinematic]" && len(d.ObjectiveCells) == 1 && d.ObjectiveCells[0].Type == 0 && d.ObjectiveCells[0].Value > 0 {
		return 0, LookCinematic, nil
	}
	return 0, "", fmt.Errorf("quest objective type or structure is not implemented: %s", d.Kind)
}
func (s *Service) Active(ctx context.Context, role storage.Character) ([]protocol.ActiveQuest, error) {
	states, e := s.Store.Quests(ctx, role.AccountID, role.ID)
	if e != nil {
		return nil, e
	}
	out := []protocol.ActiveQuest{}
	for _, q := range states {
		if q.Status != "accepted" {
			continue
		}
		d, ok := s.Catalog.Quests[uint32(q.ID)]
		if !ok || q.ConfigVersion != s.Catalog.Source.Checksum {
			return nil, fmt.Errorf("quest %d requires source migration", q.ID)
		}
		initial, model, e := InitialProgress(d)
		if e != nil || q.ProgressModel != model || q.Progress > initial {
			return nil, fmt.Errorf("quest %d requires progress migration", q.ID)
		}
		out = append(out, protocol.ActiveQuest{ID: q.ID, Progress: q.Progress})
	}
	return out, nil
}

// A completion request is not evidence of a map clear. No client-provided
// counter or submit option can mint rewards or alter persistent objectives.
func (s *Service) Submit(ctx context.Context, role storage.Character, id uint16) error {
	active, e := s.Active(ctx, role)
	if e != nil {
		return e
	}
	for _, q := range active {
		if q.ID == id {
			if q.Progress != 0 {
				return ErrObjectiveIncomplete
			}
			return ErrRewardPending
		}
	}
	return errors.New("quest is not active for this character")
}
