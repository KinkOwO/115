package main

import "testing"

func TestOdysseyReleaseGate(t *testing.T) {
	t.Setenv("DFO_ODYSSEY_REWARDS_PILOT", "")
	t.Setenv("DFO_ODYSSEY_REWARDS_RELEASE", "")
	t.Setenv("DFO_ODYSSEY_TEMPORARY_CREDITS", "")
	if odysseyRewardsEnabled() {
		t.Fatal("default enabled")
	}
	t.Setenv("DFO_ODYSSEY_REWARDS_RELEASE", "1")
	if !odysseyRewardsEnabled() {
		t.Fatal("release disabled")
	}
	if odysseyTemporaryCreditsEnabled() {
		t.Fatal("release silently grants temporary credits")
	}
	t.Setenv("DFO_ODYSSEY_TEMPORARY_CREDITS", "1")
	if !odysseyTemporaryCreditsEnabled() {
		t.Fatal("approved release credits disabled")
	}
	t.Setenv("DFO_ODYSSEY_TEMPORARY_CREDITS", "")
	t.Setenv("DFO_ODYSSEY_REWARDS_RELEASE", "")
	t.Setenv("DFO_ODYSSEY_REWARDS_PILOT", "1")
	if !odysseyRewardsEnabled() {
		t.Fatal("pilot disabled")
	}
}
