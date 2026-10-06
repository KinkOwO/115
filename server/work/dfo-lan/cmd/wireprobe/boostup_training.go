package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"fmt"
	"log"
)

func (w *worldSession) boostStepRequest(ctx context.Context, p []byte, id uint16, raw ...[]byte) ([]outboundPacket, error) {
	if isBoostChallengeRequest(p) {
		return w.boostChallengeRequest(ctx, p, id, raw...)
	}
	claim := id == 680
	r, e := protocol.DecodeEventRequest115(p, claim)
	if e != nil {
		return nil, e
	}
	refuse := func(err error) ([]outboundPacket, error) {
		return []outboundPacket{{"boost_step_refused", 1, id, protocol.EventRefusal115(r.Event, 102, !claim)}}, err
	}
	before, e := boostup.ReadState(w.role.State)
	if e != nil {
		return refuse(e)
	}
	step, e := w.boostStepGate(r, claim, before)
	if e != nil {
		return refuse(e)
	}
	next, receipt, applied, e := (&workflow.LootService{Store: w.store, Loot: w.loot}).BoostStepRequest(ctx, w.role, w.boostup, step, claim)
	if e != nil {
		return refuse(e)
	}
	w.role = next
	// A prior equipment event update can be retried here from durable facts.
	w.reconcileBoostEquipment()
	next = w.role
	var plan []outboundPacket
	if claim {
		// donor 基线在这里发的是「奖励落包落地帧 + 溢出邮寄」：本树的自动开盒
		// 走同一条 inventory.Awarder 发放线，装不下整步回滚、不代发邮件
		// （见 internal/loot/boostup_autobox.go 的约定），所以增量帧就是背包帧。
		bag, err := inventory.ReadBag(next.State)
		if err != nil {
			return nil, err
		}
		packetID := uint16(14)
		body, err := protocol.InventoryUpdate(stackSlotRows(bag, receipt.Slots...))
		if !applied {
			packetID = 13
			body, err = w.loot.Bootstrap(workflow.LootRole(next))
		}
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"boost_step_reward_inventory", 0, packetID, body})
		if len(receipt.Shared) > 0 {
			counts, err := w.store.AccountMaterials(ctx, w.account)
			if err != nil {
				return nil, err
			}
			m, err := inventory.ReadAccountMaterials(counts)
			if err != nil {
				return nil, err
			}
			// donor 基线是三参（材料快照, 变更前角色, 变更后角色）；本树两参版本
			// 从同一份快照取金币，这里前后角色相同，直接去掉重复实参。
			updates, err := accountMaterialRefreshPackets(m, next, false)
			if err != nil {
				return nil, err
			}
			plan = append(plan, updates...)
		}
	}
	status, e := boostTrainingRestore(w.boostup, next)
	if e != nil {
		return nil, e
	}
	after, e := boostup.ReadState(next.State)
	if e != nil {
		return nil, e
	}
	if after.Training.Finished {
		roles, e := w.store.Characters(ctx, w.account)
		if e != nil {
			return nil, e
		}
		roster, e := boostRosterForRoles(roles, next)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"boost_step_roster", 0, 2639, roster})
	}
	// Captured final claim: N2639 graduation roster -> N2638 finished state
	// -> C680 ACK. Ordinary steps have no roster and retain their old order.
	plan = append(plan, outboundPacket{"boost_step_progress", 0, 2638, status})
	var values [10]uint32
	values[0] = uint32(step)
	if after.Training.Finished {
		w.flushBoostGraduationMail()
		if w.characters != nil && len(w.boostup.Challenges) > 0 {
			next, _, err := (&workflow.LootService{Store: w.store, Loot: w.loot}).ReconcileBoostChallenge(ctx, w.role, w.boostup, w.characters.BoostChallengeFacts)
			if err == nil {
				w.role = next
				body, e := loot.BoostChallengeSnapshot(w.boostup, workflow.LootRole(next))
				if e == nil {
					plan = append(plan, outboundPacket{"boost_challenge_enrolled", 0, 2722, body})
				} else {
					log.Printf("boost challenge graduation snapshot role=%d: %v", w.role.ID, e)
				}
			} else {
				log.Printf("boost challenge graduation pending role=%d: %v", w.role.ID, err)
			}
		}
	}
	return append(plan, outboundPacket{"boost_step_ack", 1, id, protocol.EventReply115(boostup.EventID, values)}), nil
}

// boostStepGate 是 662 两条入站帧（680 领奖 / 681 查看引导）共用的判据。
//
// 关卡号取第三包，第三包为空时回落到第二包：实机 680 发的是两包
// `96020000 02000000`，681 发的是 `96020000 03000000 00000000 00000000 00`
// （关卡号仍在第二包，后面补零）。第三关卡住的一半原因就是只读第三包，
// 于是 681 被判成「parameter 缺失」（实机 2026-10-04 09:18:37）。
//
// [area index] 分区只约束领奖：源里每一关分区不同（第三关 area=2），而客户端
// 正是在上一关的分区里点下一关才发出 681；查询只把 Phase 0→1、不发奖励，
// 按分区拦它就是把引导卡死。
func (w *worldSession) boostStepGate(r protocol.EventRequest115, claim bool, st boostup.State) (byte, error) {
	if r.Event != boostup.EventID || r.Parameter > 254 || r.Sub > 254 || w.boostup == nil || w.loot == nil || w.role.ID == 0 {
		return 0, fmt.Errorf("unsupported event request")
	}
	step := byte(r.Parameter)
	if step == 0 {
		step = byte(r.Sub)
	}
	if step == 0 {
		return 0, fmt.Errorf("unsupported event request")
	}
	if w.activeDungeon != nil || w.inTutorial || w.state.Position.Town != w.boostup.Town || int(step) > len(w.boostup.Steps) {
		return 0, fmt.Errorf("boost step requires owned event town")
	}
	if !st.Activated {
		return 0, fmt.Errorf("event character required")
	}
	if claim && !st.Training.Claimed[step] && uint32(w.boostup.Steps[int(step)-1].Area) != w.state.Position.Area {
		return 0, fmt.Errorf("guide belongs to another training area")
	}
	return step, nil
}
