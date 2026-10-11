package loot

import (
	"dfolan/internal/aradpass"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

// AradPassRewardReceipt 是一次战令领奖的回执（同时是幂等凭据）。
//
// Slots 是物品实际落进背包/装备栏的槽位 —— 会话侧靠它发 id14 的行刷新
// （与 boostup 礼包同一条路：`stackSlotRows` + `protocol.InventoryUpdate`）。
type AradPassRewardReceipt struct {
	Track  int      `json:"track"`
	Slot   int      `json:"slot"`
	Level  int      `json:"level"`
	Item   uint32   `json:"item"`
	Amount uint32   `json:"amount"`
	Slots  []uint16 `json:"slots,omitempty"`
}

// AradPassActivationReceipt 是一次皇家通行证激活的结果。
type AradPassActivationReceipt struct {
	// PurchaseExp 是购票赠送的经验（源 `[purchase pass exp]`）；已激活时为 0。
	PurchaseExp uint32 `json:"purchase_exp"`
	// Exp 是激活后的累计经验（会话侧用它判"要不要补发 0x08A1"）。
	Exp uint32 `json:"exp"`
	// Royal 是激活后的皇家轨状态。
	Royal bool `json:"royal"`
	// Items/Slots 是首发奖励（源 `[royalpass first reward]`）实际落地的物品与槽位。
	Items []uint32 `json:"items,omitempty"`
	Slots []uint16 `json:"slots,omitempty"`
}

// PrepareAradPassReward 在锁定行上计算「领奖 → 落背包 → 标记已领」的纯状态迁移。
// 提交/幂等回读在 workflow.LootService.ClaimAradPassReward。
//
// 档位的三个准入条件全部读**持久状态**（不看请求）：未领过、等级已达到该档要求、
// 皇家轨要真的拥有皇家通行证 —— 与源 `GrantAradPassReward` 的口径一致。
//
// 满背包/装备栏满 → 直接失败并整笔回滚，**不消费领取资格**（没有未经实证的邮件兜底）。
// 与 boostup 礼包/训练领奖同一条发放路径（inventory.Awarder），期限哨兵由它统一打。
func (s *Service) PrepareAradPassReward(current Role, c *aradpass.Catalog, track, level int) (json.RawMessage, AradPassRewardReceipt, error) {
	var receipt AradPassRewardReceipt
	if s == nil || c == nil {
		return nil, receipt, fmt.Errorf("arad pass reward requires the source catalog")
	}
	// ★ `level` 是**等级**（客户端在 CMD680 第 3 个字段里给的就是它，见 RewardAtLevel 的注释）。
	row, ok := c.RewardAtLevel(track, level)
	if !ok {
		return nil, receipt, fmt.Errorf("arad pass: no reward for track=%d level=%d", track, level)
	}
	slot := row.Slot
	st, e := aradpass.ReadState(current.State)
	if e != nil {
		return nil, receipt, e
	}
	if st.IsClaimed(track, slot) {
		return nil, receipt, fmt.Errorf("arad pass: reward %d:%d is already claimed", track, slot)
	}
	if level := st.Level(c); level < uint32(row.Level) {
		return nil, receipt, fmt.Errorf("arad pass: level %d is below the reward level %d", level, row.Level)
	}
	if track == aradpass.TrackRoyal && !st.Royal {
		return nil, receipt, fmt.Errorf("arad pass: the royal track is not owned")
	}
	amount := row.Count
	if amount == 0 {
		amount = 1
	}
	awarder := &inventory.Awarder{Catalog: s.Catalog, Rules: s.BagRules, Equipment: s.Equipment}
	raw, rec, e := awarder.Grant(current.State, row.Item, amount)
	if e != nil {
		return nil, AradPassRewardReceipt{}, e
	}
	raw, e = aradpass.WriteState(raw, st.Claim(track, slot))
	if e != nil {
		return nil, AradPassRewardReceipt{}, e
	}
	receipt = AradPassRewardReceipt{
		Track: track, Slot: slot, Level: row.Level, Item: row.Item, Amount: amount, Slots: rec.Slots,
	}
	return raw, receipt, nil
}

// PrepareAradPassItemGrant 是「账号级一次性发放通行证物品」的纯状态迁移。
//
// 业主口径（2026-10-11）：战令是**账号维度**的 —— 账号第一次解锁时给**第一个登录的角色**
// 发通行证物品，之后不再发。落地的机制是 `database.Store.ApplyGrant`（`grant_id` 主键
// 就是幂等键，重复调用读回原回执、不再发货），所以**不需要新表**。
//
// 这里只做"把物品放进背包"：置 Royal / 加购票经验 / 发首发奖励由随后的
// `PrepareAradPassActivation` 负责（同一笔进城流程里的独立事务，见 aradpass_flow.go）。
func (s *Service) PrepareAradPassItemGrant(current Role, template, amount uint32) (json.RawMessage, json.RawMessage, error) {
	if s == nil {
		return nil, nil, fmt.Errorf("arad pass pass-item grant requires the loot catalog")
	}
	if template == 0 || amount == 0 {
		return nil, nil, fmt.Errorf("arad pass pass-item grant needs a template and an amount")
	}
	awarder := &inventory.Awarder{Catalog: s.Catalog, Rules: s.BagRules, Equipment: s.Equipment}
	raw, rec, e := awarder.Grant(current.State, template, amount)
	if e != nil {
		return nil, nil, e
	}
	receipt, e := json.Marshal(map[string]any{
		"kind": "arad-pass-item", "template": template, "amount": amount, "slots": rec.Slots,
	})
	if e != nil {
		return nil, nil, e
	}
	return raw, receipt, nil
}

// AradPassRoyalDailyReceipt 是「Royal Pass 每日登录礼包」的收据。
//
// 回执表（character_events.outcome）是**唯一**回执来源：重放路径不会执行闭包。
type AradPassRoyalDailyReceipt struct {
	Kind   string   `json:"kind"`
	Day    string   `json:"day"`
	Items  []uint32 `json:"items,omitempty"`
	Slots  []uint16 `json:"slots,omitempty"`
	Repeat bool     `json:"repeat,omitempty"`
}

// PrepareAradPassRoyalDaily 发当日份的「Royal Pass 每日登录礼包」并记上换日键。
//
// 口径：面板写「Once a day / Your Max Royal Pass Character Selection count (1) determines
// your rewards / Reset daily at 9:00 UTC」。本机只登记** 1 个**皇家通行证角色
// （0x08A2 只写当前角色），所以每次就是**一份**源 `[daily reward list]`。
func (s *Service) PrepareAradPassRoyalDaily(current Role, c *aradpass.Catalog, day string) (json.RawMessage, json.RawMessage, error) {
	var out AradPassRoyalDailyReceipt
	if s == nil || c == nil {
		return nil, nil, fmt.Errorf("arad pass royal daily requires the source catalog")
	}
	if len(c.DailyReward) == 0 {
		return nil, nil, fmt.Errorf("arad pass: [daily reward list] is empty")
	}
	awarder := &inventory.Awarder{Catalog: s.Catalog, Rules: s.BagRules, Equipment: s.Equipment}
	state := current.State
	for _, template := range c.DailyReward {
		raw, rec, e := awarder.Grant(state, template, 1)
		if e != nil {
			return nil, nil, e
		}
		state = raw
		out.Items = append(out.Items, template)
		out.Slots = append(out.Slots, rec.Slots...)
	}
	st, e := aradpass.ReadState(state)
	if e != nil {
		return nil, nil, e
	}
	st.RoyalDailyKey = day
	raw, e := aradpass.WriteState(state, st)
	if e != nil {
		return nil, nil, e
	}
	out.Kind, out.Day = "arad-pass-royal-daily", day
	receipt, e := json.Marshal(out)
	if e != nil {
		return nil, nil, e
	}
	return raw, receipt, nil
}

// PrepareAradPassActivation 在锁定行上计算「激活皇家轨 → 发首发奖励」的纯状态迁移。
//
// 源里通行证是**点券商城商品**（`[pass item index]`，`[item category] event`），
// 本仓库的点券商城是 PVF 直读、并没有上架它，购买报文也没有实证。所以按
// **"拥有即激活"** 收口（见 cmd/wireprobe 的 `ensureAradPassActivation`）：
// 只要背包里出现那件物品就置 Royal + 加购票赠送经验 + 发源里的首发 4 件。
//
// 幂等由 workflow 侧的 `arad-pass-activate-v1` 键保证：同键重发不会再进本函数。
// 但状态里 Royal 已为真（例如存档被直接改过、收据行缺失）时这里**零写入**返回，
// 免得把购票经验再加一次。
func (s *Service) PrepareAradPassActivation(current Role, c *aradpass.Catalog) (json.RawMessage, AradPassActivationReceipt, error) {
	var receipt AradPassActivationReceipt
	if s == nil || c == nil {
		return nil, receipt, fmt.Errorf("arad pass activation requires the source catalog")
	}
	st, e := aradpass.ReadState(current.State)
	if e != nil {
		return nil, receipt, e
	}
	if st.Royal {
		// 已激活：零写入，只回自洽状态（会话侧据此判"无需重推"）。
		receipt.Exp, receipt.Royal = st.Exp, true
		return current.State, receipt, nil
	}
	raw := current.State
	awarder := &inventory.Awarder{Catalog: s.Catalog, Rules: s.BagRules, Equipment: s.Equipment}
	if !st.FirstRewards {
		for _, id := range c.RoyalFirstReward {
			var rec inventory.AwardReceipt
			raw, rec, e = awarder.Grant(raw, id, 1)
			if e != nil {
				return nil, AradPassActivationReceipt{}, e
			}
			receipt.Items = append(receipt.Items, id)
			receipt.Slots = append(receipt.Slots, rec.Slots...)
		}
		st.FirstRewards = true
	}
	st = st.Activate(c)
	if raw, e = aradpass.WriteState(raw, st); e != nil {
		return nil, AradPassActivationReceipt{}, e
	}
	receipt.PurchaseExp, receipt.Exp, receipt.Royal = c.PurchaseExp, st.Exp, st.Royal
	return raw, receipt, nil
}
