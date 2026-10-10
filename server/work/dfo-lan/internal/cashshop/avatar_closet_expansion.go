package cashshop

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// avatarClosetKitMarker 是源商城里三张衣柜扩展券（Avatar Closet Expansion Kit）
// 索引路径里的特征串：…/avatar_closet/closet_expand_N.stk。
// 券的 PVF 脚本**没有** [action type]/[use action packet]（不是右键使用类道具），
// 因此按项目里其它 [item mod or ext] 扩展券的规律走“购买即生效”：不进背包、
// 直接把衣柜档位置为券对应的 N。
const avatarClosetKitMarker = "avatar_closet/closet_expand_"

// AvatarClosetExpansion 从券的源索引路径解析衣柜档位（closet_expand_N.stk → N）。
// 不按 SKU/模板号判定，只认源索引路径（与 InventoryExpansionTier 的做法一致）。
func (p *Pilot) AvatarClosetExpansion(template uint32) (byte, bool, error) {
	if p == nil {
		return 0, false, nil
	}
	entry, found := p.findEntry(0, template)
	if !found {
		return 0, false, nil
	}
	idx := entry.IndexPath
	pos := strings.LastIndex(idx, avatarClosetKitMarker)
	if pos < 0 {
		return 0, false, nil
	}
	suffix := strings.TrimSuffix(idx[pos+len(avatarClosetKitMarker):], ".stk")
	n, err := strconv.Atoi(suffix)
	if err != nil || n < 1 || n > int(protocol.MaxAvatarClosetExpansion) {
		return 0, true, fmt.Errorf("avatar closet kit tier invalid: %q", idx)
	}
	if entry.ImportError != "" {
		return 0, true, fmt.Errorf("avatar closet kit source mismatch")
	}
	return byte(n), true, nil
}

// TryPurchaseAvatarClosetExpansion 购买一张衣柜扩展券，在扣款事务内把
// bag.ClosetExpansion 提到该券的档位（只升不降），不向背包发货。
func (p *Pilot) TryPurchaseAvatarClosetExpansion(ctx context.Context, ledger BagLedger, account, character int64, key string, cart []protocol.CeraCartItem) (CashReceipt, bool, bool, error) {
	for _, line := range cart {
		entry, found := p.findEntry(line.Product, 0)
		if !found {
			continue
		}
		tier, handled, err := p.AvatarClosetExpansion(uint32(entry.Row[1].Value))
		if !handled && err == nil {
			continue
		}
		fail := func(err error) (CashReceipt, bool, bool, error) {
			return CashReceipt{}, false, true, err
		}
		if err != nil {
			return fail(err)
		}
		if ledger == nil || len(cart) != 1 || line.Quantity != 1 {
			return fail(fmt.Errorf("衣柜扩展券必须单独购买一张"))
		}
		if err := p.Config.Validate(); err != nil {
			return fail(err)
		}
		product, _, err := p.Config.classify(entry)
		if err != nil {
			return fail(err)
		}
		if product.Units != 1 {
			return fail(fmt.Errorf("invalid avatar closet product count"))
		}
		quote := Service{Catalog: Catalog{Source: p.Config.Source.SaveIdentity(), Products: map[uint32]Product{product.ID: product}}}
		order, err := quote.Quote(account, character, key, cart, time.Now())
		if err != nil {
			return fail(err)
		}
		gold, err := order.GoldTotal()
		if err != nil {
			return fail(err)
		}
		receipt, applied, err := ledger.PurchaseCashToBag(ctx, order, func(raw json.RawMessage) (json.RawMessage, error) {
			bag, err := inventory.ReadBag(raw)
			if err != nil {
				return nil, err
			}
			if uint64(bag.Gold) < gold {
				return nil, fmt.Errorf("insufficient Gold")
			}
			bag.Gold -= uint32(gold)
			if tier > bag.ClosetExpansion {
				bag.ClosetExpansion = tier
			}
			return inventory.SaveBag(raw, bag)
		})
		return receipt, applied, true, err
	}
	return CashReceipt{}, false, false, nil
}
