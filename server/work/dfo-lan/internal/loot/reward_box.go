package loot

import "fmt"

// RewardBoxCandidate is one entry of a wrapper's pool: the template it pays, the
// weight it is drawn with in that pool, and how many copies one draw hands over.
type RewardBoxCandidate struct {
	Template uint32
	Weight   uint32
	Count    uint32
}

// RewardBoxPool is one draw group of a wrapper. Draws is the source's
// [draw count]: the pool is rolled that many times, so one open of a wrapper is
// not necessarily one prize.
type RewardBoxPool struct {
	Draws      uint32
	Candidates []RewardBoxCandidate
}

// RewardBox is what one open of a wrapper pays.
type RewardBox struct {
	Pools []RewardBoxPool
}

// RewardBoxSource resolves a wrapper into what one open of it pays.
//
// The abyss tables never hand the player a finished prize: every reward entry is
// a [booster] wrapper, and what the player is owed is what is inside it.
type RewardBoxSource interface {
	// RewardBox returns the pools of a wrapper. ok is false for a template that
	// is not a wrapper this build can open.
	RewardBox(template uint32) (RewardBox, bool)
	// Item reports whether a template names something the server can actually
	// hand over, gear included. A pool entry the catalog does not know is the
	// source's empty face - the CTPs spend a reserved id on it - and it must be
	// paid as nothing rather than invented into an item.
	Item(template uint32) bool
	// Container reports whether a template is a box by the source's own
	// metadata, even when this build cannot open it. Such an entry must not
	// reach the ground either: an unopenable box is the same defect under a
	// different name, and the player cannot use it.
	Container(template uint32) bool
}

// rewardBoxMaxDepth guards against a source that nests boxes cyclically. The
// shipped abyss tables nest three levels deep at most.
const rewardBoxMaxDepth = 8

// OpenRewardBoxes replaces every wrapper in awards with what opening it pays,
// repeating until nothing left is a wrapper, and returns the seed advanced by
// those draws plus the entries that could not be resolved.
//
// The depth is not a knob, it is the source's shape. The abyss reward entries
// are wrappers down to real items, and a wrapper that reaches the bag cannot be
// used at all - the reported symptom was a clear that dropped only boxes, none
// of which would open. The source says the same thing about itself: the prize
// descriptions read "not actually dispensed as a gift box, dispensed in the
// opened state". So every wrapper is taken apart here, and what reaches the
// ground is the equipment, the star-stone, the oath prize or the material.
//
// Two things are deliberately *not* paid, and both come back in the skipped
// list so they stay visible: an entry the item catalog does not know (the
// source's empty face), and a box this build cannot open - a selection box that
// needs a choice the server cannot make. Paying either would put something
// useless on the ground, which is exactly the defect being fixed.
//
// The draws mirror the open path deliberately, down to the conventions
// ([draw count] of zero meaning once, the last candidate kept when the weights
// fail to cover the roll): opening the wrapper by hand and letting the drop
// open it must pay the same distribution.
func OpenRewardBoxes(seed uint32, src RewardBoxSource, awards []Award) ([]Award, uint32, []string) {
	if src == nil || len(awards) == 0 {
		return awards, seed, nil
	}
	rng := RNG{seed}
	var skipped []string
	products := make([]Award, 0, len(awards))
	level := awards
	for depth := 0; len(level) > 0; depth++ {
		if depth > rewardBoxMaxDepth {
			skipped = append(skipped, "attunement_reward_nesting_too_deep")
			break
		}
		next := make([]Award, 0, len(level))
		for _, a := range level {
			// One classification point: openable, unopenable box, empty face,
			// or a product. Deciding this only here keeps a wrapper that is not
			// yet a known item from being mistaken for an empty slot on the way
			// down - it has to be opened, not dropped.
			if box, ok := src.RewardBox(a.Template); ok {
				// 一份包装只开一次，所以「×N 份包装」必须开 N 次。忽略 a.Amount 会让
				// 它静默只发一份产物：小深渊 maze 1 的固定表发 10415192×2，就是这样
				// 变成一份的（连名字都从结晶换成了它的产物）。
				units := a.Amount
				if units == 0 {
					units = 1
				}
				for u := uint32(0); u < units; u++ {
					for _, pool := range box.Pools {
						for i := uint32(0); i < pool.draws(); i++ {
							c, ok := pool.pick(&rng)
							if !ok {
								continue
							}
							amount := c.Count
							if amount == 0 {
								amount = 1
							}
							next = append(next, Award{Template: c.Template, Amount: amount})
						}
					}
				}
				continue
			}
			if src.Container(a.Template) {
				skipped = append(skipped, fmt.Sprintf("attunement_unopenable_box_%d", a.Template))
				continue
			}
			if !src.Item(a.Template) {
				skipped = append(skipped, fmt.Sprintf("attunement_empty_prize_%d", a.Template))
				continue
			}
			products = append(products, a)
		}
		level = next
	}
	return products, rng.Seed, skipped
}

// draws is the number of rolls one open takes from this pool. A source that
// declares no [draw count] means one, matching the open path.
func (p RewardBoxPool) draws() uint32 {
	if p.Draws == 0 {
		return 1
	}
	return p.Draws
}

// pick draws one candidate with the pool's own weights. The sum is built the way
// the walk consumes it (a zero weight counts once), so no candidate is
// unreachable and the roll cannot fall past the end.
func (p RewardBoxPool) pick(rng *RNG) (RewardBoxCandidate, bool) {
	if len(p.Candidates) == 0 {
		return RewardBoxCandidate{}, false
	}
	if len(p.Candidates) == 1 {
		return p.Candidates[0], true
	}
	var total uint32
	for _, c := range p.Candidates {
		total += poolWeight(c)
	}
	if total == 0 {
		return p.Candidates[len(p.Candidates)-1], true
	}
	roll := rng.Next(total)
	var acc uint32
	for _, c := range p.Candidates {
		acc += poolWeight(c)
		if roll < acc {
			return c, true
		}
	}
	return p.Candidates[len(p.Candidates)-1], true
}

// poolWeight is the weight a draw actually uses. A zero weight becomes one so
// the entry stays reachable, which is what the open path does with it.
func poolWeight(c RewardBoxCandidate) uint32 {
	if c.Weight == 0 {
		return 1
	}
	return c.Weight
}
