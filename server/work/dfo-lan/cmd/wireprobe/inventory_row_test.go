package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
)

// 格子还在 → 返回真实行；格子被用光（整行移除）→ 必须返回**空行**
// （模板 0xFFFFFFFF）。返回「有模板 + 数量 0」是不够的：客户端会把图标留着。
func TestBagRowOrEmptyClearsRemovedSlot(t *testing.T) {
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Items:   []inventory.BagItem{{Slot: 77, Template: 50022396, Amount: 2}},
	}
	// 还在：原样返回。
	row := bagRowOrEmpty(bag, 77)
	if got := binary.LittleEndian.Uint32(row[2:]); got != 50022396 {
		t.Fatalf("格子还在时模板应原样返回，实际 %d", got)
	}
	if got := binary.LittleEndian.Uint32(row[6:]); got != 2 {
		t.Fatalf("格子还在时数量应原样返回，实际 %d", got)
	}
	// 用光：行被移除 → 空行。
	empty := bagRowOrEmpty(bag, 78)
	if got := binary.LittleEndian.Uint16(empty[:]); got != 78 {
		t.Fatalf("空行必须带槽位 78，实际 %d", got)
	}
	if got := binary.LittleEndian.Uint32(empty[2:]); got != 0xFFFFFFFF {
		t.Fatalf("空行模板必须是 0xFFFFFFFF，实际 %#x", got)
	}
	if want := protocol.EmptyOrdinaryItem(78); empty != want {
		t.Fatal("空行必须与 protocol.EmptyOrdinaryItem 完全一致")
	}
}

func TestEquipmentRefreshPreservesRowsAndAcknowledgementOrder(t *testing.T) {
	bag := inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 77, Template: 3326, Amount: 2}}}
	rows := equipmentRows(bag, 0, 10, 77, 78)
	want := [][protocol.CurrentItemRecordSize]byte{protocol.OrdinaryItem(77, 3326, 2), protocol.EmptyOrdinaryItem(78), protocol.EmptyOrdinaryItem(10)}
	if !reflect.DeepEqual(rows, want) {
		t.Fatal("material, consumed protection and equipment row order changed")
	}
	ack := outboundPacket{"result", 1, 80, []byte{1}}
	plan, err := appendEquipmentRefresh([]outboundPacket{ack}, nil, rows, 0, "inventory", "worn")
	if err != nil {
		t.Fatal(err)
	}
	body, err := protocol.InventoryUpdate(want)
	if err != nil || !reflect.DeepEqual(plan, []outboundPacket{ack, {"inventory", 0, 14, body}}) {
		t.Fatal("acknowledgement or inventory bytes changed", err)
	}
	// Enchant puts its acknowledgement after the same refresh group.
	plan, err = appendEquipmentRefresh(nil, nil, rows, 0, "inventory", "worn")
	if err != nil || len(plan) != 1 {
		t.Fatal(plan, err)
	}
	plan = append(plan, ack)
	if plan[0].ID != 14 || plan[1].ID != 80 {
		t.Fatal("refresh-before-ack order changed")
	}
}

func TestEquipmentRefreshWornAndMalformedState(t *testing.T) {
	bag := inventory.Bag{Version: "ordinary-bag-v1"}
	rows := equipmentRows(bag, 3, 12, 77)
	if !reflect.DeepEqual(rows, [][protocol.CurrentItemRecordSize]byte{protocol.EmptyOrdinaryItem(77)}) {
		t.Fatal("worn equipment must not appear in list0")
	}
	state := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","worn":[{"slot":12,"template":100,"record":"AQ=="}]}}`)
	// A malformed persisted equipment record must remain an error.
	if _, err := appendEquipmentRefresh(nil, state, rows, 3, "inventory", "worn"); err == nil {
		t.Fatal("invalid worn record accepted")
	}
	state = json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1"}}`)
	plan, err := appendEquipmentRefresh(nil, state, rows, 3, "inventory", "worn")
	if err != nil || len(plan) != 1 {
		t.Fatal("empty worn group must be omitted", plan, err)
	}
	state = json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","worn":[{"slot":12,"template":100,"durability":10}]}}`)
	plan, err = appendEquipmentRefresh(nil, state, rows, 3, "inventory", "worn")
	worn, encodeErr := inventory.EquipmentPayload(3, []inventory.BagEquipment{{Slot: 12, Template: 100, Durability: 10}}, false)
	if err != nil || encodeErr != nil || len(plan) != 2 || plan[1].Name != "worn" || !bytes.Equal(plan[1].Payload, worn) {
		t.Fatal("worn update bytes or order changed", plan, err, encodeErr)
	}
}

// 守卫：强化 / 增幅 / 锻造三个流程文件里，**除金币与点券这两格之外**的行刷新
// 必须一律走 bagRowOrEmpty。
//
// slot 0/1 由 inventory.Bag.RowAt 自己合成（永远返回 ok=true），其余格子都可能
// 被用光；手写 `row, ok := bag.RowAt(slot); if ok { ... }` 就会漏掉「整行被移除」
// 那一支 —— 实机表现就是「保护券只有一张时扣掉后图标不消失，点整理背包才没」
// （2026-09-28 玩家反馈）。
func TestUpgradeFlowsDoNotHandRollRowLookups(t *testing.T) {
	re := regexp.MustCompile(`bag\.RowAt\(([^)]*)\)`)
	for _, name := range []string{"reinforcement_flow.go", "amplify_flow.go", "refine_flow.go", "enchant_flow.go"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		body := string(src)
		if !bytes.Contains(src, []byte("equipmentRows(")) {
			t.Errorf("%s must use the shared equipment row builder", name)
		}
		for _, m := range re.FindAllStringSubmatch(body, -1) {
			arg := strings.TrimSpace(m[1])
			if arg == "0" || arg == "1" {
				continue // 金币 / 点券：RowAt 自己合成，不会「被用光」
			}
			t.Errorf("%s: bag.RowAt(%s) 必须改走 bagRowOrEmpty —— "+
				"手写存在性判断会漏掉「这一格被用光」的那一支", name, arg)
		}
	}
}
