package cashshop

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"fmt"
)

type VaultUpgrade struct {
	Product, Template, Price uint32
	Before, After            uint16
	Space                    byte
}
type VaultLedger interface {
	PurchaseCashVault(context.Context, storage.CashOrder, func(storage.VaultState) (storage.VaultState, error)) (storage.CashReceipt, bool, error)
}

// 账号金库的商城表为 39 组商品/模板和 39 组档位映射。
// 源表最后十档映射填的是模板编号，实际购买商品仍由前半表与
// AccountCargo.etc 的商城模板交叉定位，不能把模板当作商品发送。
func (c PilotConfig) AccountVaultUpgrades(rules *inventory.AccountVaultRules) (map[uint32]VaultUpgrade, error) {
	out := map[uint32]VaultUpgrade{}
	if rules == nil || len(c.Policies["[cargo account]"]) == 0 {
		return out, nil
	}
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	cells := c.Policies["[cargo account]"]
	count := len(rules.Upgrades) - 1
	if count <= 0 || len(cells) != count*4 {
		return nil, fmt.Errorf("账号金库商城映射与容量档位不一致")
	}
	entries := map[int32]OrdinaryProduct{}
	for _, entry := range c.Entries {
		if len(entry.Row) == 14 {
			entries[entry.Row[0].Value] = entry
		}
	}
	for i := 0; i < count; i++ {
		for _, at := range []int{2 * i, 2*i + 1, 2*count + 2*i, 2*count + 2*i + 1} {
			if cells[at].Type != 0 {
				return nil, fmt.Errorf("账号金库商城映射字段类型无效")
			}
		}
		id, template := cells[2*i].Value, cells[2*i+1].Value
		ordinal, mapped := cells[2*count+2*i].Value, cells[2*count+2*i+1].Value
		if id <= 0 || template <= 0 || int64(template) != rules.Upgrades[i+1][5] || ordinal != int32(i+1) || (mapped != id && mapped != template) {
			return nil, fmt.Errorf("账号金库第 %d 档商品与源模板不一致", i+1)
		}
		if _, exists := out[uint32(id)]; exists {
			return nil, fmt.Errorf("账号金库商品重复")
		}
		e, exists := entries[id]
		if !exists || e.Section != "[item mod or ext]" || e.ImportError != "" || !digestValid(e.Item.SHA256) {
			return nil, fmt.Errorf("账号金库商品 %d 缺少源脚本", id)
		}
		r := e.Row
		for _, at := range []int{0, 1, 2, 3, 4, 5, 6, 7, 9, 10, 11, 13} {
			if r[at].Type != 0 {
				return nil, fmt.Errorf("账号金库商品价格字段无效")
			}
		}
		if r[1].Value != template || r[2].Value != 1 || r[5].Value <= 0 || r[3].Value != 0 || r[4].Value != 0 || r[6].Value != 0 || r[7].Value != 0 || r[9].Value != 0 || r[10].Value != 0 || r[11].Value != -1 || r[12].Type != 6 || r[12].Text != "" || r[13].Value != -1 {
			return nil, fmt.Errorf("账号金库商品 %d 的价格或销售规则尚不支持", id)
		}
		for name, width := range map[string]int{"[purchasing limit]": 8, "[specific product mileage]": 2} {
			for j := 0; j < len(c.Policies[name]); j += width {
				if c.Policies[name][j].Value == id {
					return nil, fmt.Errorf("账号金库商品含未支持的限购或返还规则")
				}
			}
		}
		out[uint32(id)] = VaultUpgrade{Product: uint32(id), Template: uint32(template), Price: uint32(r[5].Value), Before: uint16(rules.Upgrades[i][0]), After: uint16(rules.Upgrades[i+1][0]), Space: 12}
	}
	return out, nil
}

// 原生容量表为 8..264 格。两套目录都按“当前档位 → 下一档商品”解析，
// 金库 2 的首档引用金库 1 的商品，其余 15 档使用自己的商品及模板。
func (c PilotConfig) VaultUpgrades(space ...byte) (map[uint32]VaultUpgrade, error) {
	out := map[uint32]VaultUpgrade{}
	section, pairs, target := "[cargo 1]", 16, byte(0)
	if len(space) == 1 && space[0] == 45 {
		section, pairs, target = "[cargo 2]", 15, 45
	} else if len(space) > 1 || (len(space) == 1 && space[0] != 2) {
		return nil, fmt.Errorf("金库商品目录容器无效")
	}
	cells := c.Policies[section]
	if len(cells) == 0 {
		return out, nil
	}
	if len(cells) != pairs*2+32 {
		return nil, fmt.Errorf("%s 商品与档位映射长度无效", section)
	}
	templates := map[int32]int32{}
	for i := 0; i < pairs*2; i += 2 {
		if cells[i].Type != 0 || cells[i+1].Type != 0 || cells[i].Value <= 0 || cells[i+1].Value <= 0 {
			return nil, fmt.Errorf("%s 商品模板映射无效", section)
		}
		if _, exists := templates[cells[i].Value]; exists {
			return nil, fmt.Errorf("%s 商品编号重复", section)
		}
		templates[cells[i].Value] = cells[i+1].Value
	}
	if target == 45 {
		primary, err := c.VaultUpgrades()
		if err != nil {
			return nil, err
		}
		shared, ok := primary[uint32(cells[pairs*2+1].Value)]
		if !ok || shared.Before != 8 {
			return nil, fmt.Errorf("金库 2 首档缺少有效的共用商品")
		}
		templates[int32(shared.Product)] = int32(shared.Template)
	}
	entries := map[int32]OrdinaryProduct{}
	for _, e := range c.Entries {
		if len(e.Row) == 14 {
			entries[e.Row[0].Value] = e
		}
	}
	for i := 0; i < 16; i++ {
		at := pairs*2 + 2*i
		for _, at := range []int{at, at + 1} {
			if cells[at].Type != 0 {
				return nil, fmt.Errorf("invalid cargo1 token")
			}
		}
		id := cells[at+1].Value
		template, mapped := templates[id]
		if cells[at].Value != int32(i+1) || !mapped || (target == 0 && cells[2*i].Value != id) || (target == 45 && i > 0 && cells[2*(i-1)].Value != id) {
			return nil, fmt.Errorf("%s 商品档位不一致", section)
		}
		e, ok := entries[id]
		if !ok || e.Section != "[item mod or ext]" || e.ImportError != "" {
			return nil, fmt.Errorf("missing cargo1 product definition")
		}
		r := e.Row
		for _, at := range []int{0, 1, 2, 3, 4, 5, 6, 7, 9, 10, 11, 13} {
			if r[at].Type != 0 {
				return nil, fmt.Errorf("invalid cargo price cell")
			}
		}
		// 第二金库的专属商品在源表中为隐藏展示项，由金库升级按钮选中。
		display := int32(0)
		if target == 45 && i > 0 {
			display = 1
		}
		if id <= 0 || template <= 0 || r[1].Value != template || r[2].Value != 1 || r[5].Value <= 0 || r[3].Value != 0 || r[4].Value != 0 || r[6].Value != 0 || r[7].Value != 0 || r[9].Value != display || r[10].Value != 0 || r[11].Value != -1 || r[12].Type != 6 || r[12].Text != "" || r[13].Value != -1 {
			return nil, fmt.Errorf("%s 商品价格或销售规则尚不支持：%d", section, id)
		}
		if !digestValid(e.Item.SHA256) {
			return nil, fmt.Errorf("missing cargo1 script hash")
		}
		// Limit and reward policies must not be silently discarded.
		for name, width := range map[string]int{"[purchasing limit]": 8, "[specific product mileage]": 2} {
			policy := c.Policies[name]
			for j := 0; j < len(policy); j += width {
				if policy[j].Value == id {
					return nil, fmt.Errorf("cargo1 has additional purchase policy")
				}
			}
		}
		out[uint32(id)] = VaultUpgrade{Product: uint32(id), Template: uint32(template), Price: uint32(r[5].Value), Before: uint16(8 + 16*i), After: uint16(24 + 16*i), Space: target}
	}
	return out, nil
}

func (p *Pilot) PurchaseVault(ctx context.Context, ledger VaultLedger, rules inventory.VaultRules, account, character int64, key string, cart []protocol.CeraCartItem, prepare func(storage.CashReceipt) error) (storage.CashReceipt, bool, error) {
	if p == nil || ledger == nil || prepare == nil || len(cart) != 1 || cart[0].Quantity != 1 {
		return storage.CashReceipt{}, false, fmt.Errorf("vault upgrade requires one unit in a separate order")
	}
	if e := p.Config.validate(); e != nil {
		return storage.CashReceipt{}, false, e
	}
	products, e := p.Config.VaultUpgrades()
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	u, ok := products[cart[0].Product]
	secondary, e := p.Config.VaultUpgrades(45)
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	if second, found := secondary[cart[0].Product]; found {
		if !ok {
			u, ok = second, true
		} else if selector, supports := ledger.(interface {
			VaultPurchaseSpace(context.Context, int64, int64, string) (byte, error)
		}); supports {
			space, err := selector.VaultPurchaseSpace(ctx, account, character, key)
			if err != nil {
				return storage.CashReceipt{}, false, err
			}
			if space == 45 {
				u = second
			} else if space != 0 {
				return storage.CashReceipt{}, false, fmt.Errorf("共用扩容商品的金库目标无效")
			}
		}
	}
	if !ok {
		accountProducts, err := p.Config.AccountVaultUpgrades(rules.Account)
		if err != nil {
			return storage.CashReceipt{}, false, err
		}
		u, ok = accountProducts[cart[0].Product]
		if !ok {
			return storage.CashReceipt{}, false, fmt.Errorf("unsupported vault upgrade product")
		}
	}
	o := storage.CashOrder{Key: key, Account: account, Character: character, Source: p.Config.Source.Checksum, Lines: []storage.CashOrderLine{{Product: u.Product, Template: u.Template, Quantity: 1, Units: 1, UnitPrice: u.Price}}}
	o.VaultSpace = u.Space
	return ledger.PurchaseCashVault(ctx, o, func(v storage.VaultState) (storage.VaultState, error) {
		source := rules.SourceSHA256
		if u.Space == 12 {
			source = p.Config.Source.Checksum
		}
		if v.ConfigVersion != source || v.Slots != u.Before {
			return v, fmt.Errorf("vault upgrade requires %d current slots", u.Before)
		}
		allowed := u.Space == 12
		for _, n := range rules.VerifiedSlots {
			if n == u.After {
				allowed = true
			}
		}
		if !allowed {
			return v, fmt.Errorf("vault target capacity not enabled")
		}
		v.Slots = u.After
		if u.Space == 12 {
			if _, e := inventory.AccountVaultPayload(storage.AccountVaultState{Slots: v.Slots, Items: v.Items}, *rules.Account); e != nil {
				return v, e
			}
		} else if _, e := inventory.VaultPayload(v); e != nil {
			return v, e
		}
		receipt := storage.CashReceipt{Vault: &v, VaultSpace: u.Space, Deliveries: []storage.CashDelivery{{Product: u.Product, Template: u.Template, Amount: 1, Quantity: 1}}}
		if e := prepare(receipt); e != nil {
			return v, e
		}
		return v, nil
	})
}
