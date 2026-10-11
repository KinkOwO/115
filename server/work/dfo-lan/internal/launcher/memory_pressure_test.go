package launcher

import (
	"testing"
	"time"
)

func TestMemoryPressureGuard(t *testing.T) {
	type reading struct {
		at          time.Duration
		available   uint64
		private     uint64
		event       string
		stopTrigger string
	}
	const healthyPrivate = 1228 << 20
	const low = 1675 << 20
	const critical = 400 << 20
	cases := []struct {
		name     string
		readings []reading
	}{
		{"recorded_disconnect_spike_recovers", []reading{
			{0, 6776 << 20, healthyPrivate, "", ""},
			{2200 * time.Millisecond, 3005 << 20, healthyPrivate, "", ""},
			{2500 * time.Millisecond, low, healthyPrivate, "warning", ""},
			{3 * time.Second, 4800 << 20, healthyPrivate, "recovered", ""},
		}},
		{"continuous_low_memory", []reading{
			{0, low, healthyPrivate, "warning", ""},
			{(30 * time.Second) - time.Millisecond, low, healthyPrivate, "", ""},
			{(30 * time.Second), low, healthyPrivate, "", ""},
		}},
		{"repeated_spikes_do_not_accumulate", []reading{
			{0, low, healthyPrivate, "warning", ""},
			{29 * time.Second, systemLowMemoryBytes, healthyPrivate, "recovered", ""},
			{30 * time.Second, low, healthyPrivate, "warning", ""},
			{59 * time.Second, low, healthyPrivate, "", ""},
			{60 * time.Second, low, healthyPrivate, "", ""},
		}},
		{"critical_spike_recovers", []reading{
			{0, critical, healthyPrivate, "warning", ""},
			{(2 * time.Second) - time.Millisecond, 4 << 30, healthyPrivate, "recovered", ""},
		}},
		{"continuous_critical_memory", []reading{
			{0, critical, healthyPrivate, "warning", ""},
			{(2 * time.Second) - time.Millisecond, critical, healthyPrivate, "", ""},
			{(2 * time.Second), critical, healthyPrivate, "", ""},
		}},
		{"critical_timer_resets_with_partial_recovery", []reading{
			{0, critical, healthyPrivate, "warning", ""},
			{time.Second, (512 << 20), healthyPrivate, "", ""},
			{2 * time.Second, critical, healthyPrivate, "", ""},
			{3 * time.Second, critical, healthyPrivate, "", ""},
			{4 * time.Second, critical, healthyPrivate, "", ""},
		}},
		{"low_timer_survives_partial_critical_recovery", []reading{
			{0, low, healthyPrivate, "warning", ""},
			{28 * time.Second, critical, healthyPrivate, "", ""},
			{29 * time.Second, low, healthyPrivate, "", ""},
			{30 * time.Second, low, healthyPrivate, "", ""},
		}},
		{"recovery_resets_both_timers", []reading{
			{0, critical, healthyPrivate, "warning", ""},
			{time.Second, systemLowMemoryBytes, healthyPrivate, "recovered", ""},
			{31 * time.Second, critical, healthyPrivate, "warning", ""},
			{32 * time.Second, critical, healthyPrivate, "", ""},
		}},
		{"commit_limit_still_immediate", []reading{
			{0, 8 << 30, serverMemoryBudget - serverMemoryBudget/16 + 1, "", "process_commit_limit"},
		}},
		{"commit_boundary_unchanged", []reading{
			{0, systemLowMemoryBytes, serverMemoryBudget - serverMemoryBudget/16, "", ""},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var guard memoryPressureGuard
			start := time.Date(2026, 10, 7, 2, 36, 0, 0, time.UTC)
			for _, sample := range tc.readings {
				got := guard.observe(start.Add(sample.at), sample.private, sample.available, serverMemoryBudget)
				if got.event != sample.event || got.stopTrigger != sample.stopTrigger {
					t.Fatalf("at %s: event=%q stop=%q; want event=%q stop=%q", sample.at, got.event, got.stopTrigger, sample.event, sample.stopTrigger)
				}
			}
		})
	}
}
