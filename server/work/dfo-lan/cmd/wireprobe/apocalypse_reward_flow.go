package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// apocalypseRunNonce 生成 N31 的 5B 运行期 nonce。客户端不解析具体值，
// 只要求与 N2252 尾的阶段 token 配套；由 run id + 阶段号确定性派生，
// 便于同一场重发时保持一致。
func apocalypseRunNonce(runID string, stage int) [5]byte {
	var out [5]byte
	sum := uint32(2166136261)
	for i := 0; i < len(runID); i++ {
		sum ^= uint32(runID[i])
		sum *= 16777619
	}
	for i := 0; i < 5; i++ {
		out[i] = byte(sum >> uint(8*(i%4)))
		sum = sum*31 + uint32(stage) + uint32(i)
	}
	return out
}

// 末世录终局结算链（N14 → N31 → N2252 → N2 → N2253 → N9）。
//
// 形状与维纳斯 completeVenusStage 同族（军团共用 N2252/N2253），链序按
// US-Local 会话 20261008_105231 的 events 与抓包 20261008-105227 的帧序对齐：
//
//	apocalypse_committed_reward_inventory (N14) → apocalypse_source_clear_outcome (N2895)
//	→ apocalypse_basic_clear_reward (N2252) → apocalypse_additional_clear_reward (N2253)
//
// 抓包里 N2252 之前没有 N31（US-Local 的末世录段也没有），所以本实现不发 N31，
// 只用 N2252 尾 @7760 的阶段 token；`dungeon_clear_enabled` 这个包名因此不用
// ——它会被 dispatch 的发送监视器当成通用通关横幅，在军团内容里会多出一帧。
//
// 奖励入库与展示同源：ApocalypseRewardFor 的 Grant 列表既是入库清单也是
// N2252 的展示行，不存在「显示了但没发」或「发了但没显示」。

// apocalypseGrant 收集一次终局结算的结果，供日志与测试直接断言。
type apocalypseGrant struct {
	Plan    []outboundPacket
	Events  []map[string]any
	Granted []legion.ApocalypseRewardItem
	Failed  []string
}

// apocalypseSettlementDelay 是清关与翻牌链之间的延迟。
//
// ★ 2026-10-08 修正：这里原本是 **400ms**，注释理由是「让客户端的清关演出先起步」，
// 但那是本实现自己加的，**没有依据**，而且会让客户端错过 N2252 的解析时机：
//
//   - 参考抓包（D:\zhuabao\captures\20261008-105227）：96.349s CMD39（终局 BOSS 死）
//     → 96.390s N2252，**41ms**；
//   - 2 号实录（能正常显示横幅与翻牌的对照）：
//     08:07:54 monster_death → 08:07:54 N37 EXP → N14 → N2895 → N2252，**同一秒内**。
//
// 也就是说 N2252 必须**紧跟 BOSS 死亡的结算帧立刻发出**，不能延迟到一个单独的
// 定时器里去（那会把 N2252 排到客户端已经处理完清关序列之后）。
// 这里取 0：由连接定时器的下一个 tick 立即发出。
const apocalypseSettlementDelay = 0

// apocalypseSettlement 是一次挂起的末世录终局结算。
//
// 抓包时序（2026-10-08 两场一致）：清关帧（N2895）→ 约 30ms 后 N2252 翻牌
// → N2253。服务端在 boss 死亡那一刻就把奖励算好，但发链留一小段延迟，
// 让客户端的清关演出先走完（发太早会让翻牌界面与死亡演出抢帧）。
type apocalypseSettlement struct {
	at      time.Time
	ackID   uint16
	ackName string
	// sent 置位后本任务作废（客户端提前发 CMD2046 时会走快路径）。
	sent bool
}

// armApocalypseSettlement 登记一次延迟结算；已有挂起任务时不覆盖
// （重复的清关信号不该排两次），返回 false 表示这次没排新的。
func (w *worldSession) armApocalypseSettlement(id uint16, name string, delay time.Duration) bool {
	if w == nil || w.apocalypse == nil || w.apocalypse.Rewarded {
		return false
	}
	if w.apocalypsePending != nil && !w.apocalypsePending.sent {
		return false
	}
	w.apocalypsePending = &apocalypseSettlement{
		at:      time.Now().Add(delay),
		ackID:   id,
		ackName: name,
	}
	return true
}

// apocalypseSettlementDue 在到期时执行挂起的结算，返回要发的帧。
// 未到期或没有挂起任务时返回 nil。
func (w *worldSession) apocalypseSettlementDue(now time.Time) ([]outboundPacket, []map[string]any) {
	pending := w.apocalypsePending
	if pending == nil || pending.sent || now.Before(pending.at) {
		return nil, nil
	}
	pending.sent = true
	grant, err := w.apocalypseTerminalReward(pending.ackID, pending.ackName)
	if err != nil {
		return nil, []map[string]any{{
			"kind":   "apocalypse_reward_refused",
			"reason": err.Error(),
			"id":     pending.ackID,
		}}
	}
	w.apocalypseGrant = &grant
	return grant.Plan, grant.Events
}

// apocalypseTerminalReward 组装并执行末世录终局奖励链。
//
// 幂等：以 run.ID 为事件键做一次性的发放提交，重复调用不会二次发放
// （Rewarded 标记 + CommitCharacterEvent 的键去重，两道）。
func (w *worldSession) apocalypseTerminalReward(id uint16, name string) (apocalypseGrant, error) {
	var out apocalypseGrant
	run := w.apocalypse
	if run == nil {
		return out, nil
	}
	if run.Rewarded {
		// 已经结算过：再来的清关信号只记一条事件，不再发奖励。
		out.Events = append(out.Events, map[string]any{
			"kind":   "apocalypse_reward_skipped",
			"reason": "this run was already paid",
			"run":    run.ID,
		})
		return out, nil
	}
	table, err := legion.ApocalypseRewardFor(run.Choice)
	if err != nil {
		return out, err
	}
	stage := run.Stage
	if stage < 1 {
		stage = 1
	}
	token, err := legion.ApocalypseStageToken(stage)
	if err != nil {
		return out, err
	}

	var plan []outboundPacket

	// 随机装备行：抓包里固定材料之后是 7~8 行装备（1001/1002/1003 开头）。
	//
	// ★ 池子改用**末世录专用 115 级翻牌池**（业主 2026-10-08 方案 A）：
	// `configs/apocalypse-flip-gear.generated.json`，由 `internal/toolcmd/flippool`
	// 现场枚举内层 PVF 的 `list/equipment.lst` 生成，按 rarity 分三档：
	//
	//	rarity 2 = 魔法（50 件）  15%
	//	rarity 3 = 神器（169 件） 35%
	//	rarity 4 = 史诗（241 件） 50%
	//
	// 为什么不再用 `w.loot.Equipment.DropPool()`：那是普通掉落池，
	// 高等级装备几乎全落在 `rarity=2` 一档，凑不出业主的三档口径；
	// 而且它按物品等级筛会抽到等级 1~4 的低级装备
	// （实机症状：翻牌开出「磨损的青铜护腰」那类普通物品）。
	// 维纳斯那个池子也只有 rarity 2/3、**缺 SS(史诗)**，所以单建一份。
	//
	// 池子不可用时这一段为空，事件里 roll_gear 为 0，不会假装发过。
	var flipGear []uint32
	gearPoolStats := legion.ApocalypseFlipGearPoolStats(apocalypseFlipGearPool)
	if len(apocalypseFlipGearPool.Tiers) > 0 {
		slots := legion.ApocalypseFlipGearSlots(run.Choice)
		flipGear = legion.ApocalypseRollFlipGearTiers(apocalypseFlipGearPool, slots)
	}
	// 第一件 = 源表 [reward data] 里本难度的 basic 奖励（也是抓包 N2252 头部那件）。
	// 展示与发放同源：N2252 的行就是 Grant 列表（首件单独占位，避免重复）+ 随机装备。
	var proof uint32
	if len(table.Grant) > 0 {
		proof = table.Grant[0].Template
	}
	items := table.Grant
	if len(items) > 0 && items[0].Template == proof {
		items = items[1:]
	}
	// 通关耗时毫秒：结算时刻 − 本局第一次进图时刻（规格 2252 的 @7760）。
	// 写 0 时客户端把「通关时间」显示成 EMPTY，并连带不渲染奖励行。
	var elapsedMS uint64
	if !run.RunStartedAt.IsZero() {
		if d := time.Since(run.RunStartedAt); d > 0 {
			elapsedMS = uint64(d / time.Millisecond)
		}
	}
	basic, err := legion.ApocalypseBasicClearReward(proof, items, flipGear, elapsedMS)
	if err != nil {
		return out, err
	}
	additional, err := legion.ApocalypseAdditionalClearReward()
	if err != nil {
		return out, err
	}
	n31, err := legion.ApocalypseDungeonClearEnabled(stage, apocalypseRunNonce(run.ID, stage))
	if err != nil {
		return out, err
	}

	// 发放：Awarder 逐件入库，失败只记日志并保留在 Failed 里，不谎报成功。
	if w.loot != nil {
		awarder := &inventory.Awarder{
			Catalog:   w.loot.Catalog,
			Rules:     w.loot.BagRules,
			Equipment: w.loot.Equipment,
		}
		before, _ := inventory.ReadBag(w.role.State)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		for _, item := range table.Grant {
			updated, _, gErr := awarder.Grant(w.role.State, item.Template, item.Amount)
			if gErr != nil {
				out.Failed = append(out.Failed, fmt.Sprintf("template=%d amount=%d error=%v", item.Template, item.Amount, gErr))
				log.Printf("apocalypse terminal award failed: template=%d amount=%d error=%v", item.Template, item.Amount, gErr)
				continue
			}
			w.role.State = updated
			out.Granted = append(out.Granted, item)
		}
		// 随机装备行：与固定项同样走 Awarder 入库（装备数量恒为 1），失败只记日志。
		for _, g := range flipGear {
			updated, _, gErr := awarder.Grant(w.role.State, g, 1)
			if gErr != nil {
				out.Failed = append(out.Failed, fmt.Sprintf("gear template=%d error=%v", g, gErr))
				log.Printf("apocalypse flip gear award failed: template=%d error=%v", g, gErr)
				continue
			}
			w.role.State = updated
			out.Granted = append(out.Granted, legion.ApocalypseRewardItem{Template: g, Amount: 1})
		}
		if w.store != nil {
			key := "apocalypse-terminal:" + run.ID
			_, _, _ = w.store.CommitCharacterEvent(ctx, w.account, w.role.ID, w.role.ConfigVersion,
				key, "apocalypse-terminal-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
					proofBody, _ := json.Marshal(map[string]any{
						"run":       run.ID,
						"choice":    run.Choice,
						"stage":     stage,
						"operation": apocalypseOperationIndex(run.Choice),
						"items":     out.Granted,
					})
					return w.role.State, proofBody, nil
				})
		}
		cancel()
		// N14 物品台账刷新：让客户端立刻看到奖励进包（军团链首帧）。
		if after, rErr := inventory.ReadBag(w.role.State); rErr == nil {
			if updatePayload, uErr := protocol.InventoryUpdate(inventory.ChangedItemRows(before, after)); uErr == nil {
				plan = append(plan, outboundPacket{
					"apocalypse_committed_reward_inventory", 0, 14, updatePayload,
				})
			}
		}
	}
	// ★ 冻结本局的翻牌卡组（与 freezeVenusCards 同款）：内容 = N2252 的奖励行。
	//
	// 不冻结的话客户端渲染出一张**空白的翻牌界面**（业主 2026-10-08：
	// 「翻牌还是一样，看不到物品」「维纳斯有修改」）—— 通用 PlanCards 生成的是
	// 普通地下城的金币卡组，与军团翻牌不是同一套。
	{
		fctx, fcancel := context.WithTimeout(context.Background(), 5*time.Second)
		if ferr := w.freezeApocalypseCards(fctx, table, flipGear); ferr != nil {
			out.Events = append(out.Events, map[string]any{
				"kind":   "apocalypse_card_freeze_failed",
				"reason": ferr.Error(),
			})
		}
		fcancel()
	}
	run.Rewarded = true

	// N2895 清关态（State2/全阶段 marks/角色保留），与 N2252 同刻发出。
	plan = append(plan, outboundPacket{
		"apocalypse_source_clear_outcome", 0, legion.NotiLegionInfo,
		// Stage 保持「已到达的最大阶段」= 6（五关全清），marks 全置。
		apocalypseStageClearInfo(run),
	})
	// N31 通关横幅（阶段 token；本实现不依赖它，但军团家族客户端用它开翻牌）。
	plan = append(plan, outboundPacket{
		"apocalypse_stage_clear_enabled", 0, 31, n31,
	})
	// ★ 终局奖励门的前置 N2895 / **Outcome0**（规格 G0454：「N2252 之前以
	// Outcome0 安装」；2252 规格：「生产在本包之前增加 N2895 终点投影/Outcome0」）。
	// Stage 被投影到该难度的**固定终点**（第一档 3、第二档 5），不是本次实际关数。
	plan = append(plan, outboundPacket{
		"apocalypse_terminal_endpoint_outcome0", 0, legion.NotiLegionInfo,
		apocalypseTerminalEndpointInfo(run, 0),
	})
	plan = append(plan, outboundPacket{
		"apocalypse_basic_clear_reward", 0, legion.NotiClearRewardBasic, basic,
	})
	if w.characters != nil {
		if info, e := w.characters.EntryBasicProbe(w.role, w.characters.ChannelContext); e == nil {
			plan = append(plan, outboundPacket{"apocalypse_settlement_character_info", 0, 2, info})
		}
		if detail, e := w.characters.EntryAddition(w.role); e == nil {
			plan = append(plan, outboundPacket{"apocalypse_settlement_character_detail", 0, 2, detail})
		}
	}
	plan = append(plan, outboundPacket{
		"apocalypse_additional_clear_reward", 0, legion.NotiClearRewardAdditional, additional,
	})
	// ★ 终局 N2895 / **Outcome3**（规格 2253：「apocalypseResultPackets **最后
	// 追加 N2895 Outcome3**。完成状态只在奖励展示批次生成」；G0396 补充：
	// 「其后增加 2253 无额外奖品分支，刷新并打开窗口642，最后发布 N2895 Outcome3」）。
	//
	// **这一帧是通关演出/结算窗的触发点** —— 本实现此前只发 N2252/N2253，
	// 没有这最后一帧，所以客户端不进终局演出。
	plan = append(plan, outboundPacket{
		"apocalypse_terminal_endpoint_outcome3", 0, legion.NotiLegionInfo,
		apocalypseTerminalEndpointInfo(run, 3),
	})
	// ★ 终局 **State3 + Outcome1 = 触发通关视频** —— 但**不能在这里发**！
	//
	// 家族约定（维纳斯 `VenusFinalInfo`）说它「Sent with the terminal CMD2046 ACK
	// — the client then plays the clear movie」，而本实现的 N2252/N2253 是**紧跟
	// BOSS 死亡**发出的（对齐 2 号抓包），此时玩家还没翻牌。曾经在这里发，
	// 实机后果是**演出把翻牌抢掉**：
	//
	//	业主 2026-10-08 19:35：「通关后直接出现通关视频，中间漏掉了横幅、翻牌，
	//	通关视频应该是在翻牌后才出现」
	//
	// 所以改由 `readyApocalypseClearMovie` 在**翻牌结束**（玩家点牌或自动选牌）
	// 之后发出，见那里的注释。leave 态（State5）则等客户端在演出中发
	// CMD191(state=1)，由 `apocalypseStoryPause` 应答。
	// ★ 冒险团三帧（N1337 经验 / N1331 资料 / N2425 图鉴）：2 号权威抓包
	// （D:\zhuabao\captures\20261008-184805）在 N2253 之后 1.4 秒发它们，
	// 驱动结算面板顶部的「更新账号纪录! **01分20秒**」与装备/星蕴石清单。
	//
	// 本实现此前**从不发**这三帧 —— `refreshAdventure` 只在连接 ticker 里被调，
	// 且被资料快照签名去重；翻牌奖励只进背包、不改冒险团资料 ⇒ 签名不变 ⇒
	// 三帧被静默跳过。这里改用无条件版本（forceAdventureRefresh）。
	{
		advCtx, advCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if advPackets, advErr := w.forceAdventureRefresh(advCtx); advErr != nil {
			out.Events = append(out.Events, map[string]any{
				"kind":   "apocalypse_adventure_refresh_failed",
				"reason": advErr.Error(),
			})
		} else {
			plan = append(plan, advPackets...)
		}
		advCancel()
	}
	if party, e := protocol.SoloPartyInfo(w.role.WireID); e == nil {
		plan = append(plan, outboundPacket{"apocalypse_settlement_party_steady", 0, 9, party})
	}

	out.Plan = plan
	out.Events = append(out.Events, map[string]any{
		"kind":                   "apocalypse_full_clear_reward_committed",
		"character_id":           w.role.ID,
		"id":                     id,
		"name":                   name,
		"run":                    run.ID,
		"choice":                 run.Choice,
		"difficulty":             table.Label,
		"operation":              apocalypseOperationIndex(run.Choice),
		"stage":                  stage,
		"token":                  token,
		"basic_items":            len(table.Grant),
		"granted_items":          len(out.Granted),
		"grant_failed":           out.Failed,
		"random_gear_slots":      table.RandomGearSlots,
		"roll_gear_slots":        legion.ApocalypseFlipGearSlots(run.Choice),
		"roll_gear":              flipGear,
		"gear_pool_stats":        gearPoolStats,
		"grant_is_authoritative": table.GrantIsAuthoritative,
		"reward_source":          "contents/system/legionsystem/legionsystem.cos [reward data] + capture 20261008-105227",
	})
	w.apocalypseGrant = &out
	return out, nil
}

// apocalypseRewardItems 组装本次翻牌的全部入库奖励 —— 与 N2252 的展示行**同源**
// （业主口径 2026-10-08：「只有在通关后，翻牌获得，维纳斯有修改」）。
//
// ★ 顺序与 venusRewardItems 一致：**随机装备在前、固定材料在后**。
// 这不是随意排的 —— `loot.CardPlan.Items` 是 `[8]Award` 定长数组，
// `apocalypseCardPlan` 按它截断；若把固定材料排在前面，难度2 的 8 条固定行会
// 正好占满 8 格，把**随机装备整段挤掉**（实测：翻牌界面随机位全空）。
// 维纳斯同样是「flipGear（5）+ 难度材料（3）」在前、第二排固定材料在后。
func apocalypseRewardItems(table legion.ApocalypseRewardTable, gear []uint32) []loot.Award {
	items := make([]loot.Award, 0, len(gear)+len(table.Grant))
	for _, t := range gear {
		items = append(items, loot.Award{Template: t, Amount: 1})
	}
	for _, it := range table.Grant {
		items = append(items, loot.Award{Template: it.Template, Amount: it.Amount})
	}
	return items
}

// apocalypseCardPlan 组装终局翻牌计划（纯函数，便于测试）。
//
// 与 venusCardPlan 同构：`CardPlan.Items` 是 `[8]Award` 定长数组，所以只放
// **前 8 项**（与维纳斯一样按定长截断）——入库清单不依赖这里（走
// apocalypseRewardItems 的全量）。
func apocalypseCardPlan(choice byte, table legion.ApocalypseRewardTable, gear []uint32, runID, source string, level byte) loot.CardPlan {
	plan := loot.CardPlan{Run: runID, Source: source, Model: "apocalypse-terminal-v1", Level: level}
	copy(plan.Items[:], apocalypseRewardItems(table, gear))
	return plan
}

// freezeApocalypseCards 冻结终局翻牌计划，与 freezeVenusCards 同构。
//
// ★ 这是「翻牌看不到物品」的**根本修复**：此前末世录没有冻结自己的卡组，
// 客户端于是渲染出一张物品都没有的翻牌界面（业主 2026-10-08：「翻牌还是一样，
// 看不到物品」「维纳斯有修改」）。维纳斯的做法是**冻结一份自己的卡组**
// （内容 = 第一排翻牌奖励），并用 `"cardplan:"+RunID` 事件 + 回执校验，让
// CMD71 的领取与展示完全一致。
func (w *worldSession) freezeApocalypseCards(ctx context.Context, table legion.ApocalypseRewardTable, gear []uint32) error {
	if w.loot == nil || w.store == nil || w.apocalypse == nil || w.activeDungeon == nil {
		return fmt.Errorf("apocalypse card freeze requires loot and store")
	}
	plan := apocalypseCardPlan(w.apocalypse.Choice, table, gear, w.activeDungeon.RunID,
		w.loot.Catalog.Source.SaveIdentity(), byte(w.activeDungeon.Definition.BasisLevel))
	commit := func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		b, e := json.Marshal(plan)
		return current.State, b, e
	}
	if _, _, err := w.store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, plan.Source,
		"cardplan:"+plan.Run, plan.Model, commit); err != nil {
		return err
	}
	b, err := w.store.CharacterEventReceipt(ctx, w.role.AccountID, w.role.ID, "cardplan:"+plan.Run)
	if err != nil {
		return err
	}
	var stored loot.CardPlan
	if err := json.Unmarshal(b, &stored); err != nil {
		return err
	}
	if stored != plan {
		return fmt.Errorf("apocalypse card plan round-trip mismatch")
	}
	w.cardPlan = &plan
	return nil
}

// apocalypseTerminalEndpointInfo 构造**终局奖励门**的 N2895（规格 G0454 / 2252 / 2253）：
// 把线级 Stage 投影到该难度的固定终点，并带指定的 Outcome。
//
//	Outcome 0 → 在 N2252 **之前**安装（奖励门的前置状态）
//	Outcome 3 → 在 N2253 **之后**追加（完成状态；通关演出/结算窗的触发点）
//
// 注意「固定终点」是**难度终点**（第一档 3、第二档 5），不是本次实际打到的关数；
// 规格 2252 明写「捷径的实际路线序号不能直接当此处终点」。
func apocalypseTerminalEndpointInfo(run *legion.ApocalypseRunState, outcome uint32) []byte {
	endpoint := legion.ApocalypseEndpoint(run.Choice)
	state := legion.ApocalypseWaitingInfo()
	state.OperationChoice = run.Choice
	state.State = 2
	state.Stage = uint32(endpoint)
	// ★ Outcome = 载荷 @9..12（= `LegionInfoState.Outcome`，legion_info.go 写在 b[7]）。
	//
	// 本实现一度把这一格当成 Following（还写过 FollowOverride），**错了**：
	// 2 号权威抓包 D:\zhuabao\captures\20261008-184805 三次采样显示
	//
	//	idx=335  @5..8=02000000  @9..12=00000000  @13..16=00000000  @17..20=ffffffff
	//	idx=640  @5..8=02000000  @9..12=05000000  @13..16=05000000  @17..20=ffffffff
	//	idx=654  @5..8=00000000  @9..12=05000000  @13..16=05000000  @17..20=ffffffff
	//
	// 即 @9 是 Outcome、@17 才是恒为 ffffffff 的 Following。
	//
	// 规格 G0454 要求「N2252 之前以 Outcome0 安装，N2253 之后才 Outcome3」，
	// 所以这里的 outcome 参数就是规格那个字段。
	state.Outcome = outcome
	state.StageMarks = legion.ApocalypseMarksFor(endpoint + 1)
	state.TargetMarks[0] = 1
	if run.RoleSet {
		state.RoleCount = 1
	}
	return legion.ApocalypseInfo(state)
}

// readyApocalypseClearMovie 在**翻牌结束**之后发出终局 State3（触发通关视频）。
//
// 业主口径（2026-10-08 19:35）：「通关后直接出现通关视频，中间漏掉了横幅、翻牌，
// **通关视频应该是在翻牌后才出现**」。
//
// 时序上的约束（决定了为什么不能放在奖励链里）：军团翻牌的 N2252/N2253 必须
// **紧跟 BOSS 死亡**发出（对齐 2 号权威抓包 D:\zhuabao\captures\20261008-184805
// 的 105.85s 那两帧），此时玩家还没翻牌；而家族约定的 State3 一发出客户端就开始
// 播放演出。两者放在同一批里，演出必然抢掉翻牌。
//
// 所以拆成两步：
//
//	N2252/N2253（BOSS 死时）→ 玩家翻牌 → **本函数发 State3** → 演出 → CMD191 → State5
//
// 返回 nil 表示「当前不该发」（未翻牌、已发过、或不在终局）。
func (w *worldSession) readyApocalypseClearMovie() ([]outboundPacket, []map[string]any) {
	if w == nil || w.apocalypse == nil {
		return nil, nil
	}
	run := w.apocalypse
	// 只在「已领奖（翻牌链已发）且尚未触发过演出」时发一次。
	if !run.Rewarded || run.FinalDone {
		return nil, nil
	}
	run.FinalDone = true
	endpoint := legion.ApocalypseEndpoint(run.Choice)
	return []outboundPacket{{
			"apocalypse_terminal_final", 0, legion.NotiLegionInfo,
			legion.ApocalypseInfo(legion.ApocalypseFinalInfo(run.Choice, endpoint)),
		}}, []map[string]any{{
			"kind":         "apocalypse_clear_movie_started",
			"character_id": w.role.ID,
			"choice":       run.Choice,
			"endpoint":     endpoint,
		}}
}

// apocalypseStageClearInfo 是清关时刻的 NOTI2895：Stage 报告**刚清掉的那一间**、
// marks 覆盖到它、目标标记相应置 1。
//
// 2026-10-08 按 2 号实录（roles_persist_..._20261008_160138_795939_next37）校正：
//
//	2 号 apocalypse_source_clear_outcome（07:54，全清）
//	  @9..12  = 05 00 00 00   Following 随阶段递增到 5
//	  @13..16 = 05 00 00 00   Stage = 刚清掉的第 5 关
//	  @128..   = 01 01 01 00… 六个目标标记的第 0 位也是 1
//
// 本实现此前把 @13 写成 run.Stage（= 6，「下一间要载入的」），比 2 号 多 1。
// 参考抓包看不出这个差别，因为参考抓包的清关帧 @13 恰好落在同一形状上。
//
// ⚠️ 这里**只改这一帧**：不动 run.Stage、不动 ContinueStage()、也不动等待态
// 帧 —— 「在第几关撤退、重进就是第几关」依赖的是等待态与 CMD2045 的刻度，
// 业主 2026-10-08 已实测通过，不能连带改坏。
func apocalypseStageClearInfo(run *legion.ApocalypseRunState) []byte {
	state := legion.ApocalypseWaitingInfo()
	state.OperationChoice = run.Choice
	// 报告刚清掉的那一间（run.Stage 是「已进入过的房间数」= 下一间的下标）。
	cleared := run.Stage - 1
	if cleared < 0 {
		cleared = 0
	}
	state.Stage = uint32(cleared)
	state.StageMarks = legion.ApocalypseMarksFor(run.MarkCount())
	state.State = 2
	// Outcome（载荷 @9..12）：清关帧的权威实测是**路线终点**（2 号两次全清都是
	// `05`，与其 Stage=5 一致）。规格 G0454 只规定终局奖励门那两帧用 0/3，
	// 清关帧沿用路线终点。
	state.Outcome = uint32(cleared)

	// 目标标记（载荷 @128+4*i）：2 号实录在**全部 27 条 N2895 里恒为
	// `[1, 0, 0, 0, 0, 0]`**（不随清关进度增长），所以只置第 0 位。
	// 本实现此前从下标 1 开始填（并且当 run.Cleared≥1 时写到高位），
	// 与 2 号 的形状不符。
	state.TargetMarks[0] = 1
	if run.RoleSet {
		state.RoleCount = 1
	}
	return legion.ApocalypseInfo(state)
}
