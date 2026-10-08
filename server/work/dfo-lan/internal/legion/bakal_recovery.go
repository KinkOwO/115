package legion

import (
	"fmt"
	"time"
)

type BakalReturnCause byte

const (
	BakalReturnVoluntary BakalReturnCause = iota + 1
	BakalReturnDeath
	BakalReturnNoPenalty
)

func (o *BakalOpening) applyRecovery(now time.Time, cause BakalReturnCause) error {
	if cause == BakalReturnNoPenalty {
		return nil
	}
	if cause != BakalReturnDeath && cause != BakalReturnVoluntary {
		return fmt.Errorf("unknown Bakal return cause")
	}
	// Fixtures without the native table retain their old structural behavior.
	if len(o.rules.RevivalTimes) == 0 {
		return nil
	}
	i := o.penalizedReturns
	if i >= len(o.rules.RevivalTimes) {
		i = len(o.rules.RevivalTimes) - 1
	}
	seconds := o.rules.RevivalTimes[i]
	if seconds < 0 {
		return fmt.Errorf("negative source revival time")
	}
	// The native deadline is Unix seconds; align the server gate with that
	// same second so the UI cannot finish before the server permits entry.
	until := time.Unix(now.Unix()+int64(seconds), 0)
	if until.Unix() < 0 || until.Unix() > int64(^uint32(0)) {
		return fmt.Errorf("native recovery timestamp out of range")
	}
	o.penalizedReturns++
	o.recoveryUntil = until
	return nil
}
func (o *BakalOpening) Recovering(now time.Time) bool {
	return !o.recoveryUntil.IsZero() && now.Before(o.recoveryUntil)
}
func (o *BakalOpening) RecoveryUntil() time.Time { return o.recoveryUntil }
