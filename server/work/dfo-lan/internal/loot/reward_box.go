package loot

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
// a [booster] wrapper, and what the player is meant to see is what comes out of
// it. That distinction is not cosmetic. Handing the wrapper over moves the open
// from drop time to a deliberate player action, and the two are not
// interchangeable: the wrapper's contents are what the source pays a clear, so
// dropping the wrapper instead of its contents pays the wrong items - the
// reported symptom was a run that dropped only jars, and jars that opened into
// things the dungeon never pays.
type RewardBoxSource interface {
	// RewardBox returns the pools of a wrapper. ok is false for a template that
	// is not a wrapper this build can open, which is the ordinary case for a
	// plain prize and the failure case for a wrapper nobody can resolve.
	RewardBox(template uint32) (RewardBox, bool)
	// Item reports whether a template names something the server can actually
	// hand over, gear included. A pool entry the catalog does not know is the
	// source's empty face - the CTPs spend a reserved id on it - and it must be
	// paid as nothing rather than invented into an item.
	Item(template uint32) bool
}

// RewardBoxDepth is how deep a reward entry is unwrapped, and it is one on
// purpose.
//
// The source wraps an abyss prize once. What reaches the ground is the wrapper's
// contents, which is why the equipment arrives as equipment while the oath prize
// arrives as the random book that picks it and the star-stone arrives inside its
// jar: those wrappers *are* prizes the player is meant to receive and open.
// Unwrapping further would dissolve them into leaves the player never gets to
// choose between, and the run would stop paying the book and the jar at all.
const RewardBoxDepth = 1

// OpenRewardBoxes replaces every wrapper in awards with what one open of it
// pays, and returns the seed advanced by those draws.
//
// Entries that are not wrappers are passed through untouched, so a table that
// already names a plain prize is not disturbed. A nil source passes everything
// through: the caller is expected to have refused to start in that case (see
// AttunementRewards.ValidateBoxes), because paying a wrapper is paying the
// wrong item.
//
// The draws mirror the open path deliberately, down to the conventions
// ([draw count] of zero meaning once, the last candidate kept when the weights
// fail to cover the roll): opening the wrapper by hand and letting the drop
// open it must pay the same distribution, or the same item would be worth two
// different things depending on how it arrived.
func OpenRewardBoxes(seed uint32, src RewardBoxSource, awards []Award) ([]Award, uint32) {
	if src == nil || len(awards) == 0 {
		return awards, seed
	}
	rng := RNG{seed}
	for depth := 0; depth < RewardBoxDepth; depth++ {
		out := make([]Award, 0, len(awards))
		opened := false
		for _, a := range awards {
			box, ok := src.RewardBox(a.Template)
			if !ok {
				out = append(out, a)
				continue
			}
			opened = true
			for _, pool := range box.Pools {
				for i := uint32(0); i < pool.draws(); i++ {
					c, ok := pool.pick(&rng)
					if !ok {
						continue
					}
					if !src.Item(c.Template) {
						continue
					}
					amount := c.Count
					if amount == 0 {
						amount = 1
					}
					out = append(out, Award{Template: c.Template, Amount: amount})
				}
			}
		}
		awards = out
		if !opened {
			break
		}
	}
	return awards, rng.Seed
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
