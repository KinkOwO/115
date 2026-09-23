package loot

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

type Service struct {
	Currency    *OdysseyCurrency
	Store       *storage.Store
	Catalog     catalog.LootCatalog
	DropCatalog catalog.LootCatalog
	Rules       Rules
	BagRules    inventory.BagRules
	Tables      Tables
	Equipment   *inventory.EquipmentCatalog
	CardPolicy  *CardRules
	// ItemShops 是源物品商店表（itemshop/**.shp）。它给出"用物品支付"的商品价格：
	// 奥德赛商店的盒子要 100 个银币（10418036）。缺这张表时所有购买都会拿写死的
	// 金币单价去扣，等于白送（实机 2026-09-23 玩家报告"银币没有扣减"）。
	ItemShops *catalog.ItemShops
	// ChapterDrop 是章节最终领主的章节盒掉落（手册 P3 子项 3）。默认整表
	// enabled=false，禁用行连掷骰种子都不消耗；由 profile 显式开启。
	ChapterDrop *OdysseyChapterDrop
}
type PickupReceipt struct {
	Run         string
	Map, Object uint32
	Award       Award
	Destination uint16
	Source      string
}

func (s *Service) Bootstrap(role storage.Character) ([]byte, error) {
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return nil, fmt.Errorf("inventory source mismatch")
	}
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, e
	}
	return protocol.InventoryRestore(b.Rows(), b.Expansion)
}
func (s *Service) Pickup(ctx context.Context, role storage.Character, session *Session, d *dungeon.Session, r protocol.PickupRequest) (storage.Character, PickupReceipt, bool, error) {
	var result PickupReceipt
	fail := func(e error) (storage.Character, PickupReceipt, bool, error) { return role, result, false, e }
	if session == nil || session.Catalog.Source.Checksum != s.Catalog.Source.Checksum || session.Rules.Model != s.Rules.Model {
		return fail(fmt.Errorf("pickup without loot session"))
	}
	drop, e := session.Owned(d, role.AccountID, role.ID, role.WireID, r.Object)
	if e != nil {
		return fail(e)
	}
	awardCatalog, bagRules := s.Catalog, s.BagRules
	if d.Definition.Odyssey && session.Currency != nil {
		if s.Currency == nil || session.Currency.Source != s.Catalog.Source.Checksum || session.Currency.Model != s.Currency.Model {
			return fail(fmt.Errorf("currency pickup policy mismatch"))
		}
		awardCatalog, bagRules = session.Currency.StorageCatalog(s.Catalog), session.Currency.BagRules(s.BagRules)
	}
	distance := func(a, b uint16) int {
		n := int(a) - int(b)
		if n < 0 {
			return -n
		}
		return n
	}
	if distance(r.ActorX, r.DropX) > int(s.Rules.MaximumPickupX) || distance(r.ActorY, r.DropY) > int(s.Rules.MaximumPickupY) {
		return fail(fmt.Errorf("pickup request coordinates are too far apart"))
	}
	key := fmt.Sprintf("pickup:%s:%d", drop.Run, drop.Object)
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Catalog.Source.Checksum, key, s.Rules.Model, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		b, e := inventory.ReadBag(current.State)
		if e != nil {
			return nil, nil, e
		}
		var slot uint16
		if drop.Award.Template != 0 && awardCatalog.Items[drop.Award.Template].Kind != "stackable" {
			// Gear goes to the bag's equipment range with its own durability,
			// not into a stackable category slot.
			var slots []uint16
			b, slots, e = b.AddEquipment(s.Equipment, s.BagRules.EquipmentSlots, drop.Award.Template, drop.Award.Amount)
			if e != nil {
				return nil, nil, e
			}
			slot = slots[0]
		} else if b, slot, e = b.Add(awardCatalog, bagRules, drop.Award.Template, drop.Award.Amount); e != nil {
			return nil, nil, e
		}
		updated, e := inventory.SaveBag(current.State, b)
		if e != nil {
			return nil, nil, e
		}
		result = PickupReceipt{drop.Run, drop.Map, drop.Object, drop.Award, slot, s.Catalog.Source.Checksum}
		receipt, e := json.Marshal(result)
		return updated, receipt, e
	})
	if e != nil {
		return fail(e)
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(receipt, &result); e != nil {
		return fail(e)
	}
	if result.Source != s.Catalog.Source.Checksum || result.Run != drop.Run || result.Object != drop.Object || result.Award != drop.Award {
		return fail(fmt.Errorf("pickup receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, result, applied, nil
}
