package inventory

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
)

// 2026-09-27 玩家反馈：GM 工具发的银增幅书（脚本声明 2022-11-08 到期）在客户端
// 显示「已过期」且无法使用。根因是发放路径从不写期限，物品行的偏移 56 恒为 0，
// 客户端把它当成 1970 到期 / 剩余 0 秒（取证见 protocol.MaxItemPeriod 注释）。
//
// 发放不是购买，没有真实倒计时可写，所以统一打永不过期哨兵（与商城
// cashshop.MaxExpireTime、礼盒开箱 internal/loot/box.go 一致），并且**写进存档**——
// 这样即使网关没开 DFO_MAX_ITEM_PERIOD，已发放的物品也不会过期。
func TestGrantWritesNonExpiringPeriod(t *testing.T) {
	// 关掉「带期限模板一律取最大值」的线上兜底，证明存档里本来就带着期限。
	protocol.ConfigureMaxItemPeriods(nil)
	t.Cleanup(func() { protocol.ConfigureMaxItemPeriods(nil) })

	const coin uint32 = 10418036 // 奥德赛银币，[unlimited waste]，消耗品槽 65..120
	a := testGrantAwarder(t)
	out, receipt, err := a.Grant(json.RawMessage(`{}`), coin, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipt.Slots) != 1 {
		t.Fatalf("发放槽位 %v，期望 1 个", receipt.Slots)
	}
	b, err := ReadBag(out)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range b.Items {
		if it.Slot != receipt.Slots[0] {
			continue
		}
		found = true
		if it.ExpireTime != GrantExpireTime {
			t.Fatalf("发放行 expire_time = %d，期望 %d", it.ExpireTime, GrantExpireTime)
		}
	}
	if !found {
		t.Fatalf("发放的行不在背包里：%+v", b.Items)
	}
	row, ok := b.RowAt(receipt.Slots[0])
	if !ok {
		t.Fatal("取不到发放行的报文")
	}
	if got := binary.LittleEndian.Uint32(row[56:60]); got != GrantExpireTime {
		t.Fatalf("下发行偏移 56 = %d，期望 %d（0 会被客户端判成已过期）", got, GrantExpireTime)
	}
}

// 背包里已有同模板的「已过期」旧堆时，发放不能并进去 —— 并进去就仍然是过期的。
func TestGrantSkipsExpiredStack(t *testing.T) {
	const coin uint32 = 10418036
	const expired uint32 = 1667865600 // 2022-11-08，银增幅书脚本声明的到期日
	a := testGrantAwarder(t)
	seed, err := SaveBag(json.RawMessage(`{}`), Bag{
		Version: "ordinary-bag-v1",
		Items:   []BagItem{{Slot: 65, Template: coin, Amount: 5, ExpireTime: expired}},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, receipt, err := a.Grant(seed, coin, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipt.Slots) != 1 || receipt.Slots[0] == 65 {
		t.Fatalf("发放并进了已过期的旧堆：%v", receipt.Slots)
	}
	b, err := ReadBag(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range b.Items {
		if it.Slot == receipt.Slots[0] && it.ExpireTime != GrantExpireTime {
			t.Fatalf("新堆 expire_time = %d，期望 %d", it.ExpireTime, GrantExpireTime)
		}
	}
}

// stampEquipmentPeriod 只动本次发放落到的槽位，不碰背包里其它装备行。
func TestStampEquipmentPeriodTouchesOnlyGrantedSlots(t *testing.T) {
	b := Bag{Equipment: []BagEquipment{{Slot: 10, Template: 1}, {Slot: 11, Template: 2, Period: 7}}}
	got := b.stampEquipmentPeriod([]uint16{10}, GrantExpireTime)
	if got.Equipment[0].Period != GrantExpireTime || got.Equipment[1].Period != 7 {
		t.Fatalf("期限打错了：%+v", got.Equipment)
	}
	if b.Equipment[0].Period != 0 {
		t.Fatal("改到了原背包而不是副本")
	}
}

func testGrantAwarder(t *testing.T) *Awarder {
	t.Helper()
	c, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = c.SupplementStackables("../catalog/testdata/item-flow.json"); err != nil {
		t.Fatal(err)
	}
	rules, err := LoadBagRules("../../configs/inventory.next29.json")
	if err != nil {
		t.Fatal(err)
	}
	gear, err := LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	return &Awarder{Catalog: c, Rules: rules, Equipment: gear}
}
