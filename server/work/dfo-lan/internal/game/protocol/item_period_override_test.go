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

// 一键启动器不设 DFO_MAX_ITEM_PERIOD，装载不到 PVF 期限模板表时也必须兜住：
// 存档里已有的非零期限（哪怕是早过去的到期时间戳）一律按永不过期下发。
func TestStoredPeriodLiftingCoversExpiredRows(t *testing.T) {
	const expired uint32 = 1667865600 // 2022-11-08，银增幅书脚本声明的到期日
	ConfigureMaxItemPeriods(nil)
	ConfigureSkinStoragePeriods(nil)
	ConfigureStoredPeriodLifting(false)
	t.Cleanup(func() {
		ConfigureMaxItemPeriods(nil)
		ConfigureSkinStoragePeriods(nil)
		ConfigureStoredPeriodLifting(false)
	})
	if got := ItemPeriodForWire(7001, expired); got != expired {
		t.Fatalf("兜底关闭时下发期限 = %d，期望原值 %d", got, expired)
	}
	if !StoredItemExpired(expired, int64(expired)+1) {
		t.Fatal("兜底关闭时必须仍然判过期")
	}

	ConfigureStoredPeriodLifting(true)
	if got := ItemPeriodForWire(7001, expired); got != MaxItemPeriod {
		t.Fatalf("兜底开启时下发期限 = %d，期望 %d", got, MaxItemPeriod)
	}
	if got := ItemPeriodForWire(7001, 1); got != MaxItemPeriod {
		t.Fatalf("非零期限 %d 未被抬升", 1)
	}
	if StoredItemExpired(expired, int64(expired)+1) {
		t.Fatal("兜底开启后服务端不应再判过期")
	}
	// 0 仍然原样下发 —— 没有期限语义的物品不该凭空长出「24856天」。
	if got := ItemPeriodForWire(7001, 0); got != 0 {
		t.Fatalf("零期限被改成 %d", got)
	}
}
