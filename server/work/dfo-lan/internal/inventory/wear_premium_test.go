package inventory

import (
	"context"
	"dfolan/internal/catalog/pvf"
	"errors"
	"testing"
	"time"
)

type wearPremiumFake struct {
	active bool
	err    error
	calls  int
	t      *testing.T
}

func (p *wearPremiumFake) HasConquerorPremium(ctx context.Context, account int64, now time.Time) (bool, error) {
	p.calls++
	if account != 29 || now.IsZero() {
		p.t.Fatal("premium query changed", account, now)
	}
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 2*time.Second {
		p.t.Fatal("premium query lost timeout")
	}
	return p.active, p.err
}

// The injected read port preserves both the conqueror's ten-level allowance
// and the old behavior of ignoring a failed optional premium lookup.
func TestWearPremiumPortPreservesLevelAllowance(t *testing.T) {
	s, role := wearFixture(t)
	role.AccountID = 29
	def := s.Catalog.index[20002]
	fields := make(map[string][]pvf.Token, len(def.Fields))
	for key, value := range def.Fields {
		fields[key] = value
	}
	fields["[minimum level]"] = []pvf.Token{{Type: 0, Value: 16}}
	def.Fields = fields
	s.Catalog.index[20002] = def
	item := BagEquipment{Slot: 9, Template: 20002}
	if err := s.wearable(role, item, 19); err == nil {
		t.Fatal("non-premium character bypassed level requirement")
	}
	for _, tc := range []struct {
		active bool
		err    error
		allow  bool
	}{
		{true, nil, true}, {false, nil, false}, {false, errors.New("optional premium lookup unavailable"), false},
	} {
		port := &wearPremiumFake{active: tc.active, err: tc.err, t: t}
		s.PremiumStore = port
		err := s.wearable(role, item, 19)
		if (err == nil) != tc.allow || port.calls != 1 {
			t.Fatal("premium allowance changed", tc, err, port.calls)
		}
	}
}
