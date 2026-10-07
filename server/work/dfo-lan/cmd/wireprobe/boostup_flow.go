package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/database"
	"dfolan/internal/savecontract"
	"dfolan/internal/workflow"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// dispatchBoostEvent 登记 662 的两条入站线路：CMD643 领取创建礼盒（面板上的
// 「Get」）、CMD680 领取本关奖励 / CMD681 查看引导。donor 基线写在 main.go 的巨型
// switch 里，本树把派发拆成阶段链后必须显式登记，否则帧落到 unimplemented_sample，
// 玩家看到的就是「点了没反应」（实机 2026-10-04 01:44:28 起 6 次 CMD643，
// 正文 7500010000000000 = 礼盒 117 + 直发标记 1）。
func (client *gameConnection) dispatchBoostEvent(requestData *clientRequest) dispatchAction {
	w := client.worldState
	if requestData.frame.Type != 1 || !requestData.verified || !client.bootstrapped || w == nil || w.boostup == nil {
		return dispatchNext
	}
	if requestData.frame.ID != 643 && requestData.frame.ID != 680 && requestData.frame.ID != 681 {
		return dispatchNext
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var plan []outboundPacket
	var e error
	if requestData.frame.ID == 643 {
		plan, e = w.claimBoostGift(ctx, requestData.plaintext)
	} else {
		plan, e = w.boostStepRequest(ctx, requestData.plaintext, requestData.frame.ID, requestData.frame.Raw)
	}
	if e != nil {
		client.event(map[string]any{"kind": "boost_event_request_refused", "id": requestData.frame.ID,
			"character_id": w.role.ID, "reason": e.Error(), "request_hex": hex.EncodeToString(requestData.plaintext)})
	}
	// 拒绝帧本身就在 plan 里（kind=1 回执），有错也要先把包发出去再收日志。
	if client.sendPlan(plan, client.logWorldResponseBody) != nil {
		return dispatchClose
	}
	return dispatchHandled
}

// sendBoostChannelEvents 下发 NOTI108 活动清单快照（10017/10018/662，挑战开关打开时
// 再加 665）。客户端点活动/礼物图标时按事件 id 查这张表：表里没有 662 就回落到内置的
// 韩文 18 周年通用礼物窗口（实机 2026-10-03 接线前症状）。调用点只有选角名单
// （CMD8 userInfoMode==2，reason=character_select）与回选角（CMD7，reason=return_selection）。
func (client *gameConnection) sendBoostChannelEvents(reason string) error {
	if client.boostEventInfo == nil {
		return nil
	}
	if err := client.output.send(0, 108, client.boostEventInfo); err != nil {
		return err
	}
	client.event(map[string]any{"kind": "channel_open_events_sent", "id": 108, "reason": reason, "bytes": len(client.boostEventInfo)})
	return nil
}

// boostGiftOfferSuppressed 判断本角色是否**不参与** 662 直升活动的礼物报价：
// 奥德赛创建的角色，以及已经达到 [goal level] 的满级角色。
//
// 奥德赛这条在本树 PVF 源里没有任何对应字段（boostup.evt / eventgift.evt 都不提
// 奥德赛），按 §0.2 第 4 条记为业主明确要求的服侧差异：奥德赛有自己的一条直升线，
// 不该再看到普通模式的直升活动弹窗（业主 2026-10-06「奥德赛模式直升活动会弹窗」）。
// 满级这条是源驱动的：[goal level] 就是本活动的终点，已到终点的角色领胶囊无意义
// （业主同一句里的「115级满级角色也会弹窗」）。
//
// 只看角色自身的创建标记（CreatedAsOdyssey），不吃 DFO_ODYSSEY_MODE 的启动器覆盖：
// 弹窗与否由客户端按同一个 per-character 标志（XORSTR "[is arad odyssey user]"）
// 呈现，调试开关不该改变报价。
func boostGiftOfferSuppressed(c *boostup.Catalog, role database.Character) bool {
	if character.CreatedAsOdyssey(role) {
		return true
	}
	if c == nil || c.GoalLevel == 0 {
		return false
	}
	var base struct {
		Level byte `json:"level"`
	}
	return json.Unmarshal(role.State, &base) == nil && base.Level >= c.GoalLevel
}

// boostStorySkipEligible 判断本角色是否要走 662 直升后的主线清除：只有真正吃过胶囊
// 的角色（activated 仅由 loot.UseBoostCapsule 置位）且已达 [goal level]。
// 奥德赛创建的角色不在范围内 —— 它们的主线由奥德赛毕业那条链按自己的源处理，
// 而 662 的报价对它们已经关闭（见 boostGiftOfferSuppressed）。
func boostStorySkipEligible(c *boostup.Catalog, role database.Character) bool {
	if c == nil || c.GoalLevel == 0 || character.CreatedAsOdyssey(role) {
		return false
	}
	st, e := boostup.ReadState(role.State)
	if e != nil || !st.Activated {
		return false
	}
	var base struct {
		Level byte `json:"level"`
	}
	return json.Unmarshal(role.State, &base) == nil && base.Level >= c.GoalLevel
}

// boostStorySkipApply 按 character_events 的 boost-story-skip-v2 收据把该角色的主线
// 批量标为完成（幂等：已有 v2 收据就只读一次，不重复写行；只带 v1 收据的老存档由
// CommitBoostStorySkip 补跑一次）。进城登录钩子和胶囊路径共用
// 这一条，失败只记事件不阻断各自的流程。判据与业主裁决见
// docs/protocol/boostup662-story-skip-20261006.md。
func (w *worldSession) boostStorySkipApply(ctx context.Context, role database.Character, event func(map[string]any)) (int, bool, error) {
	if w == nil || w.quests == nil || !boostStorySkipEligible(w.boostup, role) {
		return 0, false, nil
	}
	count, applied, e := w.quests.BoostStorySkip(ctx, role, w.boostup.GoalLevel)
	if event != nil {
		evt := map[string]any{"kind": "boost_story_skip", "character_id": role.ID, "count": count, "applied": applied}
		if e != nil {
			evt["reason"] = e.Error()
		}
		event(evt)
	}
	if e != nil {
		return count, applied, e
	}
	return count, applied, w.boostStorySkipTickets(ctx, role, event)
}

// boostStorySkipTicketEvent 是「按源 [quest clear item] 补发清券」的幂等键；
// 与主线清除的 boost-story-skip-v2 是两张独立收据，谁失败谁下次进城重试。
// 券只需三张一次，v1→v2 放宽主线范围时不跟着改名，避免二次发放。
const boostStorySkipTicketEvent = "boost-story-skip-tickets-v1"

// boostStorySkipTickets 把源里 [capsule info] 的 [quest clear item] 三张券以系统邮件
// 附件补发一次。券清的是源里 [grade] [side] 的三条墙任务（60/65/90 级，模板
// 10327301/10327302/10327303 各自的 [any quest clear]），和上面批量完成的 epic
// 主线不是同一批任务，所以两边都要发。邮箱满 ⇒ CommitSystemMail 报错、收据不落。
func (w *worldSession) boostStorySkipTickets(ctx context.Context, role database.Character, event func(map[string]any)) error {
	if w.store == nil || w.boostup == nil || len(w.boostup.QuestClearItems) == 0 {
		return nil
	}
	grants := make([]database.GrantItem, 0, len(w.boostup.QuestClearItems))
	for _, template := range w.boostup.QuestClearItems {
		grants = append(grants, database.GrantItem{Template: template, Amount: 1})
	}
	assets, e := database.SystemMailAssets(0, grants)
	if e != nil {
		return e
	}
	_, applied, e := w.store.CommitSystemMail(ctx, role.AccountID, role.ID, savecontract.Identity(),
		boostStorySkipTicketEvent, "system-mail-v1", "Starter Boost",
		"Quest clear tickets from your Starter Boost capsule. Use them on the story walls they match.", assets)
	if event != nil {
		evt := map[string]any{"kind": "boost_story_skip_tickets", "character_id": role.ID,
			"applied": applied, "items": len(grants)}
		if e != nil {
			evt["reason"] = e.Error()
		}
		event(evt)
	}
	return e
}

// boostStorySkipNow 是胶囊落地后的那一步：调用点必须已经把 w.role 换成升完级的角色，
// 新写入收据时紧接着重发任务手册三连（291/342/21），让手册立刻反映清除结果。
func (w *worldSession) boostStorySkipNow(ctx context.Context, event func(map[string]any)) ([]outboundPacket, error) {
	_, applied, e := w.boostStorySkipApply(ctx, w.role, event)
	if e != nil || !applied {
		return nil, e
	}
	return w.actQuestRefresh(ctx)
}

// boostGiftAvailability 下发 NOTI2265（EVENT_GIFT_USER_AVAILABILITY）：每个礼盒一行
// {u16 礼盒号, u8 已处理}。不参与本活动的角色（见 boostGiftOfferSuppressed）把**所有**
// 行标成已处理，而不是省略这一帧或省略活动行——客户端 662 首登弹窗谓词
// （IDA：sub_144D4ACC0 按 [first login open popup] 把礼盒登记进首登弹窗表
// sub_144D4B8E0，谓词 sub_144D4AA80 读的就是这条 2265 写进 manager+448 的可用性树）
// 只在「本礼盒值 == 1 且它 [link gift index] 指向的礼盒也全部 == 1」时返回不弹；
// 行缺失按 0 处理 ⇒ 照弹。官服那条 2265 只带 {118:1}，用的就是同一个「值 1 = 不再提示」
// 口径，所以这里对齐官服而不是自造新帧。
func boostGiftAvailability(c *boostup.Catalog, role database.Character) ([]byte, error) {
	if c == nil {
		return nil, nil
	}
	st, e := boostup.ReadState(role.State)
	if e != nil {
		return nil, e
	}
	suppress := boostGiftOfferSuppressed(c, role)
	var rows []protocol.EventGiftState115
	for _, g := range c.Gifts {
		rows = append(rows, protocol.EventGiftState115{Gift: g.ID, Claimed: suppress || st.Gifts[g.ID]})
	}
	return protocol.EventGiftStates115(rows)
}
func boostTrainingRestore(c *boostup.Catalog, role database.Character) ([]byte, error) {
	if c == nil {
		return nil, nil
	}
	st, e := boostup.ReadState(role.State)
	if e != nil {
		return nil, e
	}
	v := protocol.BoostTrainingState115{Mode: 2}
	if st.Activated {
		v.Active = true
		v.Step = st.Training.Step
		v.Phase = st.Training.Phase
		if !st.Training.Finished {
			// 客户端把这一字节当轨道号（`mode == 1` = 奶系轨）用来选训练副本路线，
			// 恒发 0 会让奶系角色精确匹配失败并回落到普通轨 `[dungeon index]`。
			v.Mode = byte(st.Variant)
		}
	}
	return protocol.BoostTrainingStatus115(v)
}

// 训练关卡在技能事务里完成时（第三关技能进化点）不会自己带进度帧：这里比较
// 本次事务前后的训练状态，只在确实前进时补发一条 NOTI2638，客户端的任务面板
// 才会刷新。重复下发同一帧会让已完成的引导重新弹出来。
func boostTrainingProgress(c *boostup.Catalog, before, after database.Character) []outboundPacket {
	return boostMissionProgress("boost_skill_mission_progress", c, before, after)
}

// boostMissionProgress 是各训练关卡完成钩子共用的补帧口径（技能关、分解关……）：
// 只有**本次事务真的推进了 Step/Phase/Finished** 才下发 2638。
func boostMissionProgress(name string, c *boostup.Catalog, before, after database.Character) []outboundPacket {
	if c == nil || before.ID == 0 || after.ID == 0 || string(before.State) == string(after.State) {
		return nil
	}
	a, e := boostup.ReadState(before.State)
	if e != nil {
		return nil
	}
	b, e := boostup.ReadState(after.State)
	if e != nil {
		return nil
	}
	if a.Training.Step == b.Training.Step && a.Training.Phase == b.Training.Phase && a.Training.Finished == b.Training.Finished {
		return nil
	}
	body, e := boostTrainingRestore(c, after)
	if e != nil {
		return nil
	}
	return []outboundPacket{{name, 0, 2638, body}}
}

func (w *worldSession) claimBoostGift(ctx context.Context, p []byte) ([]outboundPacket, error) {
	r, e := protocol.DecodeEventGiftRequest115(p)
	if e != nil {
		return nil, e
	}
	refuse := func(code uint16, err error) ([]outboundPacket, error) {
		return []outboundPacket{{"boost_gift_refused", 1, 643, protocol.Refusal(code)}}, err
	}
	if w.boostup == nil || w.role.ID == 0 || w.loot == nil {
		return refuse(19, fmt.Errorf("boost event is not enabled"))
	}
	if w.activeDungeon != nil || w.inTutorial || w.selectingDungeon {
		return refuse(23, fmt.Errorf("gift requires selected character in town"))
	}
	var gift *boostup.Gift
	for i := range w.boostup.Gifts {
		if w.boostup.Gifts[i].ID == r.Gift {
			gift = &w.boostup.Gifts[i]
			break
		}
	}
	if gift == nil || !gift.Direct || r.Direct != 1 {
		return refuse(23, fmt.Errorf("gift/source delivery mismatch"))
	}
	if gift.Event != 10017 && gift.Event != 10018 {
		return refuse(19, fmt.Errorf("gift operational event is not enabled"))
	}
	role, receipt, applied, e := (&workflow.LootService{Store: w.store, Loot: w.loot}).ClaimBoostGift(ctx, w.role, *gift)
	if e != nil {
		return refuse(23, e)
	}
	w.role = role
	// Replays send CURRENT inventory, not the historical grant's slot rows.
	// This avoids recreating a capsule that was consumed after a lost ACK.
	bag, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, e
	}
	id := uint16(14)
	rows, e := protocol.InventoryUpdate(stackSlotRows(bag, receipt.Slots...))
	if !applied {
		id = 13
		rows, e = w.loot.Bootstrap(workflow.LootRole(role))
	}
	if e != nil {
		return nil, e
	}
	states, e := boostGiftAvailability(w.boostup, role)
	if e != nil {
		return nil, e
	}
	return []outboundPacket{{"boost_gift_inventory", 0, id, rows}, {"boost_gift_states", 0, 2265, states}, {"boost_gift_claimed", 1, 643, protocol.EventGiftReply115(r.Gift, 1)}}, nil
}
