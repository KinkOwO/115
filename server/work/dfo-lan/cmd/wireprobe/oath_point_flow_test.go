package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"strconv"
	"strings"
	"testing"
)

// 积分源表的行格式在 catalog 包已有真实源用例；这里用最小样本钉住**计算范围与推送几何**：
//   - OathPoint 只有晶体 36..46 与誓约核心 47 参与；
//   - SetPoint 取全部穿戴装备（与名望同一批槽位），按「能力组 + 档位」查表；
//   - 每个候选键发一条 2634，载荷恰好 10 字节 = `u16 键 + u32 OathPoint + u32 SetPoint`。
const oathPointFlowTable = `
[oath point]
 [table]
  100401592 0 -1 45
  100610095 0 -1 355
 [/table]
[/oath point]
[min oath point] 1200
`

// setPointFlowTable 让 109040581（槽 12 的武器）在档位 0 命中 group 69 的 -1 行。
const setPointFlowTable = `
[set point]
 [table]
  [info]
   [group] 69
   [awakening] 0
   [part set index] -1
   [value] 140
  [/info]
 [/table]
[/set point]
`

// testEquipmentCatalog 造一个只含指定模板的最小装备目录，用来给积分用例提供
// `[part set index]`（套装号的唯一来源）。
func testEquipmentCatalog(t *testing.T, partSets map[uint32]int32) *inventory.EquipmentCatalog {
	t.Helper()
	const source = "point-flow-test"
	rows := make([]inventory.EquipmentDefinition, 0, len(partSets))
	for id, part := range partSets {
		rows = append(rows, inventory.EquipmentDefinition{
			ID:     id,
			Path:   "equipment/test/" + strconv.FormatUint(uint64(id), 10) + ".equ",
			SHA256: strings.Repeat("a", 64),
			Fields: map[string][]pvf.Token{"[part set index]": {{Type: 0, Value: part}}},
		})
	}
	c, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: source},
		Rows:   rows,
	}, source)
	if err != nil {
		t.Fatalf("test equipment catalog: %v", err)
	}
	return c
}

func TestOathPointPackets(t *testing.T) {
	oath, err := catalog.ParseOathPointInfo(oathPointFlowTable)
	if err != nil {
		t.Fatalf("parse oath point table: %v", err)
	}
	set, err := catalog.ParseSetPointInfo(setPointFlowTable)
	if err != nil {
		t.Fatalf("parse set point table: %v", err)
	}
	if len(set.Rules) != 1 || set.Rules[0].Group != 69 || set.Rules[0].PartSetIndex != -1 || set.Rules[0].Value != 140 {
		t.Fatalf("fixture set point rows = %+v, want one row {69 0 -1 140}", set.Rules)
	}
	rules := catalog.PointRules{
		Oath:          oath,
		Set:           set,
		AbilityGroups: map[uint32][]uint32{109040581: {69}},
	}
	state := []byte(`{"inventory":{"version":"ordinary-bag-v1","gold":0,"worn":[` +
		`{"slot":36,"template":100401592},` +
		`{"slot":47,"template":100610095},` +
		`{"slot":12,"template":109040581}]}}`)

	w := &worldSession{
		items: &inventory.ItemService{
			Points:    &rules,
			Equipment: testEquipmentCatalog(t, map[uint32]int32{109040581: 16300}),
		},
		role: database.Character{WireID: 7, State: state},
	}
	packets := w.oathPointPackets()
	if len(packets) != 1 {
		t.Fatalf("packets = %d, want exactly one (key narrowed after the live confirmation)", len(packets))
	}
	// 45（晶体 100401592）+ 355（誓约核心 100610095）= 400；武器不参与**誓约**积分。
	const wantOath = 45 + 355
	// 套装积分取全部穿戴：武器 109040581（套装号 16300）命中 group 69 的 -1 行 ⇒ 140；
	// 晶体/誓约核心的能力组不在表里 ⇒ 0。两者独立聚合。
	const wantSet = 140
	p := packets[0]
	if p.ID != protocol.PartSetPointOpcode {
		t.Fatalf("packet id = %d, want %d", p.ID, protocol.PartSetPointOpcode)
	}
	if len(p.Payload) != protocol.PartSetPointSize {
		t.Fatalf("payload = %d bytes, want %d", len(p.Payload), protocol.PartSetPointSize)
	}
	// 键由实机定案：`0xFFFF`（客户端印章的初始化明文）。
	if oathPartSetPointKey != 0xFFFF {
		t.Fatalf("oathPartSetPointKey = %#x, want 0xFFFF", oathPartSetPointKey)
	}
	if got := binary.LittleEndian.Uint16(p.Payload[0:2]); got != 0xFFFF {
		t.Fatalf("key = %#x, want 0xFFFF", got)
	}
	// ⚠️ 实机定案的顺序：`[2:6)` 是 **OathPoint**（落 `实体+1872`，誓约页签读它），
	// `[6:10)` 是 **SetPoint**（落 `实体+1876`，"套装积分"读它）。
	if got := binary.LittleEndian.Uint32(p.Payload[2:6]); got != wantOath {
		t.Fatalf("oath point (A/+1872) = %d, want %d", got, wantOath)
	}
	if got := binary.LittleEndian.Uint32(p.Payload[6:10]); got != wantSet {
		t.Fatalf("set point (B/+1876) = %d, want %d", got, wantSet)
	}
}

// 只有誓约表、没有能力组映射时：誓约积分照算，套装积分报"算不出"(0) —— 不允许把
// 「映射缺失」当成「角色套装积分为 0」推给客户端。
func TestOathPointPacketsWithoutAbilityGroups(t *testing.T) {
	oath, err := catalog.ParseOathPointInfo(oathPointFlowTable)
	if err != nil {
		t.Fatalf("parse oath point table: %v", err)
	}
	rules := catalog.PointRules{Oath: oath}
	state := []byte(`{"inventory":{"version":"ordinary-bag-v1","gold":0,"worn":[` +
		`{"slot":36,"template":100401592},` +
		`{"slot":47,"template":100610095}]}}`)
	w := &worldSession{
		items: &inventory.ItemService{Points: &rules},
		role:  database.Character{WireID: 7, State: state},
	}
	packets := w.oathPointPackets()
	if len(packets) != 1 {
		t.Fatalf("packets = %d, want 1 (oath points are still computable)", len(packets))
	}
	if got := binary.LittleEndian.Uint32(packets[0].Payload[2:6]); got != 45+355 {
		t.Fatalf("oath point = %d, want %d", got, 45+355)
	}
	if got := binary.LittleEndian.Uint32(packets[0].Payload[6:10]); got != 0 {
		t.Fatalf("set point = %d, want 0 when the ability-group mapping is missing", got)
	}
}

// 没有积分表时**不发**：宁可不推，也不能发一对 0 把"未知"写成"该角色积分为 0"。
func TestOathPointPacketsWithoutRules(t *testing.T) {
	state := []byte(`{"inventory":{"version":"ordinary-bag-v1","gold":0,"worn":[{"slot":47,"template":100610095}]}}`)
	w := &worldSession{items: &inventory.ItemService{}, role: database.Character{State: state}}
	if packets := w.oathPointPackets(); packets != nil {
		t.Fatalf("packets = %+v, want nil when the point table is not loaded", packets)
	}
}

// 穿戴范围：誓约积分取 36..47，其余槽位（武器/护甲）不取。
func TestOathPointItemsSlotRange(t *testing.T) {
	// 用角色 1 存档里的真实行（模板是有效 115 件，避免撞上 ReadBag 的行校验）。
	state := []byte(`{"inventory":{"version":"ordinary-bag-v1","gold":0,"worn":[` +
		`{"slot":12,"template":109040581},` +
		`{"slot":36,"template":100401633},` +
		`{"slot":37,"template":100401592},` +
		`{"slot":38,"template":100401632},` +
		`{"slot":47,"template":100610065}]}}`)
	items, err := oathPointItems(&inventory.ItemService{}, state)
	if err != nil {
		t.Fatalf("items: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("items = %+v, want the four rows in 36..47", items)
	}
	for i, want := range []uint32{100401633, 100401592, 100401632, 100610065} {
		if items[i].Template != want {
			t.Fatalf("items[%d] = %+v, want template %d", i, items[i], want)
		}
		// 晶体/誓约核心在源里没有 [part set index]；无装备目录时统一按 -1，档位取不到 record ⇒ 0。
		if items[i].PartSetIndex != -1 || items[i].Awakening != 0 {
			t.Fatalf("items[%d] = %+v, want defaults (awakening 0, set -1)", i, items[i])
		}
	}
}

// 套装积分取**全部**穿戴（含武器/护甲），并排除副手 24/30 与幻化 11/32 ——
// 与名望侧 EquipmentFameBreakdown 同一批槽位。
func TestWornPointItemsSetPointScope(t *testing.T) {
	state := []byte(`{"inventory":{"version":"ordinary-bag-v1","gold":0,"worn":[` +
		`{"slot":12,"template":109040581},` + // 武器：计入套装积分
		`{"slot":36,"template":100401633},` + // 晶体：计入
		`{"slot":47,"template":100610065},` + // 誓约核心：计入
		`{"slot":24,"template":109040582},` + // 副手：排除
		`{"slot":30,"template":109040583},` + // 副手：排除
		`{"slot":11,"template":109040584},` + // 光环幻化：排除
		`{"slot":32,"template":109040585}]}}`) // 宠物幻化：排除
	items, _, err := wornPointItems(&inventory.ItemService{}, state)
	if err != nil {
		t.Fatalf("items: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %+v, want 3 (the 24/30/11/32 slots are excluded)", items)
	}
	for i, want := range []uint32{109040581, 100401633, 100610065} {
		if items[i].Template != want {
			t.Fatalf("items[%d] = %+v, want template %d", i, items[i], want)
		}
	}
}
