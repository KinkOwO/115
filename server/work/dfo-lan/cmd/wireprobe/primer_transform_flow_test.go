package main

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
)

func TestPrimerTransformPayOptionResolution(t *testing.T) {
	cases := []struct {
		key  uint32
		want int
	}{
		{1, 1}, // 实机样本 / 构造器默认
		{2, 2}, // 若该字段真是付款方式，玩家切到巡礼之印
		{0, primerTransformPayOptionDefault},
		{3, primerTransformPayOptionDefault},
		{0xFFFFFFFF, primerTransformPayOptionDefault}, // 未初始化的 -1 也得兜住
	}
	for _, c := range cases {
		r := protocol.PrimerTransformRequest{WindowKey: c.key}
		if got := primerTransformPayOption(r); got != c.want {
			t.Fatalf("window key %d -> pay option %d, want %d", c.key, got, c.want)
		}
	}
}

// primerTransformFrame 造一帧 203 字节的 CMD2381 正文（record 下标 → 槽 36+下标）。
func primerTransformFrame(t *testing.T, index int, template uint32) []byte {
	t.Helper()
	if index < 0 || index >= protocol.PrimerTransformCrystalSlotCount {
		t.Fatalf("record index %d out of range", index)
	}
	body := make([]byte, protocol.PrimerTransformBodySize)
	binary.LittleEndian.PutUint32(body[13:], 1) // 窗口对象字段（构造器恒 1）
	// 行 0（誓约核心槽 47）保持空；[32:36] 与实机一致写 1。
	binary.LittleEndian.PutUint32(body[32:], 1)
	body[17] = 0x2e
	binary.LittleEndian.PutUint32(body[20:], protocol.PrimerTransformEmptyTemplate)
	binary.LittleEndian.PutUint32(body[28:], protocol.PrimerTransformEmptyTemplate)
	for i := 0; i < protocol.PrimerTransformCrystalSlotCount; i++ {
		off := protocol.PrimerTransformHeaderSize + i*protocol.PrimerTransformEntrySize
		body[off] = 0x2e
		binary.LittleEndian.PutUint32(body[off+3:], protocol.PrimerTransformEmptyTemplate)
		binary.LittleEndian.PutUint32(body[off+11:], protocol.PrimerTransformEmptyTemplate)
	}
	off := protocol.PrimerTransformHeaderSize + index*protocol.PrimerTransformEntrySize
	body[off], body[off+1], body[off+2] = 0x2e, 0, 0
	binary.LittleEndian.PutUint32(body[off+3:], template)
	return body
}

// primerDef 造一条晶体定义（`[primer]` 在 durabilityOptional 里，所以不需要 [durability]）。
//
// `[grade]` 是必需的：`equipmentGradeRarity` 同时读 `[grade]` 与 `[rarity]`（真源里晶体是 116）。
func primerDef(id uint32, rarity int32) inventory.EquipmentDefinition {
	return inventory.EquipmentDefinition{
		ID: id, Path: fmt.Sprintf("primer/%d.equ", id), SHA256: strings.Repeat("e", 64),
		Fields: map[string][]pvf.Token{
			"[grade]":           {{Type: 0, Value: 116}},
			"[rarity]":          {{Type: 0, Value: rarity}},
			"[equipment type]":  {{Type: 6, Text: "[primer]"}},
			"[minimum level]":   {{Type: 0, Value: 115}},
			"[item group name]": {{Type: 6, Text: "primer"}},
		},
	}
}

// primerTransformFlowTable 与真实源同形的**最小**成本/返还表（真实源由 catalog 包的真实源用例钉住）。
const primerTransformFlowTable = `[need materials]
 [info]
  [condition] 115 ` + "`rare`" + `
  [cost]
   [group] 1
    0 25000
    10361512 1
   [/group]
   [group] 2
    10401346 5
    10361512 1
   [/group]
  [/cost]
 [/info]
[/need materials]
[refund materials]
 115 ` + "`rare`" + ` 0 1 10361512 1
[/refund materials]
[need amalgamation materials]
 [info]
  [condition] 115 ` + "`unique`" + `
  [cost]
   [group] 1
    0 30000
   [/group]
   [group] 2
    10401346 6
   [/group]
  [/cost]
 [/info]
[/need amalgamation materials]
[refund amalgamation materials]
 115 ` + "`rare`" + ` 0 -1 0
[/refund amalgamation materials]
[need primer materials]
 [info]
  [condition] 115 ` + "`rare`" + `
  [cost]
   [group] 1
    0 25000
   [/group]
   [group] 2
    10401346 5
   [/group]
  [/cost]
 [/info]
 [info]
  [condition] 115 ` + "`legendary`" + `
  [cost]
   [group] 1
    0 35000
   [/group]
   [group] 2
    10401346 7
   [/group]
  [/cost]
 [/info]
 [info]
  [condition] 115 ` + "`primeval`" + `
  [cost]
   [group] 1
    0 50000
   [/group]
   [group] 2
    10401346 10
   [/group]
  [/cost]
 [/info]
[/need primer materials]
[refund primer materials]
 115 ` + "`rare`" + ` 0 1 10415190 1
 115 ` + "`rare`" + ` 1 0
 115 ` + "`legendary`" + ` 0 1 10415190 10
 115 ` + "`legendary`" + ` 1 0
 115 ` + "`primeval`" + ` 0 1 10415190 100
 115 ` + "`primeval`" + ` 1 0
[/refund primer materials]
`