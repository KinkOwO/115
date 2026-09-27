package protocol

import (
	"encoding/binary"
	"testing"
)

// The client refuses to use a period-declaring `[add skin storage]` consumable
// whose offset-56 cell is 0 (「剩余期限已过」, and it sends no use request at all), so
// those templates ship the sentinel. Nothing else may change: a row that holds a
// real remaining period keeps showing it, and the server still expires items.
func TestSkinStoragePeriodsOnlyLiftZeroPeriodRows(t *testing.T) {
	const skinTemplate = 10160911
	ConfigureMaxItemPeriods(nil)
	ConfigureSkinStoragePeriods(nil)
	t.Cleanup(func() {
		ConfigureMaxItemPeriods(nil)
		ConfigureSkinStoragePeriods(nil)
	})

	disabled := OrdinaryItem(69, skinTemplate, 1)
	if got := binary.LittleEndian.Uint32(disabled[56:60]); got != 0 {
		t.Fatalf("without the skin storage table period = %d, want 0", got)
	}

	ConfigureSkinStoragePeriods([]uint32{skinTemplate})
	row := OrdinaryItem(69, skinTemplate, 1)
	if got := binary.LittleEndian.Uint32(row[56:60]); got != MaxItemPeriod {
		t.Fatalf("zero-period skin consumable period = %d, want %d", got, MaxItemPeriod)
	}
	for _, stored := range []uint32{1, 604800} {
		kept := OrdinaryItem(70, skinTemplate, 1, stored)
		if got := binary.LittleEndian.Uint32(kept[56:60]); got != stored {
			t.Fatalf("stored period %d rewritten to %d", stored, got)
		}
	}
	unrelated := OrdinaryItem(71, skinTemplate+1, 1)
	if got := binary.LittleEndian.Uint32(unrelated[56:60]); got != 0 {
		t.Fatalf("unrelated template period = %d, want 0", got)
	}
	if !StoredItemExpired(1, 2) {
		t.Fatal("skin storage override must not disable the expiry check")
	}
}
