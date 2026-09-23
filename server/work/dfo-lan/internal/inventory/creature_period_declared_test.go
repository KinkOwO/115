package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"testing"
)

// 2026-09-23 实机反馈：只补哨兵值确实不再显示「剩余期限已过。」，但面板出现了
// 「过期时间:24856天」。客户端把行内偏移 56 当**剩余秒数**，除以 86400 后用
// `Expires in : %d Day(s)`（dstr 1057，PopupWindow\CNRDItemInfoWindow.cpp）渲染，
// 且不减当前时间：2147483647 / 86400 = 24855.6 → 24856，与面板逐位吻合。
//
// 所以对脚本声明了 `[usable period] N` 的生物，这一格必须是 N*86400 秒。
// 这组用例锁死「按脚本真值换算」这条路，以及查表不可用时的兜底。

// creaturePeriodStub 安装一个假的期限查表，并在用例结束时恢复。
func creaturePeriodStub(t *testing.T, table map[uint32]int32) {
	t.Helper()
	prev := creaturePeriodDays
	SetCreaturePeriodSource(func(template uint32) (int32, bool) {
		days, ok := table[template]
		return days, ok
	})
	t.Cleanup(func() { creaturePeriodDays = prev })
}

// 脚本声明 7 天 ⇒ 必须下发 7*86400 = 604800 秒（面板才会显示「7天」）。
func TestDeclaredCreaturePeriodBecomesSeconds(t *testing.T) {
	creaturePeriodStub(t, map[uint32]int32{100991331: 7})
	p, e := EquipmentPayload(7, []BagEquipment{creaturePeriodFixture(0, 100991331)}, false)
	if e != nil {
		t.Fatal(e)
	}
	got := expiryOf(rowAt(t, p, 0))
	if got != 7*secondsPerDay {
		t.Fatalf("宠物行偏移 56 = %d，期望 %d（7 天换算成秒）", got, 7*secondsPerDay)
	}
	// 自检：客户端按 ÷86400 显示天数，必须正好是物品名里写的 7 天。
	if days := got / 86400; days != 7 {
		t.Fatalf("面板将显示 %d 天，期望 7 天", days)
	}
}

// 登录那条 restore 路径同样要带上换算后的秒数。
func TestRestorePayloadUsesDeclaredPeriod(t *testing.T) {
	creaturePeriodStub(t, map[uint32]int32{100991331: 7})
	state := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","gold":0,"items":[],
		"special_equipment":{"7":[{"slot":0,"template":100991331,"durability":0}]}}}`)
	p, e := SpecialEquipmentRestorePayload(state, 7)
	if e != nil {
		t.Fatal(e)
	}
	if got := expiryOf(rowAt(t, p, 0)); got != 7*secondsPerDay {
		t.Fatalf("restore 下发偏移 56 = %d，期望 %d", got, 7*secondsPerDay)
	}
}

// 穿戴中的宠物（space 3 槽位 26）走另一条路径，同样按脚本真值。
func TestWornCreatureUsesDeclaredPeriod(t *testing.T) {
	creaturePeriodStub(t, map[uint32]int32{100991331: 30})
	p, e := EquipmentPayload(3, []BagEquipment{creaturePeriodFixture(26, 100991331)}, false)
	if e != nil {
		t.Fatal(e)
	}
	if got := expiryOf(rowAt(t, p, 0)); got != 30*secondsPerDay {
		t.Fatalf("穿戴宠物行偏移 56 = %d，期望 %d", got, 30*secondsPerDay)
	}
}

// 查表不可用（未安装 / 查不到 / 取值不是真实期限）时，退回兼容兜底，不得填坏值。
func TestCreaturePeriodFallsBackWhenUnusable(t *testing.T) {
	cases := []struct {
		name  string
		table map[uint32]int32
		id    uint32
	}{
		{"未声明期限", map[uint32]int32{}, 100991331},
		{"超过 100 年", map[uint32]int32{100991331: 36501}, 100991331},
		{"天数为 0", map[uint32]int32{100991331: 0}, 100991331},
		{"天数为负", map[uint32]int32{100991331: -3}, 100991331},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			creaturePeriodStub(t, c.table)
			p, e := EquipmentPayload(7, []BagEquipment{creaturePeriodFixture(0, c.id)}, false)
			if e != nil {
				t.Fatal(e)
			}
			if got := expiryOf(rowAt(t, p, 0)); got != protocol.MaxItemPeriod {
				t.Fatalf("偏移 56 = %d，期望兜底值 %d", got, protocol.MaxItemPeriod)
			}
		})
	}
}

// 未安装查表时（例如某些入口没配全量装备目录）行为必须与修复前一致，不得 panic。
func TestCreaturePeriodWithoutLookupKeepsSentinel(t *testing.T) {
	prev := creaturePeriodDays
	SetCreaturePeriodSource(nil)
	t.Cleanup(func() { creaturePeriodDays = prev })
	p, e := EquipmentPayload(7, []BagEquipment{creaturePeriodFixture(0, 100991331)}, false)
	if e != nil {
		t.Fatal(e)
	}
	if got := expiryOf(rowAt(t, p, 0)); got != protocol.MaxItemPeriod {
		t.Fatalf("偏移 56 = %d，期望兜底值 %d", got, protocol.MaxItemPeriod)
	}
}

// 存档里记过期限时仍然优先（装扮礼盒那条路径的语义不变）。
func TestStoredPeriodStillWinsOverDeclared(t *testing.T) {
	creaturePeriodStub(t, map[uint32]int32{100991331: 7})
	item := creaturePeriodFixture(0, 100991331)
	item.Period = 12345
	p, e := EquipmentPayload(7, []BagEquipment{item}, false)
	if e != nil {
		t.Fatal(e)
	}
	if got := expiryOf(rowAt(t, p, 0)); got != 12345 {
		t.Fatalf("偏移 56 = %d，期望沿用存档里的 12345", got)
	}
}

// 普通装备行不受影响：即使查表里有值，非生物行也不许被填期限。
func TestOrdinaryRowIgnoresCreaturePeriodLookup(t *testing.T) {
	creaturePeriodStub(t, map[uint32]int32{100051399: 7})
	p, e := EquipmentPayload(3, []BagEquipment{creaturePeriodFixture(14, 100051399)}, false)
	if e != nil {
		t.Fatal(e)
	}
	if got := expiryOf(rowAt(t, p, 0)); got != 0 {
		t.Fatalf("普通装备行偏移 56 = %d，期望 0", got)
	}
	// 顺带确认没有碰坏模板字段。
	if got := binary.LittleEndian.Uint32(rowAt(t, p, 0)[2:]); got != 100051399 {
		t.Fatalf("模板被改坏：%d", got)
	}
}
