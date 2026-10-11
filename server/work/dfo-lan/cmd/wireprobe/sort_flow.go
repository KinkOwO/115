package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

type sortSession struct {
	nonce       [16]byte
	initialized bool
}

// handleSortItem adopts the bag arrangement the client already applied locally
// (CMD20 SORT_ITEM) and answers with the authoritative bag restore.
//
// The command used to be dropped entirely, which left the client's own
// "inventory in use" latch set until the next reconnect: every later inventory
// action, equipment disassembly included, was then refused locally with DSTR
// 1659 ("The Inventory is currently in use."). Answering it is what clears the
// latch, so a replayed frame is answered too.
func (s *sortSession) handle(service *workflow.WearService, w *worldSession, p, raw []byte) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("item sort requires the owned character")
	}
	r, e := protocol.DecodeSortItem(p)
	if e != nil {
		return nil, e
	}
	if !s.initialized {
		if _, e = rand.Read(s.nonce[:]); e != nil {
			return nil, e
		}
		s.initialized = true
	}
	hash := sha256.Sum256(raw)
	key := fmt.Sprintf("itemsort:%x:%x", s.nonce, hash)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, applied, e := workflow.SortBag(ctx, w.store, w.role, service.BagRules, key, r)
	if e != nil {
		return nil, e
	}
	w.role = saved
	b, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	// A replay still gets the answer: the client's latch clears on the answer,
	// not on the state change.
	name := "item_sort_committed"
	if !applied {
		name = "item_sort_replayed"
	}
	plan := []outboundPacket{{name, 1, 20, protocol.SortItemSuccess()}}
	// [FIX-20261007 特殊容器整理] 整理后的权威排列必须走与登录同通道的
	// NOTI 13 整体恢复（登录 avatar_inventory_restored / creature_inventory_restored
	// 即是如此）：客户端识别 NOTI 13 + 对应 space 为"容器整体替换"，清空旧显示
	// 后重填。实测两种错误做法均异常：NOTI 14 全量（槽位更新通道）被客户端当
	// 增量叠加（旧行不消失，"出现两个"）；restore=true 但走 NOTI 14 则闪退。
	if r.List == 0 {
		if body, e := protocol.InventoryRestore(b.Rows(), b.Expansion); e == nil {
			plan = append(plan, outboundPacket{"item_sort_inventory_restored", 0, 13, body})
		}
	} else if r.List == 1 {
		if body, e := inventory.SpecialEquipmentRestorePayload(saved.State, 1); e == nil && len(body) > 0 {
			plan = append(plan, outboundPacket{"avatar_inventory_restored", 0, 13, body})
		}
	} else if r.List == 7 {
		if body, e := inventory.PetContainerRestorePayload(saved.State); e == nil && len(body) > 0 {
			plan = append(plan, outboundPacket{"creature_inventory_restored", 0, 13, body})
		}
	}
	return plan, nil
}
