package boostup

import "testing"

func TestBoostPointMissionNeedsContractsAndOneCompleteSet(t *testing.T) {
	r := &PointRequirement{Contracts: []byte{22, 27, 79, 92}, Minimum: 1265}
	active := map[uint8]bool{22: true, 27: true, 79: true, 92: true}
	if r.Satisfied(map[int32]uint32{1: 1150, 2: 115}, active) {
		t.Fatal("mixed sets combined")
	}
	points := map[int32]uint32{1: 1265}
	if !r.Satisfied(points, active) {
		t.Fatal("complete set and contracts rejected")
	}
	delete(active, 92)
	if r.Satisfied(points, active) {
		t.Fatal("missing cube contract accepted")
	}
	if r.Satisfied(nil, map[uint8]bool{22: true, 27: true, 79: true, 92: true}) {
		t.Fatal("contracts alone accepted")
	}
}
