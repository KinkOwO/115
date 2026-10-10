package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
)

// 本文件是**蔚蓝号（征服频道 102）的源驱动翻牌** —— 与沉月湖
// （moon_solo_flow.go / moon_source_config.go）**同一套产出模型**，
// 差别只有两处，且都由参数给出：
//
//	结算层   沉月湖 = 第二层 100004137；蔚蓝号 = 频道 [guide dungeon index]（实机 100004131）
//	信息帧   沉月湖 = N2622（MoonLakeBootstrap115）；蔚蓝号 = N2621（azureMainInfoBody）
//
// 领取事务与三段翻牌（CMD69 滚动 / CMD70 布局 / CMD71 选牌）**完全复用同一批构件**，
// 所以两张单子的行为逐项对齐（next190 §R 的「等效深渊模式」）。
//
// ⚠️ 为什么不能像蔚蓝号原先那样把奖单塞进 `CardPlan`：`CardPlan.Items` 只有 8 格，
// 而源驱动的单子是「固定产物 3 行 + 掷骰产物 ~14 行」⇒ 用 `loot.MoonRewardPlan`
// （`Grants` 变长），领取走 `ClaimMoonReward`（按 run 幂等）—— 这正是沉月湖那条
// **已实机验收**的路径。`FreezeAzureMainCards`（1..4 件随机装备）因此不再被调用。
//
// 与沉月湖的**信息帧差异**只体现在清关阶段值：蔚蓝号 N2621 `[0:4]` 的 4/5 由
// `azureClearInfo()`（CMD1654 的应答）发出，本文件不需要也不该重复发。

// azureFlipConfig 是蔚蓝号的源驱动翻牌装配结果（进程级，只装配一次）。
type azureFlipConfig struct {
	Policy loot.MoonSourcePolicy
	Note   string
}

// azureFlipState 是本局的翻牌事务状态（与 moonSoloState 的 plan/claimed 同义）。
type azureFlipState struct {
	plan    *loot.MoonRewardPlan
	claimed bool
}

// azureFlipPolicy 装配蔚蓝号的翻牌策略。
//
// 分工与 `conquestFlipPolicy` 一致：装配在 cmd 层（要知道源目录与装备目录），
// 掷骰与结算判据在 internal/loot（与发奖同源）。
//
// guide 取该频道的 `[guide dungeon index]`（`channelGuidesFromDirectory`），
// **不写死副本号** —— 蔚蓝号换内容时装配跟着走。
func azureFlipPolicy(svc *loot.Service, dungeons *catalog.DungeonCatalog, guide uint32, booster *catalog.BoosterCatalog) (*azureFlipConfig, error) {
	if svc == nil || dungeons == nil {
		return nil, fmt.Errorf("蔚蓝号源驱动翻牌需要掉落服务与副本目录")
	}
	if guide == 0 {
		return nil, fmt.Errorf("蔚蓝号频道没有 [guide dungeon index]，无法确定结算层")
	}
	settlement, ok := dungeons.Dungeons[guide]
	if !ok {
		return nil, fmt.Errorf("蔚蓝号结算层 %d 不在副本目录里", guide)
	}
	family := conquestFamily(dungeons, conquestFamilyDungeons)
	if len(family) == 0 {
		return nil, fmt.Errorf("征讨族一个成员都没加载到（%v）", conquestFamilyDungeons)
	}
	policy, note, e := conquestFlipPolicy(svc, family, settlement, booster)
	if e != nil {
		return nil, e
	}
	return &azureFlipConfig{Policy: policy, Note: note}, nil
}

// azureFlipActive 报告「本局该不该走源驱动翻牌」。
//
// 判据用**策略自己记的结算层**（`Policy.SettlementDungeon`，由 guide dungeon 装出），
// 不用频道号：蔚蓝号频道里也可能出现别的副本，那些照旧走通用翻牌。
func (w *worldSession) azureFlipActive() bool {
	if w == nil || w.azureFlipCfg == nil || w.activeDungeon == nil {
		return false
	}
	p := w.azureFlipCfg.Policy
	return p.Source != "" && w.activeDungeon.Definition.ID == p.SettlementDungeon
}

// azureRecoverPendingRewards 补发「上一局没领走」的源驱动奖单。
//
// 与月湖 `moonTick` 开头那段是**同一个事务**（`workflow.RecoverMoonRewards` 按事件模型
// `moon-solo-clear-v1` 扫挂起奖单），只是蔚蓝号没有月湖那套单局状态、也不跑 250ms tick
// ——所以挂在「蔚蓝号频道进城」这个既有钩子上（client_entry.go），一次进场补一次。
//
// 触发场景：领奖时背包满 ⇒ `ClaimMoonReward` 返回 `ErrMoonBagFull` ⇒ 奖单留在事件表里。
// 不补这一下，蔚蓝号那次奖励就永远拿不到（月湖那边由 moonTick 兜住）。
func (w *worldSession) azureRecoverPendingRewards() ([]outboundPacket, error) {
	if w == nil || w.azureFlipCfg == nil || w.role.ID == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, e := (&workflow.LootService{Store: w.store, Loot: w.loot}).RecoverMoonRewards(ctx, w.role)
	if saved.ID != 0 {
		w.role = saved
	}
	if e != nil && !errors.Is(e, loot.ErrMoonBagFull) {
		return nil, e
	}
	body, err := w.loot.Bootstrap(workflow.LootRole(w.role))
	if err != nil {
		return nil, err
	}
	return []outboundPacket{{"azure_pending_rewards_restored", 0, 13, body}}, nil
}

// azureFlipOwned 报告「本局的源驱动奖单已就位」——领取侧与卡片侧共用这一条，
// 免得三处各写一遍 run 比对。
func (w *worldSession) azureFlipOwned() bool {
	return w != nil && w.azure.flip.plan != nil && w.activeDungeon != nil &&
		w.azure.flip.plan.Run == w.activeDungeon.RunID
}

// azureFreezeReward 冻结本局奖单（CMD46 那一步）。同一局只冻结一次（幂等）。
//
// 源驱动那一份把「源声明了什么 / 实发什么 / 跳过了什么」记进日志，便于实机对账
// （D4：宁缺勿编 —— 发不出去的要写明原因，而不是静默丢掉）。
func (w *worldSession) azureFreezeReward() error {
	if w.azureFlipOwned() {
		return nil // 本局已经冻结过
	}
	// 换了一局（结算面板上的「再挑战」、重新进本等）：旧单子按 run 认，直接丢掉，
	// 免得下一局拿着上一局的奖单发奖。
	w.azure.flip = azureFlipState{}
	if w.azureFlipCfg == nil || w.activeDungeon == nil {
		return fmt.Errorf("蔚蓝号奖单：翻牌策略或副本会话缺失")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	plan, audit, e := (&workflow.LootService{Store: w.store, Loot: w.loot}).
		FreezeMoonSourceReward(ctx, w.role, w.activeDungeon, w.azureFlipCfg.Policy)
	if e != nil {
		return e
	}
	if audit.Summary != "" {
		log.Printf("蔚蓝号翻牌（源驱动）：%s 角色=%d 挑战=%s", audit.Summary, w.role.ID, w.activeDungeon.RunID)
		for _, sk := range audit.Skipped {
			log.Printf("蔚蓝号翻牌跳过：%s / %s / %s", sk.What, sk.Reason, sk.Detail)
		}
	}
	w.azure.flip.plan = &plan
	return nil
}

// azureClearRewardBody 组蔚蓝号清关奖励的 N35 载荷。
//
// ⚠️ 形态与沉月湖**逐字节同源**（同一个 281B 基座 + `ConquestClearRewardBody115`）：
// 两个副本的 `.dgn` 都带 `[disable clear reward]`，而那个编码器的注释就写着它是给
// 「sourced [disable clear reward]/special card path」用的（客户端读第一组里的 29B 特殊行）。
//
// 官方抓包对照（`F16-s2c.txt` 第 682 帧，id=35 body=616）：行同样是
// `template u32 + value u32 + 21B metadata`，且里面确实有本副本的固定产物
// （10362432×160、10362429×100）—— 与本函数产出的行形态一致。
func (w *worldSession) azureClearRewardBody() ([]byte, error) {
	if w.azure.flip.plan == nil {
		return nil, fmt.Errorf("蔚蓝号清关奖励：奖单尚未冻结")
	}
	rows := w.azure.flip.plan.WireRows()
	if len(rows) == 0 {
		return nil, fmt.Errorf("蔚蓝号清关奖励：源驱动奖单没有产物")
	}
	state := protocol.ConquestClearReward115{}
	state.Present[0] = true
	state.Rewards[0] = rows
	return protocol.ConquestClearRewardBody115(state)
}

// azureClaim 领取本局奖单（CMD71 选牌 / 自动翻牌 / 退场兜底补领）。
//
// 与沉月湖 `moonClaim` 同源：奖单按 run 冻结在事件表里，`ClaimMoonReward` 按
// `moon-grant:<run>` 幂等 —— 重复调用不会重复发奖，只会再回一次应答。
//
// ★ 应答必须带**被翻的那张牌的下标**（`CardSelected(index)`）。2026-10-09 21:1x
// 实机教训：我先前把旧帧 `0100ff0000…` 误读成 `CardSelected(-1)` 并照抄，结果服务端
// 把奖发进背包了、客户端却**没有任何一张牌被翻开**——玩家看到的正是「自动翻牌没了」，
// 只能自己再点一下。旧实现的真身是 `CardSelected(0)`（自动翻牌选第 0 张）。
//
// 「已领」状态同时写进通用的 `w.cardReceipt`：通用那条路就是靠它判断「这一局的牌已经
// 翻过」，`autoPickSettlementCard` 的重复守卫、退场链的兜底补领都读它 —— 用同一个字段
// 才不会出现「服务端认为领过、退出时又补领一次」。
func (w *worldSession) azureClaim(index byte) ([]outboundPacket, error) {
	if !w.azureFlipOwned() {
		return nil, fmt.Errorf("蔚蓝号领奖：本局奖单不存在")
	}
	ack, e := protocol.CardSelected(int(index))
	if e != nil {
		return nil, e
	}
	if w.azure.flip.claimed {
		// 已经领过（自动翻牌先领了 / 玩家又点了一次）：只回选中帧让面板收尾，
		// 不再推背包、也不再走一遍发奖事务。
		return []outboundPacket{{"azure_card_selection_ack", 1, 71, ack}}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, e := (&workflow.LootService{Store: w.store, Loot: w.loot}).
		ClaimMoonReward(ctx, w.role, w.azure.flip.plan.Run)
	if e != nil {
		return nil, e
	}
	w.role = saved
	body, e := w.loot.Bootstrap(workflow.LootRole(saved))
	if e != nil {
		return nil, e
	}
	w.azure.flip.claimed = true
	// 与通用翻牌同一套「已领」标记（`cardSnapshot()` 也读它）。
	w.cardReceipt = &loot.CardReceipt{Plan: loot.CardPlan{Run: w.azure.flip.plan.Run, Source: w.azure.flip.plan.Source}, Index: index}
	return []outboundPacket{{"azure_reward_inventory", 0, 13, body}, {"azure_card_selection_ack", 1, 71, ack}}, nil
}
