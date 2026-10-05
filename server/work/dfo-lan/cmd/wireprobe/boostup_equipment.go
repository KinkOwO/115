package main

import (
	"context"
	"dfolan/internal/loot"
	"dfolan/internal/database"
	"dfolan/internal/workflow"
	"log"
	"time"
)

// Keep the already-committed inventory ACK if event reconciliation fails.
// Later equipment/event requests or character entry retry from durable facts.
func (w *worldSession) reconcileBoostEquipment() []outboundPacket {
	if w == nil || w.boostup == nil || w.loot == nil || w.role.ID == 0 || w.account != w.role.AccountID {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lsvc := &workflow.LootService{Store: w.store, Loot: w.loot}
	// 套装积分真源是名望侧的 character.Service.BoostWornSetPoints（§0.2 单一规则）；
	// provider 为 nil 时依赖积分的关卡由 workflow 判为未满足，活动不读第二遍源。
	var setPoints workflow.BoostSetPoints
	if w.characters != nil {
		setPoints = func(role database.Character) (map[int32]uint32, error) {
			return w.characters.BoostWornSetPoints(role.State)
		}
	}
	next, changed, err := lsvc.ReconcileBoostEquipment(ctx, w.role, w.boostup, setPoints)
	if err != nil {
		log.Printf("boost wear reconcile role=%d: %v", w.role.ID, err)
		return nil
	}
	w.role = next
	var plan []outboundPacket
	if changed {
		body, err := boostTrainingRestore(w.boostup, next)
		if err != nil {
			log.Printf("boost wear status role=%d: %v", w.role.ID, err)
		} else {
			plan = append(plan, outboundPacket{"boost_equipment_mission_progress", 0, 2638, body})
		}
	}
	if w.characters != nil && len(w.boostup.Challenges) > 0 {
		next, changed, err := lsvc.ReconcileBoostChallenge(ctx, w.role, w.boostup, w.characters.BoostChallengeFacts)
		if err != nil {
			log.Printf("boost challenge equipment role=%d: %v", w.role.ID, err)
			return plan
		}
		w.role = next
		if changed {
			body, err := loot.BoostChallengeSnapshot(w.boostup, workflow.LootRole(next))
			if err != nil {
				log.Printf("boost challenge equipment snapshot role=%d: %v", w.role.ID, err)
			} else {
				plan = append(plan, outboundPacket{"boost_challenge_equipment_unlock", 0, 2722, body})
			}
		}
	}
	return plan
}
