package catalog

import "testing"

func TestLotteryDiscoveryClassifiesWholePools(t *testing.T) {
	index := map[uint32]ItemIndexEntry{
		1: {ID: 1, Kind: "stackable"},
		2: {ID: 2, Kind: "equipment", Path: "equipment/weapon/test.equ"},
		3: {ID: 3, Kind: "avatar", Path: "equipment/avatar/test.equ"},
		4: {ID: 4, Kind: "equipment", Path: "equipment/creature/test.equ"},
	}
	for _, tc := range []struct {
		name      string
		values    []int32
		equipment bool
		valid     bool
	}{
		{"gold", []int32{0, 100, 1000000}, false, true},
		{"stackable", []int32{1, 5, 7}, false, true},
		{"mixed", []int32{1, 3, 2, 2, 7, 1, 3, 8, 1}, true, true},
		{"missing reward invalidates whole pool", []int32{1, 3, 2, 99, 7, 1}, false, false},
		{"creature", []int32{4, 5, 1}, false, false},
		{"multiple equipment", []int32{2, 5, 2}, false, false},
		{"zero weight", []int32{1, 0, 1}, false, false},
		{"zero count", []int32{1, 1, 0}, false, false},
		{"negative id", []int32{-1, 1, 1}, false, false},
		{"incomplete", []int32{1, 1}, false, false},
		{"empty", nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, equipment, err := classifyLotteryRewards(tc.values, index)
			if (err == nil) != tc.valid || equipment != tc.equipment {
				t.Fatal(rows, equipment, err)
			}
			if !tc.valid {
				if rows != nil {
					t.Fatal("partial pool retained")
				}
				return
			}
			if len(rows)*3 != len(tc.values) {
				t.Fatal("source rows dropped")
			}
			for i, row := range rows {
				for j, value := range row {
					if value != uint32(tc.values[i*3+j]) {
						t.Fatal("source odds/count/order changed")
					}
				}
			}
		})
	}
}
