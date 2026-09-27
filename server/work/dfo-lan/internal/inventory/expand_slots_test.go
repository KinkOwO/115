package inventory

import (
	"encoding/json"
	"strings"
	"testing"
)

func expandState(t *testing.T, body string) json.RawMessage {
	t.Helper()
	return json.RawMessage(`{"level":84,"experience":123,"future_field":{"keep":true},"inventory":` + body + `}`)
}

// 扩展装备槽靠任务奖励累加解锁：support 1、magic stone 2、耳环 16。
// 这三个数是"解锁位"，不是 [reward int data] 里的槽索引 0/1/2 —— 耳环的索引 2
// 对应位 4，若直接按位或索引，存档会写成 1|2|4，耳环那一格永远不会开。
func TestUnlockEquipSlotsAccumulates(t *testing.T) {
	const body = `{"version":"ordinary-bag-v1","gold":7,"coin":2,"items":[{"slot":65,"Template":15,"Amount":1}],"worn":[{"slot":0,"Template":3}]}`
	raw := expandState(t, body)
	// 649 -> 650 -> 2636 的完成顺序：1 -> 3 -> 19。
	for _, step := range []struct{ mask, want byte }{{1, 1}, {2, 3}, {16, 19}} {
		out, e := UnlockEquipSlots(raw, step.mask)
		if e != nil {
			t.Fatal(e)
		}
		raw = out
		b, e := ReadBag(raw)
		if e != nil {
			t.Fatal(e)
		}
		if b.ExpandEquipFlags != step.want {
			t.Fatalf("mask %d: flags %d, want %d", step.mask, b.ExpandEquipFlags, step.want)
		}
	}
	// 重复领取同一个槽不改动已开位（按位或，不是赋值）。
	repeated, e := UnlockEquipSlots(raw, 1)
	if e != nil {
		t.Fatal(e)
	}
	if b, e := ReadBag(repeated); e != nil || b.ExpandEquipFlags != 19 {
		t.Fatalf("repeated unlock lost bits: %+v %v", b.ExpandEquipFlags, e)
	}
	// 顺序颠倒（2636 -> 650 -> 649）同样必须保留全部已开位。
	shuffled := expandState(t, body)
	for _, mask := range []byte{16, 2, 1} {
		if shuffled, e = UnlockEquipSlots(shuffled, mask); e != nil {
			t.Fatal(e)
		}
	}
	b, e := ReadBag(shuffled)
	if e != nil {
		t.Fatal(e)
	}
	if b.ExpandEquipFlags != 1|2|16 {
		t.Fatalf("out-of-order unlock lost a slot: %d", b.ExpandEquipFlags)
	}
	// 解锁不得丢弃背包、穿戴与无关的角色状态。
	if b.Gold != 7 || b.Coin != 2 || len(b.Items) != 1 || len(b.Worn) != 1 {
		t.Fatalf("unlock dropped bag contents: %+v", b)
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(shuffled, &fields); e != nil {
		t.Fatal(e)
	}
	if string(fields["future_field"]) != `{"keep":true}` || string(fields["experience"]) != "123" {
		t.Fatal("unlock dropped unrelated character state")
	}
}

// mask 0 是"这次奖励不开槽"，必须原样返回，不能顺手重写存档。
func TestUnlockEquipSlotsZeroMaskIsNoOp(t *testing.T) {
	raw := expandState(t, `{"version":"ordinary-bag-v1","gold":1}`)
	out, e := UnlockEquipSlots(raw, 0)
	if e != nil {
		t.Fatal(e)
	}
	if string(out) != string(raw) {
		t.Fatal("zero mask rewrote the character state")
	}
}

// 2026-09-22 之前的存档没有 expand_equip_flags：读作 0（未解锁），不是错误；
// 首次开槽后该字段才出现，且不覆盖存档里的其它键。
func TestUnlockEquipSlotsLegacySave(t *testing.T) {
	raw := expandState(t, `{"version":"ordinary-bag-v1","gold":1}`)
	if b, e := ReadBag(raw); e != nil || b.ExpandEquipFlags != 0 {
		t.Fatalf("legacy save unlock byte: %d %v", b.ExpandEquipFlags, e)
	}
	out, e := UnlockEquipSlots(raw, 2)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(out), `"expand_equip_flags":2`) {
		t.Fatalf("unlock byte missing from the saved state: %s", out)
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(out, &fields); e != nil {
		t.Fatal(e)
	}
	if string(fields["level"]) != "84" {
		t.Fatal("unlock dropped unrelated character state")
	}
}

// 客户端读的是单字节槽位标志，存档里出现更宽的值必须显式失败，
// 而不是被静默截断成另一个槽位组合。
func TestUnlockEquipSlotsRejectsWideSavedValue(t *testing.T) {
	raw := expandState(t, `{"version":"ordinary-bag-v1","expand_equip_flags":300}`)
	if _, e := ReadBag(raw); e == nil {
		t.Fatal("a saved unlock value wider than the native byte was accepted")
	}
}

// 幻化栏扩展券映射到 USERINFO1 解锁字节里独立的一位，且不得与装备槽的
// 1/2/16 重叠：145f02500 用 bit5 放开宠物幻化栏。
func TestSkinSlotMaskForTicket(t *testing.T) {
	creature, ok := SkinSlotMaskForTicket(CreatureSkinTicket)
	if !ok || creature != 1<<5 {
		t.Fatalf("creature ticket -> %d %v", creature, ok)
	}
	if creature&(ExpandSupport|ExpandMagicStone|ExpandEarring) != 0 {
		t.Fatal("the skin bit collides with an equipment unlock bit")
	}
	if _, ok := SkinSlotMaskForTicket(3037); ok {
		t.Fatal("an unrelated stackable claims a skin bit")
	}
}

// 券只能按模板反查：反查必须命中先出现的那一格，且不能把别的消耗品当成券。
func TestSkinSlotTicketSlotFindsByTemplate(t *testing.T) {
	raw := expandState(t, `{"version":"ordinary-bag-v1","items":[`+
		`{"slot":3,"Template":3037,"Amount":10},`+
		`{"slot":11,"Template":10309084,"Amount":2}]}`)
	b, e := ReadBag(raw)
	if e != nil {
		t.Fatal(e)
	}
	slot, ok := SkinSlotTicketSlot(b, ExpandCreatureSkin)
	if !ok || slot != 11 {
		t.Fatalf("creature ticket -> slot %d %v", slot, ok)
	}
	if _, ok = SkinSlotTicketSlot(b, ExpandEarring); ok {
		t.Fatal("an equipment unlock mask must not match a ticket")
	}
}
