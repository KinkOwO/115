// Package npcpresence projects the native NPC state used by the current client.
// It does not authorize quests, synthesize packets, or infer missing state from
// an NPC's placement in some other phase.
package npcpresence

import (
	"fmt"
	"sort"
)

// Truth distinguishes missing evidence from an observed false value.
type Truth uint8

const (
	Unknown Truth = iota
	False
	True
)

func (t Truth) MarshalJSON() ([]byte, error) {
	switch t {
	case Unknown:
		return []byte("null"), nil
	case False:
		return []byte("false"), nil
	case True:
		return []byte("true"), nil
	default:
		return nil, fmt.Errorf("invalid NPC evidence truth value")
	}
}

type Visibility struct {
	LogicalShow   Truth `json:"logical_show"`
	EntityVisible Truth `json:"entity_visible"`
	ShowOverride  Truth `json:"show_override"`
	Protected     Truth `json:"protected"`
}

// VisibilityReplay follows explicit native operations, not whole CMD handlers.
// Even a known visibility here does not prove an NPC instance exists in the
// currently selected map. Callers must independently resolve its placement.
type VisibilityReplay struct {
	logical        map[uint32]Truth
	entity         map[uint32]Truth
	overrides      map[uint32]map[uint32]struct{}
	overridesKnown bool
	batch          []Effect
	protected      map[uint32]Truth
	protectedKnown bool
	batchUncertain map[uint32]bool
}

// NewVisibilityReplay starts without a cache snapshot. An omitted snapshot is
// not an empty override table, including after a successful CMD4 response.
func NewVisibilityReplay() *VisibilityReplay {
	return &VisibilityReplay{
		logical: make(map[uint32]Truth), entity: make(map[uint32]Truth),
		overrides: make(map[uint32]map[uint32]struct{}),
		protected: make(map[uint32]Truth), batchUncertain: make(map[uint32]bool),
	}
}

// NewConstructedVisibilityReplay is valid at the verified TownPhaseManager
// construction/reset point (146CF1CC0/146CFA2B0). It establishes an empty
// override table, but does not invent an entity-manager visibility baseline.
// CMD4 clears phase conditions with different helpers, not this entire state.
func NewConstructedVisibilityReplay() *VisibilityReplay {
	r := NewVisibilityReplay()
	r.overridesKnown = true
	r.protectedKnown = true
	return r
}

func (r *VisibilityReplay) Snapshot(npc uint32) Visibility {
	override := Unknown
	if len(r.overrides[npc]) != 0 {
		override = True
	} else if r.overridesKnown {
		override = False
	}
	protected := r.protected[npc]
	if protected == Unknown && r.protectedKnown {
		protected = False
	}
	return Visibility{r.logical[npc], r.entity[npc], override, protected}
}

// ClearProtection is NOTI342's146CFA960, not a visibility/override reset.
func (r *VisibilityReplay) ClearProtection() {
	r.protected = make(map[uint32]Truth)
	r.protectedKnown = true
}

// VisibilityBlock is the confirmed 40-byte source block. Conditions are
// accept0, clear1, clearable2 and clearing3; the caller must prove which
// condition executes. This method does not turn completed membership into a
// complete native handler replay or interpret clearing's consumer.
type VisibilityBlock struct {
	Condition             int32
	Show, Protect, Revert bool
	NPCs                  []uint32
}

// ApplyCondition follows144F3B900's NPC-state operations in source order.
// The initial guarded effect, temporary unprotecting show, and final effect
// are distinct records: deduplicating them changes first-winner tie behavior.
// NPC-service/UI side effects are outside this visibility-state projection.
func (r *VisibilityReplay) ApplyCondition(quest uint32, blocks []VisibilityBlock, condition int32, apply bool) {
	for _, block := range blocks {
		if block.Condition != condition || !apply && !block.Revert {
			continue
		}
		for _, npc := range block.NPCs {
			show := block.Show
			if apply {
				switch r.Snapshot(npc).Protected {
				case False:
					r.applyAndAppend(npc, quest, show)
				case Unknown:
					r.batchUncertain[npc] = true
				}
				if block.Protect {
					r.protected[npc] = True //146CF6930
				} else {
					r.applyAndAppend(npc, quest, true)
					r.protected[npc] = False //146D0F120
				}
			} else {
				show = !show
			}
			r.applyAndAppend(npc, quest, show)
		}
	}
}

func (r *VisibilityReplay) applyAndAppend(npc, quest uint32, show bool) {
	if show {
		r.RequestShow(npc)
	} else {
		r.RequestHide(npc)
	}
	r.AppendEffect(Effect{npc, quest, show})
}

// RequestShow is 146D14000: logical show=1 and remove hidden-ID sets.
func (r *VisibilityReplay) RequestShow(npc uint32) {
	r.logical[npc], r.entity[npc] = True, True
}

// RequestHide is 146D04EF0: logical show=0, but a nonempty quest-show
// override vector keeps the entity visible. Source hiddenNpc loading calls
// this function; the source [show] consumer calls RequestShow instead.
func (r *VisibilityReplay) RequestHide(npc uint32) {
	r.logical[npc] = False
	r.entity[npc] = r.Snapshot(npc).ShowOverride
}

// ShowEntity is 146D14060. It preserves the requested logical state.
func (r *VisibilityReplay) ShowEntity(npc uint32) { r.entity[npc] = True }

// AddShowOverride is 146CF6650. Registration itself does not show an entity;
// 144F394FD subsequently calls 146D14060 as a separate native operation.
func (r *VisibilityReplay) AddShowOverride(npc, quest uint32) {
	if r.overrides[npc] == nil {
		r.overrides[npc] = make(map[uint32]struct{})
	}
	r.overrides[npc][quest] = struct{}{}
}

// RemoveShowOverride is 146CFC070. Other quests retain their overrides. When
// the last entry is removed, native code reapplies an existing logical state;
// an absent logical record does not cause an unconditional show.
func (r *VisibilityReplay) RemoveShowOverride(npc, quest uint32) {
	ids := r.overrides[npc]
	if ids == nil && r.overridesKnown {
		return
	}
	delete(ids, quest)
	if len(ids) != 0 {
		return
	}
	delete(r.overrides, npc)
	if !r.overridesKnown {
		// Unobserved other quest IDs could still keep an override active.
		r.entity[npc] = Unknown
		return
	}
	switch r.logical[npc] {
	case True:
		r.RequestShow(npc)
	case False:
		r.RequestHide(npc)
	}
}

// Forget records a gap in operation coverage. Neither a later completed-quest
// snapshot nor a source placement repairs an unknown visibility history.
func (r *VisibilityReplay) Forget() { *r = *NewVisibilityReplay() }

type Effect struct {
	NPC, Quest uint32
	Show       bool
}

type RankStatus uint8

const (
	RankUnknown    RankStatus = iota
	RankIneligible            // verified missing metadata, unit and fallback, etc.
	RankKnown
)

// Rank is signed (unit parent key, native sequence), not numeric quest order.
// Fallback selection and metadata binding must be resolved by source evidence
// before assigning RankKnown. Missing catalog data remains RankUnknown.
type Rank struct {
	Status           RankStatus
	Parent, Sequence int32
}

func (r *VisibilityReplay) AppendEffect(effect Effect) { r.batch = append(r.batch, effect) }

// ResolveBatch follows 146D00BD0: strict lexicographic greater-than from
// (0,0), first inserted effect wins ties, and the entire batch is consumed.
// Unknown rank evidence does not silently skip a potentially winning effect.
func (r *VisibilityReplay) ResolveBatch(ranks map[uint32]Rank) map[uint32]Effect {
	byNPC := make(map[uint32][]Effect)
	for _, effect := range r.batch {
		byNPC[effect.NPC] = append(byNPC[effect.NPC], effect)
	}
	r.batch = nil
	npcs := make([]uint32, 0, len(byNPC))
	for npc := range byNPC {
		npcs = append(npcs, npc)
	}
	sort.Slice(npcs, func(i, j int) bool { return npcs[i] < npcs[j] })
	winners := make(map[uint32]Effect)
	for _, npc := range npcs {
		best := Rank{}
		var winner Effect
		found, unresolved := false, r.batchUncertain[npc]
		for _, effect := range byNPC[npc] {
			rank := ranks[effect.Quest]
			switch rank.Status {
			case RankIneligible:
				continue
			case RankKnown:
				if rank.Parent > best.Parent || rank.Parent == best.Parent && rank.Sequence > best.Sequence {
					best, winner, found = rank, effect, true
				}
			default:
				unresolved = true
			}
		}
		if unresolved {
			r.logical[npc], r.entity[npc] = Unknown, Unknown
			continue
		}
		if found && winner.Quest != 0 { // native HIDWORD(selected record) sentinel
			winners[npc] = winner
			if winner.Show {
				r.RequestShow(npc)
			} else {
				r.RequestHide(npc)
			}
		}
	}
	r.batchUncertain = make(map[uint32]bool)
	return winners
}
