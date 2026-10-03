package cashshop

import (
	"context"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"time"
)

// InventoryExpansionTier 对应当前源商城的两个扩展券模板；容量由客户端按 40+8*档位计算。
func InventoryExpansionTier(template uint32) byte {
	switch template {
	case 2660296:
		return 1
	case 2660297:
		return 2
	}
	return 0
}

func (p *Pilot) TryPurchaseInventoryExpansion(ctx context.Context, ledger BagLedger, account, character int64, key string, cart []protocol.CeraCartItem) (CashReceipt, bool, bool, error) {
	for _, line := range cart {
		entry, found := p.findEntry(line.Product, 0)
		if !found {
			continue
		}
		tier := InventoryExpansionTier(uint32(entry.Row[1].Value))
		if tier == 0 {
			continue
		}
		fail := func(err error) (CashReceipt, bool, bool, error) {
			return CashReceipt{}, false, true, err
		}
		if len(cart) != 1 || line.Quantity != 1 || ledger == nil {
			return fail(fmt.Errorf("背包扩展券必须单独购买一张"))
		}
		if err := p.Config.Validate(); err != nil {
			return fail(err)
		}
		expected := fmt.Sprintf("stackable/cash/inven_upgradekit%d.stk", tier)
		if entry.ImportError != "" || entry.Section != "[item mod or ext]" || entry.Item.Path != expected || entry.IndexPath != expected || !digestValid(entry.Item.SHA256) {
			return fail(fmt.Errorf("背包扩展券源定义不匹配"))
		}
		// 复用价格与来源校验；单件购买、档位前置条件及立即生效由下面的事务负责。
		config := p.Config
		entry.Section = "[item]"
		config.Policies = make(map[string][]pvf.Token, len(p.Config.Policies))
		for name, cells := range p.Config.Policies {
			config.Policies[name] = cells
		}
		for _, name := range []string{"[immediately adaptive product]", "[not stackable buy]"} {
			config.Policies[name] = nil
			for _, cell := range p.Config.Policies[name] {
				if cell.Value != int32(line.Product) {
					config.Policies[name] = append(config.Policies[name], cell)
				}
			}
		}
		product, _, err := config.classify(entry)
		if err != nil {
			return fail(err)
		}
		if product.Units != 1 {
			return fail(fmt.Errorf("背包扩展券数量配置无效"))
		}
		quote := Service{Catalog: Catalog{Source: p.Config.Source.SaveIdentity(), Products: map[uint32]Product{product.ID: product}}}
		order, err := quote.Quote(account, character, key, cart, time.Now())
		if err != nil {
			return fail(err)
		}
		receipt, applied, err := ledger.PurchaseCashToBag(ctx, order, func(raw json.RawMessage) (json.RawMessage, error) {
			bag, err := inventory.ReadBag(raw)
			if err != nil {
				return nil, err
			}
			if bag.Expansion+1 != tier {
				return nil, fmt.Errorf("背包扩展需要当前档位为 %d，实际为 %d", tier-1, bag.Expansion)
			}
			bag.Expansion = tier
			return inventory.SaveBag(raw, bag)
		})
		return receipt, applied, true, err
	}
	return CashReceipt{}, false, false, nil
}
