package main

import (
	"errors"
	"fmt"
	"testing"

	"dfolan/internal/loot"
)

// A confirmed death must survive a drop-coverage gap: on 2026-09-26 every death
// report in border-of-attunement was refused for exactly this reason, so the
// kills never registered and the clear could not happen. The other half matters
// just as much - a defect of the drop model must keep failing the request
// instead of quietly turning into "this monster paid nothing".
func TestDropFailurePolicyKeepsConfirmedDeaths(t *testing.T) {
	if e := fatalDropFailure(nil); e != nil {
		t.Fatalf("a successful roll is not a failure: %v", e)
	}
	gap := fmt.Errorf("%w: missing source drop level", loot.ErrOutOfDropRange)
	if e := fatalDropFailure(gap); e != nil {
		t.Fatalf("coverage gap treated as fatal, the death report would be withheld: %v", e)
	}
	for _, defect := range []error{
		errors.New("drop identity exhausted"),
		errors.New("invalid drop model tables"),
		errors.New("overlapping drop ranges"),
	} {
		if e := fatalDropFailure(defect); !errors.Is(e, defect) {
			t.Fatalf("model defect not propagated: %v", e)
		}
	}
}
