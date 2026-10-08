package legion

import (
	"fmt"
	"time"
)

func (o *BakalOpening) CheckPhaseCinematic() (uint32, error) {
	if o.stage != BakalOpeningActive || o.secondPhase {
		return 0, fmt.Errorf("phase cinematic outside first stage")
	}
	if o.rules.Stage.HealthPercent <= 0 || o.rules.Stage.UnlockGradeBelow <= 0 {
		return 0, fmt.Errorf("native phase condition unavailable")
	}
	grade := o.script.HPUnlockGrade
	for _, d := range o.rules.Dungeons {
		if d.Type != "" && d.Type != "bakal" && o.cleared[d.Index] {
			grade--
		}
	}
	if int32(grade) >= o.rules.Stage.UnlockGradeBelow {
		return 0, fmt.Errorf("phase cinematic before source HP unlock")
	}
	return uint32(int64(o.script.MonsterMaxHP) * int64(o.rules.Stage.HealthPercent) / 100), nil
}

// Called only for an owned source first-stage actor and the ACT's phase-grid
// request. The first actor is alive at this threshold; CMD39 is not required.
func (o *BakalOpening) AcceptPhaseCinematic(now time.Time) ([]BakalFrame, error) {
	hp, e := o.CheckPhaseCinematic()
	if e != nil {
		return nil, e
	}
	if o.runtime == nil {
		o.firstPhaseDefeated = true
		return nil, o.BeginSecondPhase()
	}
	c := o.eventClone()
	q := []bakalSignal{}
	if e := c.setEventSymbol("[BAKAL HP]", int32(hp), &q); e != nil {
		return nil, e
	}
	if e := c.setEventSymbol("[BAKAL PHASE]", 1, &q); e != nil {
		return nil, e
	}
	for _, signal := range q {
		if e := c.dispatchEvent(signal, now); e != nil {
			return nil, e
		}
	}
	c.firstPhaseDefeated, c.secondPhase = true, true
	delete(c.defeatedLocations, c.location)
	*o = *c
	return o.eventFrames(), nil
}
