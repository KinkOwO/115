package loot

import (
	"dfolan/internal/aradpass"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"encoding/json"
	"strings"
	"testing"
)

// aradPassTestCatalog 造一份**最小可发放**的目录：只声明几条 `[etc]` 可堆叠物，
// 与 arad pass 奖励表里的物品号一一对应。
//
// 刻意不读仓库的 loot.next25.json —— 战令奖励是 5907225xx/5907226xx 这一批
// **活动专属**模板，不在常规掉落表里；用合成目录才能把"领奖 → 落槽位"这条链
// 单独钉住，不依赖 PVF。
func aradPassTestCatalog(t *testing.T) (catalog.LootCatalog, inventory.BagRules) {
	t.Helper()
	rules, err := inventory.LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	c := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{}}
	for _, id := range []uint32{590722601, 590722646, 590722598, 590722593} {
		c.Items[id] = catalog.LootItem{ID: id, Kind: "stackable", StackableType: "[etc]", StackLimit: 1000}
	}
	return c, rules
}

func aradPassTestSource(t *testing.T) *aradpass.Catalog {
	t.Helper()
	// 等级门槛：1→0、2→600、3→1200（与源 `[pass level info]` 同一形状，缩短到 3 级）。
	return &aradpass.Catalog{
		Levels: []aradpass.Level{{Level: 1, Exp: 0}, {Level: 2, Exp: 600}, {Level: 3, Exp: 1200}},
		Rewards: []aradpass.Reward{
			{Track: aradpass.TrackNormal, Slot: 1, Item: 590722601, Count: 2, Level: 1},
			{Track: aradpass.TrackNormal, Slot: 2, Item: 590722601, Count: 1, Level: 3},
			// 等级档刻意设成 1：皇家轨的**准入**判据是"有没有买"，等级门槛另有
			// 普通轨第 2 档覆盖，两条判据别在同一个用例里互相遮挡。
			{Track: aradpass.TrackRoyal, Slot: 1, Item: 590722646, Count: 1, Level: 1},
		},
		RoyalFirstReward:  []uint32{590722598, 590722593},
		PurchaseExp:       5000,
		WeeklyGainableExp: 7150,
	}
}

func aradPassTestService(t *testing.T) (*Service, *aradpass.Catalog) {
	t.Helper()
	c, rules := aradPassTestCatalog(t)
	return &Service{Catalog: c, BagRules: rules}, aradPassTestSource(t)
}

func bagTemplates(t *testing.T, state json.RawMessage) map[uint32]uint32 {
	t.Helper()
	bag, err := inventory.ReadBag(state)
	if err != nil {
		t.Fatal(err)
	}
	out := map[uint32]uint32{}
	for _, item := range bag.Items {
		out[item.Template] += item.Amount
	}
	return out
}

// 普通轨第一档：等级 1、未领过 → 落袋 + 状态标已领 + 回执带槽位。
func TestPrepareAradPassRewardGrantsAndMarksClaimed(t *testing.T) {
	s, c := aradPassTestService(t)
	raw, receipt, err := s.PrepareAradPassReward(Role{State: json.RawMessage(`{}`)}, c, aradpass.TrackNormal, 1)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Item != 590722601 || receipt.Amount != 2 || len(receipt.Slots) != 1 {
		t.Fatalf("receipt = %+v", receipt)
	}
	if got := bagTemplates(t, raw)[590722601]; got != 2 {
		t.Fatalf("产物入包数量 = %d，期望 2", got)
	}
	st, err := aradpass.ReadState(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsClaimed(aradpass.TrackNormal, 1) {
		t.Fatal("领取后必须标记已领")
	}
}

// 重放同一档：必须在**纯计算层**就拒绝（不是靠幂等键兜住）。
func TestPrepareAradPassRewardRejectsAlreadyClaimed(t *testing.T) {
	s, c := aradPassTestService(t)
	first, _, err := s.PrepareAradPassReward(Role{State: json.RawMessage(`{}`)}, c, aradpass.TrackNormal, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.PrepareAradPassReward(Role{State: first}, c, aradpass.TrackNormal, 1); err == nil {
		t.Fatal("同档二次领取必须被拒")
	} else if !strings.Contains(err.Error(), "already claimed") {
		t.Fatalf("错误应说明已领，实际：%v", err)
	}
}

// 等级不够：第 2 档要 **Lv3**（第 3 个参数是等级，不是槽号），0 经验时只有 1 级。
func TestPrepareAradPassRewardRejectsLowLevel(t *testing.T) {
	s, c := aradPassTestService(t)
	if _, _, err := s.PrepareAradPassReward(Role{State: json.RawMessage(`{}`)}, c, aradpass.TrackNormal, 3); err == nil {
		t.Fatal("等级不足必须被拒")
	} else if !strings.Contains(err.Error(), "below the reward level") {
		t.Fatalf("错误应说明等级不足，实际：%v", err)
	}
}

// 皇家轨：没买通行证不能领；买了就能领。
func TestPrepareAradPassRewardGatesRoyalTrack(t *testing.T) {
	s, c := aradPassTestService(t)
	if _, _, err := s.PrepareAradPassReward(Role{State: json.RawMessage(`{}`)}, c, aradpass.TrackRoyal, 1); err == nil {
		t.Fatal("未持有皇家通行证时必须拒绝")
	} else if !strings.Contains(err.Error(), "royal track is not owned") {
		t.Fatalf("错误应说明皇家轨未拥有，实际：%v", err)
	}
	raw, _, err := s.PrepareAradPassActivation(Role{State: json.RawMessage(`{}`)}, c)
	if err != nil {
		t.Fatal(err)
	}
	if _, receipt, err := s.PrepareAradPassReward(Role{State: raw}, c, aradpass.TrackRoyal, 1); err != nil {
		t.Fatalf("激活后应可领皇家轨：%v", err)
	} else if receipt.Item != 590722646 {
		t.Fatalf("皇家轨产物号 = %d", receipt.Item)
	}
}

// 激活：置 Royal + 加购票经验 + 发首发奖励，且**只发一次**。
func TestPrepareAradPassActivationGrantsFirstRewardsOnce(t *testing.T) {
	s, c := aradPassTestService(t)
	raw, receipt, err := s.PrepareAradPassActivation(Role{State: json.RawMessage(`{}`)}, c)
	if err != nil {
		t.Fatal(err)
	}
	if !receipt.Royal || receipt.PurchaseExp != 5000 || receipt.Exp != 5000 {
		t.Fatalf("activation receipt = %+v", receipt)
	}
	if len(receipt.Items) != 2 || len(receipt.Slots) != 2 {
		t.Fatalf("首发奖励 = %+v", receipt)
	}
	bag := bagTemplates(t, raw)
	for _, id := range c.RoyalFirstReward {
		if bag[id] != 1 {
			t.Fatalf("首发奖励 %d 数量 = %d，期望 1", id, bag[id])
		}
	}
	st, err := aradpass.ReadState(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Royal || !st.FirstRewards || st.Exp != 5000 {
		t.Fatalf("state = %+v", st)
	}
	// 再激活：Royal 已为真 ⇒ **零写入**，经验不能翻倍。
	again, receipt2, err := s.PrepareAradPassActivation(Role{State: raw}, c)
	if err != nil {
		t.Fatal(err)
	}
	if receipt2.PurchaseExp != 0 || len(receipt2.Items) != 0 {
		t.Fatalf("重复激活不应再发东西：%+v", receipt2)
	}
	if string(again) != string(raw) {
		t.Fatal("重复激活必须是零写入")
	}
}

// 背包满：装备栏用尽时整笔失败（不落袋、不标记已领），由调用方回滚。
func TestPrepareAradPassRewardFailsWhenBagFull(t *testing.T) {
	s, c := aradPassTestService(t)
	bag := inventory.Bag{Version: "ordinary-bag-v1"}
	// 把 [etc] 的 65..120 全部塞成同一种模板（每次 1000，占满 56 格）。
	for i := 0; i < 56; i++ {
		bag.Items = append(bag.Items, inventory.BagItem{Template: 590722601, Amount: 1000})
	}
	raw, err := inventory.SaveBag(json.RawMessage(`{}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.PrepareAradPassReward(Role{State: raw}, c, aradpass.TrackNormal, 1); err == nil {
		t.Fatal("背包/装备栏满时必须失败")
	}
}
