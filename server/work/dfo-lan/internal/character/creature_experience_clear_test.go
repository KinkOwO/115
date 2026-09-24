package character

import "testing"

func TestDungeonCreatureExperienceUsesFreeRoomReceipts(t *testing.T) {
	tests := []struct {
		name                 string
		charged, loadedRooms int64
		exempt               bool
		want                 uint32
	}{
		{"free ordinary clear", 0, 7, false, 7},
		{"paid ordinary clear", 5, 7, false, 5},
		{"training room", 0, 7, true, 0},
		{"no loaded rooms", 0, 0, false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := dungeonCreatureExperience(tc.charged, tc.loadedRooms, tc.exempt)
			if err != nil || got != tc.want {
				t.Fatalf("gain=%d err=%v, want %d", got, err, tc.want)
			}
		})
	}
	if _, err := dungeonCreatureExperience(-1, 1, false); err == nil {
		t.Fatal("negative ledger amount accepted")
	}
}
