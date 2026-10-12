package legion

import (
	"dfolan/internal/catalog"
	"fmt"
	"math"
	"sort"
	"time"
)

type NativeRunState uint8

const (
	NativeCountdown NativeRunState = iota
	NativeWaiting
	NativeSelecting
	NativeReady
	NativeFighting
	NativePhaseComplete
	NativeComplete
	NativeAbandoned
)

// NativeRun owns one source-defined three-stage run. No protocol handler may
// advance it from a claimed phase or a foreign dungeon alone: Clear requires
// the same dungeon and attempt token that Enter accepted.
type NativeRun struct {
	Source                     catalog.LegionContent
	Mode                       string
	Order                      [3]int
	Phase                      int
	State                      NativeRunState
	Offers                     []catalog.NativeLegionCard
	Selected                   int
	History                    [3]*catalog.NativeLegionCard
	Reselections               int64
	ReadyAt, SelectionDeadline time.Time
	Attempt                    string
	RewardCommitted            bool
	Retries                    int64
	previousSelection          int
	draw                       func(int64) (int64, error)
}

func NewNativeRun(source catalog.LegionContent, mode string, order [3]int, readyAt time.Time, draw func(int64) (int64, error)) (*NativeRun, error) {
	if source.Name != "Dimension Cloister" && source.Name != "DuskyIsland" {
		return nil, fmt.Errorf("unsupported native legion %q", source.Name)
	}
	if source.LastPhase != 2 || source.NativeOperations == nil || source.SelectLimit <= 0 || source.ReselectCount == nil || source.RetryCount == nil || *source.ReselectCount < -1 || *source.RetryCount < -1 || draw == nil {
		return nil, fmt.Errorf("native legion run rules missing")
	}
	seen := map[int]bool{}
	for _, index := range order {
		if index < 0 || index >= len(source.Dungeons) || seen[index] {
			return nil, fmt.Errorf("invalid native route")
		}
		stage := source.Dungeons[index]
		if stage.Dungeon == 0 || stage.Boss == 0 {
			return nil, fmt.Errorf("native route lacks dungeon/boss")
		}
		seen[index] = true
	}
	r := &NativeRun{Source: source, Mode: mode, Order: order, State: NativeCountdown, Selected: -1, ReadyAt: readyAt, draw: draw}
	cards, err := r.drawOffers(0)
	if err != nil {
		return nil, err
	}
	r.Offers = cards
	return r, nil
}

func (r *NativeRun) drawOffers(phase int) ([]catalog.NativeLegionCard, error) {
	indices, ok := r.Source.NativeOperations.Modes[r.Mode]
	if !ok {
		return nil, fmt.Errorf("native operation mode absent")
	}
	poolSet := map[uint32]bool{}
	for _, id := range indices {
		d, ok := r.Source.NativeOperations.Definitions[id]
		if !ok {
			return nil, fmt.Errorf("native mode has dangling operation")
		}
		if d.MemberLimit == "party" && d.Probability > 0 {
			poolSet[d.SelectPool] = true
		}
	}
	var pools, extra []uint32
	for p := range poolSet {
		if p < 3 {
			pools = append(pools, p)
		} else {
			extra = append(extra, p)
		}
	}
	sort.Slice(pools, func(i, j int) bool { return pools[i] < pools[j] })
	sort.Slice(extra, func(i, j int) bool { return extra[i] < extra[j] })
	if len(pools) != 3 {
		return nil, fmt.Errorf("native common card slots changed")
	}
	if r.Source.Name == "Dimension Cloister" {
		if len(extra) != 1 || extra[0] != 3 {
			return nil, fmt.Errorf("Cloister final card pool changed")
		}
		if phase == 2 {
			pools = append(pools, extra[0])
		}
	} else {
		if len(extra) != 3 || extra[0] != 4 || extra[1] != 5 || extra[2] != 6 {
			return nil, fmt.Errorf("Dusky phase card pools changed")
		}
		pools = append(pools, extra[phase])
	}
	var cards []catalog.NativeLegionCard
	for _, pool := range pools {
		var card catalog.NativeLegionCard
		var err error
		if r.Source.Name == "DuskyIsland" {
			card, err = r.Source.NativeOperations.DrawDuskyCard(r.Mode, "party", "normal", pool, r.draw)
		} else {
			card, err = r.Source.NativeOperations.DrawCard(r.Mode, "party", "normal", pool, r.draw)
		}
		if err != nil {
			return nil, err
		}
		if _, err = DuskyOperationOperand(card.Parameter); err != nil {
			return nil, err
		}
		if card.FixedValue != nil {
			if _, err = DuskyOperationOperand(*card.FixedValue); err != nil {
				return nil, err
			}
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func (r *NativeRun) Stage() catalog.LegionDungeon { return r.Source.Dungeons[r.Order[r.Phase]] }

func (r *NativeRun) Ready(now time.Time) bool {
	if r.State != NativeCountdown || now.Before(r.ReadyAt) {
		return false
	}
	r.State = NativeWaiting
	return true
}

func (r *NativeRun) Open(reselect bool, now time.Time) error {
	if r.State == NativeSelecting && now.Before(r.SelectionDeadline) {
		return nil
	}
	if reselect {
		if r.State != NativeReady || r.Reselections >= math.MaxInt16 || (*r.Source.ReselectCount >= 0 && r.Reselections >= *r.Source.ReselectCount) {
			return fmt.Errorf("native reselection unavailable")
		}
		r.Reselections++
	} else if r.State != NativeWaiting {
		return fmt.Errorf("native selection unavailable")
	}
	// Reselect reopens the same frozen offers; it does not reroll probabilities.
	r.previousSelection = r.Selected
	r.Selected = -1
	r.SelectionDeadline = now.Add(time.Duration(r.Source.SelectLimit) * time.Second)
	r.State = NativeSelecting
	return nil
}

func (r *NativeRun) Select(index uint16, now time.Time) error {
	if r.State == NativeReady && r.Selected >= 0 && r.Offers[r.Selected].Index == index {
		return nil
	}
	if r.State != NativeSelecting || !now.Before(r.SelectionDeadline) {
		return fmt.Errorf("native selection window inactive")
	}
	for i, card := range r.Offers {
		if card.Index == index {
			r.Selected = i
			r.SelectionDeadline = time.Time{}
			r.State = NativeReady
			return nil
		}
	}
	return fmt.Errorf("operation not in owned native offers")
}

func (r *NativeRun) Enter(phase int, dungeon uint32, attempt string) error {
	if r.State != NativeReady || r.Selected < 0 || phase != r.Phase || dungeon != r.Stage().Dungeon || attempt == "" {
		return fmt.Errorf("native stage admission mismatch")
	}
	r.Attempt = attempt
	r.State = NativeFighting
	return nil
}

func (r *NativeRun) Clear(dungeon uint32, attempt string, completed bool) error {
	if !completed || attempt == "" || attempt != r.Attempt || dungeon != r.Stage().Dungeon {
		return fmt.Errorf("native clear lacks owned completed attempt")
	}
	if r.State == NativePhaseComplete || r.State == NativeComplete {
		return nil
	}
	if r.State != NativeFighting {
		return fmt.Errorf("native clear outside battle")
	}
	card := r.Offers[r.Selected]
	r.History[r.Phase] = &card
	r.State = NativePhaseComplete
	return nil
}

func (r *NativeRun) Advance() error {
	if r.State != NativePhaseComplete || !r.RewardCommitted {
		return fmt.Errorf("native reward completion before stage clear")
	}
	if r.Phase == 2 {
		r.State = NativeComplete
		return nil
	}
	// Prepare next cards before mutating progress, so source failures are retryable.
	cards, err := r.drawOffers(r.Phase + 1)
	if err != nil {
		return err
	}
	r.Phase++
	r.Offers = cards
	r.Selected = -1
	r.Reselections = 0
	r.Attempt = ""
	r.RewardCommitted = false
	r.Retries = 0
	r.SelectionDeadline = time.Time{}
	r.State = NativeWaiting
	return nil
}

// Settle is called only after the durable reward service has acknowledged the
// same attempt (including an already exhausted source entitlement).
func (r *NativeRun) Settle(attempt string) error {
	if r.State != NativePhaseComplete || attempt == "" || attempt != r.Attempt {
		return fmt.Errorf("native settlement does not own completed attempt")
	}
	r.RewardCommitted = true
	return nil
}

func (r *NativeRun) Expire(now time.Time) bool {
	if r.State != NativeSelecting || now.Before(r.SelectionDeadline) {
		return false
	}
	return r.CloseSelection()
}

func (r *NativeRun) CloseSelection() bool {
	if r.State != NativeSelecting {
		return false
	}
	r.SelectionDeadline = time.Time{}
	r.Selected = r.previousSelection
	r.State = NativeWaiting
	if r.Selected >= 0 {
		r.State = NativeReady
	}
	return true
}

func (r *NativeRun) Retry(attempt string) error {
	if r.State != NativeFighting || attempt == "" || attempt != r.Attempt || r.Retries == math.MaxInt64 || r.Source.RetryCount == nil || (*r.Source.RetryCount >= 0 && r.Retries >= *r.Source.RetryCount) {
		return fmt.Errorf("native retry unavailable")
	}
	r.Retries++
	r.Attempt = ""
	r.State = NativeReady
	return nil
}

func (r *NativeRun) Abandon() {
	r.State = NativeAbandoned
	r.SelectionDeadline = time.Time{}
	// Retain the last attempt identity until its owned dungeon leaves. Clearing
	// it here would make the following C72 impossible to authorize.
}
