package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/database"
	"dfolan/internal/workflow"
	"encoding/hex"
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

func boostGiftAvailability(c *boostup.Catalog, role database.Character) ([]byte, error) {
	if c == nil {
		return nil, nil
	}
	st, e := boostup.ReadState(role.State)
	if e != nil {
		return nil, e
	}
	var rows []protocol.EventGiftState115
	for _, g := range c.Gifts {
		rows = append(rows, protocol.EventGiftState115{Gift: g.ID, Claimed: st.Gifts[g.ID]})
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
			v.Mode = 0
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
