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
const MonsterKillCheckpoint = "monster-kill-checkpoint-client-gated-v1"
const LegionContentClear = "legion-content-clear-client-gated-v1"

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
	px, py := int32(p.X), int32(p.Y)
	if r.X < 0 && p.X >= 0x8000 {
		px = int32(int16(p.X))
	}
	if r.Y < 0 && p.Y >= 0x8000 {
		py = int32(int16(p.Y))
	}
	return p.Town == r.Town && p.Area == r.Area &&
		px >= r.X && px <= r.X+r.W &&
		py >= r.Y && py <= r.Y+r.H
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
	if MonsterKillCheckpointShape(d) {
		return 0, MonsterKillCheckpoint, nil
	}
	if LegionContentClearShape(d) {
		return 0, LegionContentClear, nil
	}
	return 0, "", fmt.Errorf("quest objective type or structure is not implemented: %s", d.Kind)
}

// MonsterKillCheckpointShape validates a `[monster kill checkpoint]` objective
// without interpreting it.
//
// Source shape (all four quests that use it: 23053/23054/23055/23099):
//
//	[int data] 109019626 1 8 109019627 3 5 109019628 1 3 109019629 2 1 109019630 -1 -1
//
// i.e. numeric triples `(key, a, b)` where `key` walks a fixed ascending set of
// parameter ids and the final triple is `(_, -1, -1)`. What those keys mean is
// NOT established: they are bare integers in the source (not `[name]` id
// references), so nothing in the data spells them out, and reading the client's
// reader for this objective is still outstanding
// (analysis/tasks/next74-legion-entry-prereq-chain.md §3.1).
//
// The validation therefore pins the *shape* only: a different cell count, a
// negative key, or a missing terminator stays unimplemented rather than being
// accepted under a model whose semantics we cannot state.
//
// Why progress 0 (see the InitialProgress call site): the checkpoint is a client
// -side condition - the client records the checkpoint during the run and only
// then offers the submit action - so the server cannot verify it yet. This is the
// same compromise already recorded for `[look cinematic]`, and it mints nothing
// beyond the quest's own configured reward. Without it all four quests are
// dropped from the acceptable list and the entire 2026 content line
// (grandconstellationturtlelibrary -> castleoftheapostate ->
// labyrinthofparadox -> apocalypseantienbi) becomes unreachable with no error.
func MonsterKillCheckpointShape(d catalog.QuestDefinition) bool {
	if d.Kind != "[monster kill checkpoint]" || len(d.Pending) != 0 {
		return false
	}
	c := d.ObjectiveCells
	if len(c) < 3 || len(c)%3 != 0 {
		return false
	}
	for i, t := range c {
		if t.Type != 0 {
			return false
		}
		// Each triple starts with a positive parameter id.
		if i%3 == 0 && t.Value <= 0 {
			return false
		}
	}
	// The final triple is the (-1, -1) terminator.
	if c[len(c)-2].Value != -1 || c[len(c)-1].Value != -1 {
		return false
	}
	// Keys ascend: the source lists them in a fixed order.
	for i := 3; i < len(c); i += 3 {
		if c[i].Value <= c[i-3].Value {
			return false
		}
	}
	return true
}

// LegionContentClearShape validates the two guide-quest objectives that name a
// legion content by number:
//
//	[legion content clear]                  `[int data] 5 1`     -> venus_scenario guide
//	[legion content clear with difficulty]  `[int data] 6 1 2 1` -> apocalypse guide
//
// The leading number is the content index, and the run of numbers is the
// objective: content 0/1/2/3 are the 2022-2024 guides, 5 is venus_scenario and
// 6 is apocalypse (the index starts at 0, so 0 must be accepted). The trailing numbers are not decoded (they are plausibly a
// required clear count and a difficulty), so only their presence is checked.
//
// Progress 0 for the same reason as the checkpoint model: the condition is a
// legion run that this build does not yet track to completion, the client only
// offers the submit once the content is cleared, and the alternative - waiting
// for a verified counter - leaves the guide entry (apocalypse = entry 239, see
// the [go guide] cell of 23128) permanently missing from the adventure guide.
func LegionContentClearShape(d catalog.QuestDefinition) bool {
	if len(d.Pending) != 0 {
		return false
	}
	switch d.Kind {
	case "[legion content clear]":
		// (content) or (content, count)
		if len(d.ObjectiveCells) < 2 || len(d.ObjectiveCells) > 3 {
			return false
		}
	case "[legion content clear with difficulty]":
		// (content, a, difficulty, b)
		if len(d.ObjectiveCells) != 4 {
			return false
		}
	default:
		return false
	}
	c := d.ObjectiveCells
	for i, t := range c {
		if t.Type != 0 {
			return false
		}
		// The content index starts at 0 (quest 13714 is content 0,
		// stolenlandispins), so 0 is valid and only a negative value is not.
		if i == 0 && t.Value < 0 {
			return false
		}
	}
	return true
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
