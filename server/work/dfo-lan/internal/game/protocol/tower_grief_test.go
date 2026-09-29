package protocol

import (
	"encoding/hex"
	"testing"
)

func TestTowerGriefClearRewardReaderLayout(t *testing.T) {
	for _, check := range []struct {
		floor uint16
		want  string
	}{{1, "00000000010000"}, {100, "00000000640000"}} {
		got, err := TowerGriefClearReward(check.floor)
		if err != nil {
			t.Fatal(err)
		}
		if hex.EncodeToString(got) != check.want {
			t.Fatalf("floor %d NOTI1255 = %x, want %s", check.floor, got, check.want)
		}
	}
	for _, floor := range []uint16{0, 101} {
		if _, err := TowerGriefClearReward(floor); err == nil {
			t.Fatalf("accepted invalid floor %d", floor)
		}
	}
	items, err := TowerGriefClearReward(50, TowerRewardItem{Template: 123456, Amount: 3})
	if err != nil || hex.EncodeToString(items) != "0000000032000140e2010003000000" {
		t.Fatalf("item reward rows = %x, %v", items, err)
	}
	if _, err := TowerGriefClearReward(1, TowerRewardItem{Amount: 1}); err == nil {
		t.Fatal("accepted reward row without template")
	}
}
