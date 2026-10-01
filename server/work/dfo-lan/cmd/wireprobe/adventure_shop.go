package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/adventure"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"errors"
	"fmt"
)

var errAdventureLevel = errors.New("冒险团等级不足")
var errAdventureLimit = errors.New("冒险团商品已达到本期限购数量")
var errAdventurePoints = errors.New("冒险团商店积分不足")

// 0x143C6C138 的跳转表分别映射源 DSTR 的等级、限购、积分、背包满提示。
func adventureFailure(id uint16, err error) []byte {
	if id == 2139 {
		return protocol.AdventureCollectionResponse(false)
	}
	code := uint16(3)
	if id == 2419 || id == 2405 {
		var refusal seasonRefusal
		if errors.As(err, &refusal) {
			return protocol.Refusal(refusal.code)
		}
		// 其它失败不谎报为原生错误3（角色不存在）。
		return protocol.Refusal(204)
	}
	if id == 1406 {
		switch {
		case errors.Is(err, errAdventureLevel):
			code = 235
		case errors.Is(err, errAdventureLimit):
			code = 19
		case errors.Is(err, errAdventurePoints):
			code = 22
		case errors.Is(err, inventory.ErrMailBagFull):
			code = 204
		}
	}
	return protocol.Refusal(code)
}

func (w *worldSession) buyAdventureItem(ctx context.Context, p, raw []byte, prefix string) ([]outboundPacket, error) {
	category, template, count, err := protocol.DecodeAdventurePurchase(p)
	if err != nil {
		return nil, err
	}
	if w.characters == nil || w.store == nil || w.loot == nil || w.role.ID == 0 || w.role.AccountID != w.account {
		return nil, fmt.Errorf("冒险团购买缺少当前角色或背包目录")
	}
	if _, err = w.prepareAdventure(ctx); err != nil {
		return nil, err
	}
	rules, err := adventure.Current()
	if err != nil {
		return nil, err
	}
	shop, ok := rules.Shops[category]
	if !ok {
		return nil, fmt.Errorf("当前客户端没有该冒险团商店")
	}
	var product adventure.ShopItem
	for _, row := range shop.Items {
		if row.Template == template {
			product = row
			break
		}
	}
	if product.Template == 0 || count > product.Limit {
		return nil, fmt.Errorf("冒险团商品或数量无效")
	}
	definition := rules.Items[template]
	// 仅补入此商品的当前源定义，不修改共享掉落目录或借用其它物品的栏位。
	itemCatalog := catalog.LootCatalog{Source: w.loot.Catalog.Source, Items: map[uint32]catalog.LootItem{
		template: {ID: template, Kind: "stackable", StackableType: definition.Type, StackLimit: definition.Limit}}}
	key := fmt.Sprintf("adventure-shop:%s:%x", prefix, sha256.Sum256(raw))
	saved, _, _, err := w.store.CommitAdventure(ctx, w.account, w.role.ID, key, func(role storage.Character, profile *storage.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		if profile.Level < product.Level {
			return nil, nil, errAdventureLevel
		}
		if profile.Data.Purchases[template] > product.Limit-count {
			return nil, nil, errAdventureLimit
		}
		cost := uint64(product.Price) * uint64(count)
		if cost > uint64(profile.Data.Points[category]) {
			return nil, nil, errAdventurePoints
		}
		bag, e := inventory.ReadBag(role.State)
		if e != nil {
			return nil, nil, e
		}
		// 复用邮件入包的严格期限及堆叠校验，所有数量全部入包后才扣点。
		for n := uint32(0); n < count; n++ {
			bag, e = bag.AddMailItem(itemCatalog, w.loot.BagRules, nil, inventory.MailItem{Stack: &inventory.BagItem{Template: template, Amount: 1}})
			if e != nil {
				return nil, nil, e
			}
		}
		state, e := inventory.SaveBag(role.State, bag)
		if e != nil {
			return nil, nil, e
		}
		profile.Data.Points[category] -= uint32(cost)
		profile.Data.Purchases[template] += count
		receipt, e := json.Marshal(map[string]any{"category": category, "template": template, "count": count, "cost": cost, "remaining": profile.Data.Points[category]})
		return state, receipt, e
	})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	inventoryBody, err := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	if err != nil {
		return nil, err
	}
	detail, err := w.adventureDetail(ctx)
	if err != nil {
		return nil, err
	}
	// 原生购买 ACK 只刷新 UI；必须先同步实际背包与余额，避免继续显示旧限购。
	return []outboundPacket{{"冒险团购买背包同步", 0, 13, inventoryBody}, {"冒险团购买资料同步", 0, 1331, detail}, {"冒险团购买完成", 1, 1406, []byte{1}}}, nil
}

func (w *worldSession) adventureDetail(ctx context.Context) ([]byte, error) {
	request := []byte{byte(w.role.WireID), byte(w.role.WireID >> 8), 1, 0, 0, 0}
	p, err := w.handleAdventure(ctx, w.role.ID, request)
	if err != nil {
		return nil, err
	}
	return p[1:], nil // NOTI1331 与 CMD1395 共享字段，但没有命令成功字节。
}
