package inventory

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"strings"
	"testing"
)

// 阶段 10 第二环节（装备变换）：源里**没有成本条目**的教学件，在 662 训练轨道内
// 必须按 0 材料 / 0 金币变换成功。
//
// 实机反例（2026-10-06 03:15:20，会话 ..._025747_213882）：
//
//	equipment craft open: panel=36 context=0x551afe0 action=0 pay_option=1 slots=[14] templates=[100051285]
//	equipment craft TRANSFORM-REFUSED: requested=1: need 35000 gold, have 6973 in bag + 0 in vault
//
// 根因：变换表 `[need materials]` 是按**稀有度**分档查的（`chain+rarity+payOption`），
// 不是按件。`100051285` 在 `[create cost]` 里根本没有条目（`GroupFor` 精确未命中），
// 却因稀有度是 legendary 命中「登记证×1 + 35,000 金币」——于是免单不触发、0 金币学员号被拒。
// 客户端对同一件显示的是 FREE ⇒ L0 口径就是 0。
func TestEquipmentTransformTutorialWaivesUnpricedTarget(t *testing.T) {
	prev := inBoostTraining
	SetInBoostTraining(func(state json.RawMessage) bool {
		return strings.Contains(string(state), `"_tut":"on"`)
	})
	t.Cleanup(func() { SetInBoostTraining(prev) })

	s, cc := loadTransformFixtures(t)
	rules := repairRules()
	s.Journal = &rules
	s.WearRules = WearRules{Slots: map[string]uint16{"[coat]": 14}}

	const (
		worn   = uint32(100051282) // 身上穿的 coat
		target = uint32(100051285) // 第 10 关教学件：源里没有成本条目
	)
	if _, ok := cc.GroupFor(target); ok {
		t.Fatalf("前提失效：%d 已在某个组的 items 里（源已给它定价），测不到免单", target)
	}
	if _, ok := JournalLimit(s.Equipment, s.Journal, target); !ok {
		t.Fatalf("前提失效：%d 不可登记", target)
	}
	// 前提：变换成本表**算得出**这件的成本（这正是缺陷的一半 —— 按稀有度命中 legendary 档）。
	if _, e := s.transformPayment(catalog.TransformChainEquipment, target, 1); e != nil {
		t.Fatalf("前提失效：%d 算不出变换成本（%v）", target, e)
	}

	// 训练轨道内：0 金币、0 账号材料也必须换得动。
	for _, tc := range []struct {
		name     string
		tutorial bool
		gold     uint32
		wantPair int
		wantErr  bool
	}{
		{"训练中 + 0 金币 ⇒ 免单成功", true, 0, 1, false},
		{"已出关 + 0 金币 ⇒ 照源扣、付不起", false, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bag := Bag{Version: "ordinary-bag-v1", Gold: tc.gold,
				Worn: []BagEquipment{{Slot: 14, Template: worn}},
			}
			state, e := SaveBag(json.RawMessage(`{"level":115,"advancement":0}`), bag)
			if e != nil {
				t.Fatal(e)
			}
			ledger := EquipmentJournal{Counts: map[uint32]uint32{target: 1}}
			if state, e = SaveEquipmentJournal(state, ledger); e != nil {
				t.Fatal(e)
			}
			if tc.tutorial {
				// 训练态由 ReadBag 从 state 现算，所以标记写进 state 本体。
				state = json.RawMessage(strings.TrimSuffix(string(state), "}") + `,"_tut":"on"}`)
			}
			role := Role{ConfigVersion: s.Catalog.Source.SaveIdentity(), State: state}

			plan, e := s.PlanEquipmentTransform(role, []uint32{14}, []uint32{target}, 1)
			if e != nil {
				t.Fatalf("plan: %v", e)
			}
			if len(plan.Steps) != 1 || !plan.Steps[0].unpriced {
				t.Fatalf("计划未按件标出 unpriced：%+v", plan.Steps)
			}

			accountRaw, e := NewAccountMaterials().Save()
			if e != nil {
				t.Fatal(e)
			}
			updated, _, _, receipt, e := s.PrepareEquipmentTransform(role, accountRaw, 0, plan)
			if tc.wantErr {
				if e == nil {
					t.Fatalf("出关后 0 金币竟然换成了：pairs=%+v gold=%d", receipt.Pairs, receipt.Gold)
				}
				return
			}
			if e != nil {
				t.Fatalf("训练轨道内应免单，却被拒：%v", e)
			}
			if len(receipt.Pairs) != 1 {
				t.Fatalf("pairs = %+v, want one", receipt.Pairs)
			}
			if receipt.Gold != 0 {
				t.Fatalf("免单仍扣了金币 %d", receipt.Gold)
			}
			live, e := ReadBag(updated)
			if e != nil {
				t.Fatal(e)
			}
			if got, ok := wornOf(live, 14); !ok || got != target {
				t.Fatalf("worn 14 = %d ok=%v, want %d（变换没落到身上）", got, ok, target)
			}
			next, e := ReadEquipmentJournal(updated)
			if e != nil {
				t.Fatal(e)
			}
			if next.Counts[target] != 0 || next.Counts[worn] != 1 {
				t.Fatalf("图鉴没记账：%v", next.Counts)
			}
		})
	}
}
