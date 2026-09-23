package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"testing"
)

// 2026-09-23 玩家报告：送的奥德赛宠物在生物栏提示「剩余期限已过。」。
//
// 宠物的脚本（`equipment/creature/100991331.equ`）里有 `[usable period] 7`，
// 而装备行的偏移 56 一直是 0 —— 这一格是**剩余秒数**（客户端除以 86400 显示天数），
// 0 即已过期。这里**按字节**断言真正下发的那一行：线上表现由它决定，
// 存档里那个 period 字段只是"服务端记过没有"。
//
// 本文件断言的是**未安装期限查表**时的兜底路径（MaxItemPeriod）；安装查表后
// 按脚本真值换算成 N*86400 的情形见 creature_period_declared_test.go。
//
// 下的行布局（space 7 / space 3 非装扮行，restore 与否都不带前缀）：
// [0] space, [1:3] 行数 u16, [3:3+181] 第 0 行 …
const (
	creaturePeriodRowBase = 3
	expiryCell            = 56
)

// creaturePeriodFixture 造一只"存档里没记过期限"的宠物，与实机那只一致。
func creaturePeriodFixture(slot uint16, template uint32) BagEquipment {
	return BagEquipment{Slot: slot, Template: template}
}

func rowAt(t *testing.T, payload []byte, index int) []byte {
	t.Helper()
	start := creaturePeriodRowBase + index*protocol.CurrentItemRecordSize
	end := start + protocol.CurrentItemRecordSize
	if len(payload) < end {
		t.Fatalf("下发行太短：len=%d, 需要 %d", len(payload), end)
	}
	return payload[start:end]
}

func expiryOf(row []byte) uint32 { return binary.LittleEndian.Uint32(row[expiryCell:]) }

// 生物栏（space 7）里的宠物：偏移 56 必须非零，否则客户端显示「剩余期限已过」。
func TestCreatureRowGetsNonZeroExpiry(t *testing.T) {
	p, e := EquipmentPayload(7, []BagEquipment{creaturePeriodFixture(0, 100991331)}, false)
	if e != nil {
		t.Fatal(e)
	}
	row := rowAt(t, p, 0)
	if got := binary.LittleEndian.Uint16(row[0:]); got != 0 {
		t.Fatalf("槽位被改坏：%d", got)
	}
	if got := binary.LittleEndian.Uint32(row[2:]); got != 100991331 {
		t.Fatalf("模板被改坏：%d", got)
	}
	if got := expiryOf(row); got != protocol.MaxItemPeriod {
		t.Fatalf("宠物行到期时间戳 = %d，期望 MaxItemPeriod(%d)", got, protocol.MaxItemPeriod)
	}
	// 同一次下发里，宠物实例键（偏移 6/24）的行为不能变。
	if k6, k24 := binary.LittleEndian.Uint32(row[6:]), binary.LittleEndian.Uint32(row[24:]); k6 == 0 || k6 != k24 {
		t.Fatalf("宠物实例键被改坏：offset6=%d offset24=%d", k6, k24)
	}
}

// 穿戴在身上的宠物（space 3 槽位 26）走另一条下发路径，同样要带期限。
func TestWornCreatureRowGetsNonZeroExpiry(t *testing.T) {
	p, e := EquipmentPayload(3, []BagEquipment{creaturePeriodFixture(26, 100991331)}, false)
	if e != nil {
		t.Fatal(e)
	}
	if got := expiryOf(rowAt(t, p, 0)); got != protocol.MaxItemPeriod {
		t.Fatalf("已穿戴宠物行到期时间戳 = %d，期望 %d", got, protocol.MaxItemPeriod)
	}
}

// 回归：普通装备行（space 3 槽位 14）不得被顺手填上期限。
func TestOrdinaryEquipmentRowKeepsZeroExpiry(t *testing.T) {
	p, e := EquipmentPayload(3, []BagEquipment{creaturePeriodFixture(14, 100051399)}, false)
	if e != nil {
		t.Fatal(e)
	}
	if got := expiryOf(rowAt(t, p, 0)); got != 0 {
		t.Fatalf("普通装备行被填了到期时间戳 %d，期望 0", got)
	}
}

// 服务端自己记过期限时以记的为准（不覆盖成哨兵值）。
func TestStoredCreaturePeriodWins(t *testing.T) {
	item := creaturePeriodFixture(0, 100991331)
	item.Period = 12345
	p, e := EquipmentPayload(7, []BagEquipment{item}, false)
	if e != nil {
		t.Fatal(e)
	}
	if got := expiryOf(rowAt(t, p, 0)); got != 12345 {
		t.Fatalf("到期时间戳 = %d，期望沿用存档里的 12345", got)
	}
}

// 行里已经被 Record 填过非零期限（服务端实例字节）时不许覆盖。
func TestExistingRecordExpiryIsPreserved(t *testing.T) {
	item := creaturePeriodFixture(0, 100991331)
	// Record 是服务端保留的实例字节，必须自带模板（ValidateRecord 会核对）。
	item.Record = make([]byte, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint32(item.Record[2:], 100991331)
	binary.LittleEndian.PutUint32(item.Record[expiryCell:], 999)
	p, e := EquipmentPayload(7, []BagEquipment{item}, false)
	if e != nil {
		t.Fatal(e)
	}
	if got := expiryOf(rowAt(t, p, 0)); got != 999 {
		t.Fatalf("到期时间戳 = %d，期望保留实例字节里的 999", got)
	}
}

// 端到端：从存档 JSON 出发（登录时那条 restore 路径），实机那只宠物必须带上期限。
func TestRestorePayloadStampsCreaturePeriod(t *testing.T) {
	state := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","gold":0,"items":[],
		"special_equipment":{"7":[{"slot":0,"template":100991331,"durability":0}]}}}`)
	p, e := SpecialEquipmentRestorePayload(state, 7)
	if e != nil {
		t.Fatal(e)
	}
	if p[0] != 7 {
		t.Fatalf("space = %d，期望 7", p[0])
	}
	row := rowAt(t, p, 0)
	if got := binary.LittleEndian.Uint32(row[2:]); got != 100991331 {
		t.Fatalf("模板 = %d，期望 100991331", got)
	}
	if got := expiryOf(row); got != protocol.MaxItemPeriod {
		t.Fatalf("下发到客户端的到期时间戳 = %d，期望 %d", got, protocol.MaxItemPeriod)
	}
}
