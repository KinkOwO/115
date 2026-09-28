package legion

import (
	"dfolan/internal/catalog"
	"fmt"
)

// ApocalypseClock is the phase clock the client's own compiled table carries
// (apocalypse.ctp, the [phase info] column). The container stores one
// (phase index, seconds) pair per phase, and all four operation blocks repeat
// the same six pairs, so the clock is a constant of the release build rather
// than a per-operation value.
//
// The server does not need to send this clock: the client reads the same table
// itself. It is loaded so the server can describe and later drive a run without
// hard-coding numbers that live in the source.
type ApocalypseClock struct {
	phases []catalog.ApocalypsePhase
}

// NewApocalypseClock validates and wraps the catalog clock.
func NewApocalypseClock(cat *catalog.ApocalypseCatalog) (*ApocalypseClock, error) {
	if cat == nil {
		return nil, fmt.Errorf("apocalypse catalog is required")
	}
	if len(cat.PhaseClock) == 0 {
		return nil, fmt.Errorf("apocalypse catalog carries no phase clock")
	}
	phases := make([]catalog.ApocalypsePhase, len(cat.PhaseClock))
	copy(phases, cat.PhaseClock)
	return &ApocalypseClock{phases: phases}, nil
}

// Len is the number of phases.
func (c *ApocalypseClock) Len() int { return len(c.phases) }

// Order lists the phase indices in clock order.
func (c *ApocalypseClock) Order() []int64 {
	out := make([]int64, 0, len(c.phases))
	for _, p := range c.phases {
		out = append(out, p.Phase)
	}
	return out
}

// Durations lists the phase durations in clock order.
func (c *ApocalypseClock) Durations() []float64 {
	out := make([]float64, 0, len(c.phases))
	for _, p := range c.phases {
		out = append(out, p.Seconds)
	}
	return out
}

// TotalSeconds is the sum of every phase duration.
func (c *ApocalypseClock) TotalSeconds() float64 {
	var total float64
	for _, p := range c.phases {
		total += p.Seconds
	}
	return total
}

// Seconds returns the duration of one phase.
func (c *ApocalypseClock) Seconds(phase int64) (float64, bool) {
	for _, p := range c.phases {
		if p.Phase == phase {
			return p.Seconds, true
		}
	}
	return 0, false
}

// Next returns the phase after the given one.
func (c *ApocalypseClock) Next(phase int64) (int64, bool) {
	for i, p := range c.phases {
		if p.Phase != phase {
			continue
		}
		if i+1 >= len(c.phases) {
			return 0, false
		}
		return c.phases[i+1].Phase, true
	}
	return 0, false
}

// RunPlan is the compiled table's own description of one apocalypse operation,
// assembled when the player confirms it (CMD2045).
//
// Every field is either a column read or the phase clock. Values whose meaning
// is not yet proven stay positional: nothing here renames a container tuple into
// a guessed gameplay field, and a missing column stays empty rather than
// defaulting to zero, because absence is how the source expresses "not
// configured".
type RunPlan struct {
	Gates       GateRules
	OperationID uint32
	// Row is the record index of the [operation data set] block, which is what
	// the trailer maps the block to.
	Row int
	// PhaseOrder and PhaseSeconds come from the [phase info] clock.
	PhaseOrder   []int64
	PhaseSeconds []float64
	TotalSeconds float64

	// AllowCoinConfigured reports whether this block carries an [allow coin]
	// column at all. Only one of the four release operations does, so absence
	// must not be read as "deny".
	AllowCoinConfigured bool
	AllowCoin           []int64

	GateSchedule    []int64
	GateFlow        []int64
	GateCloseWarn   []catalog.ApocalypseValue
	RecommendFame   *int64
	CardSymbolIndex *int64
	StringData      []string

	// MemberLimitClass is the [member limit] value the block carries, e.g.
	// "party". It stays a string because that is what the source stores.
	MemberLimitClass string

	// RewardLabel and RewardValues keep the container's own order: the label is
	// the bucket name, the values are positional.
	RewardLabel  string
	RewardValues []float64
}

// BuildRunPlan describes one operation. An unknown operation id is an error:
// the client only offers the ids the table declares, so anything else means the
// two sides disagree about the table.
func BuildRunPlan(cat *catalog.ApocalypseCatalog, clock *ApocalypseClock, operationID uint32) (*RunPlan, error) {
	if cat == nil {
		return nil, fmt.Errorf("apocalypse catalog is required")
	}
	if clock == nil {
		return nil, fmt.Errorf("apocalypse clock is required")
	}
	op := cat.Operation(int64(operationID))
	if op == nil {
		ids := make([]int64, 0, len(cat.Operations))
		for _, o := range cat.Operations {
			if o.Index != nil {
				ids = append(ids, *o.Index)
			}
		}
		return nil, fmt.Errorf("operation %d is not declared by the table (declared: %v)", operationID, ids)
	}
	plan := &RunPlan{
		OperationID:         operationID,
		Row:                 op.Row,
		PhaseOrder:          clock.Order(),
		PhaseSeconds:        clock.Durations(),
		TotalSeconds:        clock.TotalSeconds(),
		AllowCoinConfigured: op.AllowsCoin(),
		AllowCoin:           op.AllowCoin,
		GateSchedule:        op.GateSchedule,
		GateFlow:            op.GateFlow,
		GateCloseWarn:       op.GateCloseWarn,
		RecommendFame:       op.RecommendFame,
		CardSymbolIndex:     op.CardSymbolIndex,
		StringData:          op.StringData,
		MemberLimitClass:    op.MemberLimit,
	}
	if len(op.GateSchedule) != 0 || len(op.GateFlow) != 0 {
		var err error
		plan.Gates, err = CompileGateRules(op.GateSchedule, op.GateFlow)
		if err != nil {
			return nil, err
		}
	}
	if op.Reward != nil {
		plan.RewardLabel = op.Reward.Label
		plan.RewardValues = op.Reward.Values
	}
	return plan, nil
}
