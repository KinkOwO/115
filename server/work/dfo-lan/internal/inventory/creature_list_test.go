package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"testing"
)

// wireCreature 是 NOTI105 里一条生物条目的可读投影（key + 名字）。
type wireCreature struct {
	Key  uint32
	Name string
}

// wireCreatures 按客户端读取器（sub_1452CA7E0）的布局解出下发列表，
// 并顺带校验整份载荷刚好读完，避免布局写错却断言通过。
func wireCreatures(t *testing.T, p []byte) []wireCreature {
	t.Helper()
	if len(p) == 0 {
		t.Fatal("空列表")
	}
	count, off := int(p[0]), 1
	out := make([]wireCreature, 0, count)
	for i := 0; i < count; i++ {
		if off+11 > len(p) {
			t.Fatalf("第 %d 条越界", i)
		}
		key := binary.LittleEndian.Uint32(p[off:])
		off += 5 // key + satiety
		mode := p[off]
		off += 5 // mode + exp
		if mode == 1 {
			off += 2
		}
		off++ // level
		n := int(binary.LittleEndian.Uint32(p[off:]))
		off += 4
		if off+n+1 > len(p) {
			t.Fatalf("第 %d 条名字越界", i)
		}
		out = append(out, wireCreature{key, string(p[off : off+n])})
		off += n + 1 // name + tail
	}
	if off != len(p) {
		t.Fatalf("列表长度不符：读完 %d 字节，共 %d 字节", off, len(p))
	}
	return out
}

// 2026-09-26 玩家报告：小退重登后 F6 的 Skin（幻化槽）框里图标消失，外观却仍是
// 幻化槽宠物的外观。
//
// 客户端画那个框是「按 key 解析到的生物对象」+ list 3 那一行，而 NOTI105 原先只
// 收录穿戴槽 26 与宠物栏，幻化槽宠物的 key 解析不到 ⇒ 框空白；外观走 mode-0
// 生物段（直接读存档）所以一直在。这里按实机形状断言幻化槽宠物既进了列表，
// key 又与 list 3 里槽 32 那一行完全一致。
func TestCreatureListIncludesSkinSlotCreature(t *testing.T) {
	// 实机形状：本体 63003(Charp, key1)、幻化槽 63011(Haagenti, 实例 key 3)、
	// 宠物栏 slot2 的 63008(Botis, key = slot+2 = 4)。
	skin := BagEquipment{Slot: CreatureSkinSlot, Template: 63011}
	skin.Record = make([]byte, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint32(skin.Record[2:], 63011)
	binary.LittleEndian.PutUint32(skin.Record[6:], 3)
	binary.LittleEndian.PutUint32(skin.Record[24:], 3)
	b := Bag{
		Version: "ordinary-bag-v1",
		Worn:    []BagEquipment{{Slot: 26, Template: 63003}, skin},
		Special: map[byte][]BagEquipment{7: {{Slot: 2, Template: 63008}}},
	}
	raw, err := SaveBag(json.RawMessage(`{}`), b)
	if err != nil {
		t.Fatal(err)
	}
	p, err := CreatureListPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	got := wireCreatures(t, p)
	want := []wireCreature{{Key: 1, Name: "Charp"}, {Key: 3, Name: "Haagenti"}, {Key: 4, Name: "Botis"}}
	if len(got) != len(want) {
		t.Fatalf("生物条目 = %v，期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 条 = %+v，期望 %+v（完整列表 %v）", i, got[i], want[i], got)
		}
	}

	// 列表里的 key 必须能从 list 3 槽 32 那一行解析到（客户端按 key 找物品行）。
	rows, err := EquipmentPayload(3, b.Worn, false)
	if err != nil {
		t.Fatal(err)
	}
	skinRow := rowAt(t, rows, 1)
	if s := binary.LittleEndian.Uint16(skinRow[0:]); s != CreatureSkinSlot {
		t.Fatalf("槽位 = %d，期望 %d", s, CreatureSkinSlot)
	}
	if tpl := binary.LittleEndian.Uint32(skinRow[2:]); tpl != 63011 {
		t.Fatalf("模板 = %d，期望 63011", tpl)
	}
	k6, k24 := binary.LittleEndian.Uint32(skinRow[6:]), binary.LittleEndian.Uint32(skinRow[24:])
	if k6 != 3 || k24 != 3 {
		t.Fatalf("幻化槽行实例键 = %d/%d，期望 3/3（列表里也是 3）", k6, k24)
	}
}

// 存档里没带实例 key 时，幻化槽宠物也要有 key，且与 list 3 行一致 —— 兜底值
// 不能是宠物本体的 1。
func TestSkinSlotCreatureFallbackKeyMatchesRow(t *testing.T) {
	b := Bag{Version: "ordinary-bag-v1", Worn: []BagEquipment{{Slot: CreatureSkinSlot, Template: 63011}}}
	raw, err := SaveBag(json.RawMessage(`{}`), b)
	if err != nil {
		t.Fatal(err)
	}
	p, err := CreatureListPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	got := wireCreatures(t, p)
	if len(got) != 1 || got[0].Key != CreatureSkinFallbackKey || got[0].Name != "Haagenti" {
		t.Fatalf("生物条目 = %v，期望单条 key=%d Haagenti", got, CreatureSkinFallbackKey)
	}
	if CreatureSkinFallbackKey == 1 {
		t.Fatal("兜底 key 与宠物本体的 1 撞车")
	}
	rows, err := EquipmentPayload(3, b.Worn, false)
	if err != nil {
		t.Fatal(err)
	}
	row := rowAt(t, rows, 0)
	if k6, k24 := binary.LittleEndian.Uint32(row[6:]), binary.LittleEndian.Uint32(row[24:]); k6 != CreatureSkinFallbackKey || k24 != CreatureSkinFallbackKey {
		t.Fatalf("幻化槽行实例键 = %d/%d，期望 %d", k6, k24, CreatureSkinFallbackKey)
	}
}

func TestCreatureListPayloadAndEquipmentRow(t *testing.T) {
	b := Bag{
		Version: "ordinary-bag-v1",
		Worn: []BagEquipment{
			{Slot: 26, Template: 63000}, // Faras
		},
		Special: map[byte][]BagEquipment{
			7: {
				{Slot: 0, Template: 63009}, // Marbas
				{Slot: 1, Template: 63006}, // unhatched Pareas egg
			},
		},
	}
	raw, err := SaveBag(json.RawMessage(`{}`), b)
	if err != nil {
		t.Fatal(err)
	}

	payload, err := CreatureListPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Expected 2 hatched creatures (slot 26 + slot 0), egg at slot 1 skipped
	if payload[0] != 2 {
		t.Fatalf("expected count 2, got %d", payload[0])
	}
	// Verify first entry (equipped creature at slot 26) has Key = 1
	key1 := binary.LittleEndian.Uint32(payload[1:5])
	if key1 != 1 {
		t.Fatalf("expected equipped creature key = 1, got %d", key1)
	}
	// Verify creature name Faras is written
	nameLen := binary.LittleEndian.Uint32(payload[12:16])
	if nameLen != 5 || string(payload[16:21]) != "Faras" {
		t.Fatalf("expected Faras name, got len=%d str=%s", nameLen, string(payload[16:16+nameLen]))
	}

	// Verify EquipmentRow of slot 26 has Key = 1 at offset 6
	row := EquipmentRow(b.Worn[0])
	dataVal := binary.LittleEndian.Uint32(row[6:10])
	if dataVal != 1 {
		t.Fatalf("expected EquipmentRow offset 6 = 1, got %d", dataVal)
	}
	if binary.LittleEndian.Uint16(row[0:2]) != 26 {
		t.Fatalf("expected slot 26, got %d", binary.LittleEndian.Uint16(row[0:2]))
	}
	if binary.LittleEndian.Uint32(row[2:6]) != 63000 {
		t.Fatalf("expected template 63000, got %d", binary.LittleEndian.Uint32(row[2:6]))
	}

	// Verify HasEquippedCreature
	if !HasEquippedCreature(raw) {
		t.Fatal("expected HasEquippedCreature to be true")
	}

	// Verify empty worn returns false
	emptyRaw, _ := SaveBag(json.RawMessage(`{}`), Bag{Version: "ordinary-bag-v1"})
	if HasEquippedCreature(emptyRaw) {
		t.Fatal("expected empty bag HasEquippedCreature to be false")
	}
}

func TestCreatureExperienceAwardPersistsAndLevels(t *testing.T) {
	bag := Bag{Version: "ordinary-bag-v1", Worn: []BagEquipment{{Slot: 26, Template: 63000}}}
	state, err := SaveBag(json.RawMessage(`{"other_state":7}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		exp   uint32
		level byte
	}{{1, 1}, {2, 2}} {
		var gained uint32
		state, gained, err = AwardEquippedCreatureExperience(state, 1)
		if err != nil || gained != 1 {
			t.Fatalf("award: gained=%d err=%v", gained, err)
		}
		growth, err := CreatureGrowthPayload(state)
		if err != nil || len(growth) != 6 || growth[0] != want.level || binary.LittleEndian.Uint32(growth[2:]) != want.exp {
			t.Fatalf("growth for exp %d: %x err=%v", want.exp, growth, err)
		}
		list, err := CreatureListPayload(state)
		if err != nil || binary.LittleEndian.Uint32(list[7:11]) != want.exp || list[11] != want.level {
			t.Fatalf("list for exp %d: %x err=%v", want.exp, list, err)
		}
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(state, &fields); err != nil || string(fields["other_state"]) != "7" {
		t.Fatalf("unrelated state changed: %s err=%v", state, err)
	}
	bag, err = ReadBag(state)
	if err != nil {
		t.Fatal(err)
	}
	bag.Special = map[byte][]BagEquipment{7: {{Slot: 0, Template: bag.Worn[0].Template, Record: bag.Worn[0].Record}}}
	bag.Worn = nil
	state, err = SaveBag(state, bag)
	if err != nil {
		t.Fatal(err)
	}
	if growth, err := CreatureGrowthPayload(state); err != nil || growth != nil {
		t.Fatalf("unequipped growth=%x err=%v", growth, err)
	}
	if _, gained, err := AwardEquippedCreatureExperience(state, 5); err != nil || gained != 0 {
		t.Fatalf("unequipped award=%d err=%v", gained, err)
	}
	bag.Worn = []BagEquipment{bag.Special[7][0]}
	bag.Worn[0].Slot = 26
	bag.Special = nil
	state, err = SaveBag(state, bag)
	if err != nil {
		t.Fatal(err)
	}
	growth, err := CreatureGrowthPayload(state)
	if err != nil || binary.LittleEndian.Uint32(growth[2:]) != 2 {
		t.Fatalf("re-equipped experience lost: %x err=%v", growth, err)
	}
}
