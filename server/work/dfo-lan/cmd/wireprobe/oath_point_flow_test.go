package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/binary"
	"testing"
)

// 积分源表的行格式在 catalog 包已有真实源用例；这里用最小样本钉住**穿取范围与推送几何**：
//   - 只有晶体 36..46 与誓约核心 47 参与算分（武器/护甲不参与誓约积分）；
//   - 每个候选键发一条 2634，载荷恰好 10 字节 = `u16 键 + u32 SetPoint + u32 OathPoint`。
const oathPointFlowTable = `
[oath point]
 [table]
  100401592 0 -1 45
  100610095 0 -1 355
 [/table]
[/oath point]
[min oath point] 1200
`

func TestOathPointPackets(t *testing.T) {
	oath, err := catalog.ParseOathPointInfo(oathPointFlowTable)
	if err != nil {
		t.Fatalf("parse oath point table: %v", err)
	}
	rules := catalog.PointRules{Oath: oath}
	state := []byte(`{"inventory":{"version":"ordinary-bag-v1","gold":0,"worn":[` +
		`{"slot":36,"template":100401592},` +
		`{"slot":47,"template":100610095},` +
		`{"slot":12,"template":109040581}]}}`)

	w := &worldSession{
		items: &inventory.ItemService{Points: &rules},
		role:  storage.Character{WireID: 7, State: state},
	}
	packets := w.oathPointPackets()
	if len(packets) != 1 {
		t.Fatalf("packets = %d, want exactly one (key narrowed after the live confirmation)", len(packets))
	}
	// 45（晶体 100401592）+ 355（誓约核心 100610095）= 400；武器不参与。
	const wantOath = 45 + 355
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
	if got := binary.LittleEndian.Uint32(p.Payload[6:10]); got != 0 {
		t.Fatalf("set point (B/+1876) = %d, want 0 (set-point table key still unverified)", got)
	}
}

// 没有积分表时**不发**：宁可不推，也不能发一对 0 把"未知"写成"该角色积分为 0"。
func TestOathPointPacketsWithoutRules(t *testing.T) {
	state := []byte(`{"inventory":{"version":"ordinary-bag-v1","gold":0,"worn":[{"slot":47,"template":100610095}]}}`)
	w := &worldSession{items: &inventory.ItemService{}, role: storage.Character{State: state}}
	if packets := w.oathPointPackets(); packets != nil {
		t.Fatalf("packets = %+v, want nil when the point table is not loaded", packets)
	}
}

// 穿戴范围：36..47 全取，其余槽位（武器/护甲）不取。
func TestOathPointItemsSlotRange(t *testing.T) {
	// 用角色 1 存档里的真实行（模板是有效 115 件，避免撞上 ReadBag 的行校验）。
	state := []byte(`{"inventory":{"version":"ordinary-bag-v1","gold":0,"worn":[` +
		`{"slot":12,"template":109040581},` +
		`{"slot":36,"template":100401633},` +
		`{"slot":37,"template":100401592},` +
		`{"slot":38,"template":100401632},` +
		`{"slot":47,"template":100610065}]}}`)
	items, err := oathPointItems(state)
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
		if items[i].PartSetIndex != -1 || items[i].Awakening != 0 {
			t.Fatalf("items[%d] = %+v, want defaults (awakening 0, set -1)", i, items[i])
		}
	}
}
