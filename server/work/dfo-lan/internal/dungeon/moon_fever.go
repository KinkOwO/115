package dungeon

import (
	"fmt"
	"time"
)

// Native147928CA0 computes(score/50)*1000 milliseconds. S1/2276 carries
// SCORE, not milliseconds:1413272D0 calls that conversion on the received u32.
// Cooldown is max(cos minimum5s, active duration+500ms).
func (r *MoonSoloOwner) UseMoonFever(stamp MoonSoloStamp, member uint16, now time.Time) (uint32, MoonProgress, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.validate(stamp, member); e != nil {
		return 0, MoonProgress{}, e
	}
	s := r.session
	if r.phase != moonSoloRunning || member != r.leader || !s.Loaded || s.Completed() ||
		(s.Definition.ID != 100004136 && s.Definition.ID != 100004137) {
		return 0, MoonProgress{}, fmt.Errorf("Moon fever requires current active controller")
	}
	if now.Before(s.MoonFeverCooldown) {
		return 0, MoonProgress{}, fmt.Errorf("Moon fever cooldown active")
	}
	score := min(s.MoonFeverScore, uint32(1000))
	if score < 50 {
		return 0, MoonProgress{}, fmt.Errorf("Moon fever charge insufficient")
	}
	duration := time.Duration(score/50) * time.Second
	s.MoonFeverScore = 0
	s.MoonFeverUntil = now.Add(duration)
	s.MoonFeverCooldown = now.Add(max(5*time.Second, duration+500*time.Millisecond))
	return score, s.MoonProgress(), nil
}
