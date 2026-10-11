package main

import (
	"os"
	"testing"
)

func TestTownRobotManagementNamesNative(t *testing.T) {
	archive := os.Getenv("DFO_TOWN_ROBOT_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("set DFO_TOWN_ROBOT_TEST_ARCHIVE for current-PVF display metadata")
	}
	t.Setenv("DFO_PVF_ARCHIVE", archive)
	t.Setenv("DFO_PVF_SHA256", os.Getenv("DFO_TOWN_ROBOT_TEST_SHA256"))
	f := &townRobotFleet{stop: make(chan struct{}), manager: &townRobotManager{}}
	f.loadManagementNames()
	if !f.manager.displayReady {
		t.Fatal(f.manager.displayError)
	}
	if len(f.manager.display) == 0 {
		t.Fatal("no item metadata")
	}
	for _, id := range []uint32{27000, 39000} {
		d, ok := f.manager.display[id]
		if !ok || d.Name == "" {
			t.Fatalf("source item %d has no display name", id)
		}
		t.Logf("source item %d = %s", id, d.Name)
	}
	t.Logf("display-only metadata: %d items; no player storage or game listener", len(f.manager.display))
}
