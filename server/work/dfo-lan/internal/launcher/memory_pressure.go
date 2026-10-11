package launcher

import "time"

const systemLowMemoryBytes uint64 = 2 << 30

// Available physical RAM is a host-wide observation, not evidence that this
// process exceeded its budget. Killing a healthy gateway to reclaim unrelated
// memory disconnects the local client. Warn/recover on physical pressure only;
// the private commit guard and Windows Job hard limit remain enforced.
type memoryPressureGuard struct {
	lowSince time.Time
}

type memoryPressureDecision struct {
	event       string
	stopTrigger string
	lowFor      time.Duration
}

func (g *memoryPressureGuard) observe(now time.Time, private, available, budget uint64) memoryPressureDecision {
	var decision memoryPressureDecision
	if private > budget-budget/16 {
		decision.stopTrigger = "process_commit_limit"
		return decision
	}
	if available >= systemLowMemoryBytes {
		if !g.lowSince.IsZero() {
			decision.event = "recovered"
			decision.lowFor = now.Sub(g.lowSince)
		}
		g.lowSince = time.Time{}
		return decision
	}
	if g.lowSince.IsZero() {
		g.lowSince = now
		decision.event = "warning"
	}
	decision.lowFor = now.Sub(g.lowSince)
	return decision
}
