package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/cashshop"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"
)

type shopPilotSession struct {
	prefix     string
	keys       []byte
	vaultRules *inventory.VaultRules
}

type preparedBagLedger struct {
	ledger cashshop.BagLedger
	keys   []byte
	pilot  *cashshop.Pilot
}

// PurchaseCashMixed activates the cart's contract lines and delivers the rest
// inside the same order; the packets mirror the ordinary purchase response.
func (l preparedBagLedger) PurchaseCashMixed(ctx context.Context, o storage.CashOrder, deliver func(json.RawMessage) (json.RawMessage, error), premiums map[int]storage.CashPremiumActivation) (storage.CashReceipt, bool, error) {
	ledger, ok := l.ledger.(cashshop.ContractCartLedger)
	if !ok {
		return storage.CashReceipt{}, false, fmt.Errorf("contract cart ledger missing")
	}
	receipt, applied, err := ledger.PurchaseCashMixed(ctx, o, func(raw json.RawMessage) (json.RawMessage, error) {
		state, e := deliver(raw)
		if e != nil {
			return nil, e
		}
		// The stored receipt projects every order line (contract lines carry
		// no inventory row but still receive their per-line ACK), so the
		// encode check mirrors that without filtering.
		lines := make([]storage.CashDelivery, 0, len(o.Lines))
		for _, line := range o.Lines {
			lines = append(lines, storage.CashDelivery{Product: line.Product, Template: line.Template, Amount: line.Quantity * line.Units, Quantity: line.Quantity})
		}
		packets, e := shopPilotSpaces(l.pilot, storage.CashReceipt{CharacterState: state, Deliveries: lines, Premiums: receiptPremiumsFor(premiums)}, 0, true)
		if e != nil {
			return nil, e
		}
		if len(l.keys) != wire.SessionKeyBytes {
			return nil, fmt.Errorf("purchase cipher not initialized")
		}
		if _, e = preparePackets(l.keys, packets); e != nil {
			return nil, e
		}
		return state, nil
	}, premiums)
	return receipt, applied, err
}

// receiptPremiumsFor projects the activations for the encode-time packet
// check; the authoritative times come from the stored receipt.
func receiptPremiumsFor(premiums map[int]storage.CashPremiumActivation) []storage.CashPremium {
	if len(premiums) == 0 {
		return nil
	}
	out := make([]storage.CashPremium, 0, len(premiums))
	for _, act := range premiums {
		out = append(out, storage.CashPremium{Type: act.Type})
	}
	return out
}

func (l preparedBagLedger) PurchaseCashToBag(ctx context.Context, o storage.CashOrder, deliver func(json.RawMessage) (json.RawMessage, error)) (storage.CashReceipt, bool, error) {
	return l.ledger.PurchaseCashToBag(ctx, o, func(raw json.RawMessage) (json.RawMessage, error) {
		state, e := deliver(raw)
		if e != nil {
			return nil, e
		}
		lines := []storage.CashDelivery{}
		for _, line := range o.Lines {
			lines = append(lines, storage.CashDelivery{Product: line.Product, Template: line.Template, Amount: line.Quantity * line.Units, Quantity: line.Quantity})
		}
		packets, e := shopPilotSpaces(l.pilot, storage.CashReceipt{CharacterState: state, Deliveries: lines}, 0, true)
		if e != nil {
			return nil, e
		}
		if len(l.keys) != wire.SessionKeyBytes {
			return nil, fmt.Errorf("purchase cipher not initialized")
		}
		if _, e = preparePackets(l.keys, packets); e != nil {
			return nil, e
		}
		return state, nil
	})
}

func newShopPilotSession() (*shopPilotSession, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return nil, e
	}
	return &shopPilotSession{prefix: fmt.Sprintf("shop-pilot-%x-", b)}, nil
}

// A byte-identical retransmission in the same session uses the original key;
// a new frame is a new purchase, including another copy of the same SKU.
func (s *shopPilotSession) purchase(ctx context.Context, p *cashshop.Pilot, store cashshop.BagLedger, account, character int64, plain, frame []byte) (storage.CashReceipt, bool, error) {
	var r storage.CashReceipt
	if s == nil || len(frame) < 13 || character <= 0 {
		return r, false, fmt.Errorf("missing selected purchase session")
	}
	key := fmt.Sprintf("%s%x", s.prefix, sha256.Sum256(frame))
	cart, e := protocol.DecodeCeraCart(plain)
	if e != nil {
		return r, false, e
	}
	if s.vaultRules != nil {
		upgrades, err := p.Config.VaultUpgrades()
		if err != nil {
			return r, false, err
		}
		secondary, err := p.Config.VaultUpgrades(45)
		if err != nil {
			return r, false, err
		}
		for id, upgrade := range secondary {
			if _, exists := upgrades[id]; !exists {
				upgrades[id] = upgrade
			}
		}
		accountUpgrades, err := p.Config.AccountVaultUpgrades(s.vaultRules.Account)
		if err != nil {
			return r, false, err
		}
		for id, upgrade := range accountUpgrades {
			if _, exists := upgrades[id]; exists {
				return r, false, fmt.Errorf("账号金库商品与角色金库商品冲突")
			}
			upgrades[id] = upgrade
		}
		for _, item := range cart {
			if _, ok := upgrades[item.Product]; !ok {
				continue
			}
			ledger, ok := store.(cashshop.VaultLedger)
			if !ok {
				return r, false, fmt.Errorf("vault purchase ledger missing")
			}
			return p.PurchaseVault(ctx, ledger, *s.vaultRules, account, character, key, cart, func(receipt storage.CashReceipt) error {
				packets, err := shopPilotPackets(receipt, 0, true)
				if err != nil {
					return err
				}
				if len(s.keys) != wire.SessionKeyBytes {
					return fmt.Errorf("purchase cipher not initialized")
				}
				_, err = preparePackets(s.keys, packets)
				return err
			})
		}
	}
	prepared := preparedBagLedger{store, s.keys, p}
	r, applied, e := p.Purchase(ctx, prepared, account, character, key, cart)
	return r, applied, e
}

func shopPilotPackets(receipt storage.CashReceipt, balance uint64, applied bool) ([]outboundPacket, error) {
	return shopPilotSpaces(nil, receipt, balance, applied)
}

// shopPilotSpaces builds the purchase response; the pilot refines which
// special equipment spaces the delivery touched so the avatar wardrobe and
// creature tab refresh alongside the ordinary bag. Without a pilot the
// legacy pet-egg SKU fallback still refreshes creatures.
func shopPilotSpaces(p *cashshop.Pilot, receipt storage.CashReceipt, balance uint64, applied bool) ([]outboundPacket, error) {
	avatarTouched, creatureTouched := p.DeliverySpaces(receipt)
	var update outboundPacket
	var vaultUpgrade *outboundPacket
	var creatureUpdate *outboundPacket
	var avatarUpdate *outboundPacket
	var b inventory.Bag
	expansion := false
	for _, delivery := range receipt.Deliveries {
		if cashshop.InventoryExpansionTier(delivery.Template) != 0 {
			expansion = true
		}
	}
	if expansion {
		var err error
		b, err = inventory.ReadBag(receipt.CharacterState)
		if err != nil {
			return nil, err
		}
		payload, err := protocol.InventoryExpansionNotice(b.Expansion)
		if err != nil {
			return nil, err
		}
		update = outboundPacket{"cera_purchase_inventory_expansion", 0, 66, payload}
	} else if receipt.Vault != nil && receipt.VaultSpace == 12 {
		vault, err := inventory.ReadExtendedVault(*receipt.Vault)
		if err != nil {
			return nil, err
		}
		payload, err := protocol.AccountVaultRestore(vault.Slots, receipt.VaultGold, vault.Rows())
		if err != nil {
			return nil, err
		}
		update = outboundPacket{"点券账号金库快照", 0, 13, payload}
		if applied {
			// 0x14529A4A0 成功分支从当前容量取下一档并刷新金库页签，
			// 必须先于 NOTI13；重放只同步权威容量，不能再次执行升级。
			vaultUpgrade = &outboundPacket{"点券账号金库即时解锁", 1, 306, []byte{1}}
		}
	} else if receipt.Vault != nil {
		space := byte(2)
		if receipt.VaultSpace != 0 {
			space = receipt.VaultSpace
		}
		payload, err := inventory.VaultPayload(*receipt.Vault, space)
		if err != nil {
			return nil, err
		}
		update = outboundPacket{"cera_purchase_vault", 0, 13, payload}
		notice, err := protocol.PersonalVaultUpgradeNotice(receipt.Vault.Slots, space)
		if err != nil {
			return nil, err
		}
		vaultUpgrade = &outboundPacket{"cera_purchase_vault_expansion", 0, 66, notice}
	} else {
		var e error
		b, e = inventory.ReadBag(receipt.CharacterState)
		if e != nil {
			return nil, e
		}
		// For ordinary item purchases in CeraShop, NOTI 14 must be sent to update inventory.
		// However, virtual currency slots (slot <= 1, Gold & Coin) must be omitted from NOTI 14
		// because slot 1 lacks an item UI/implementation object in client 115, which would
		// trigger a null-dereference crash in sub_1452E9810 (acquired-item toast display).
		rows := b.Rows()
		filtered := make([][protocol.CurrentItemRecordSize]byte, 0, len(rows))
		for _, r := range rows {
			slot := binary.LittleEndian.Uint16(r[0:2])
			if slot <= 1 {
				continue
			}
			filtered = append(filtered, r)
		}
		items, e := protocol.InventoryUpdate(filtered)
		if e != nil {
			return nil, e
		}
		update = outboundPacket{"cera_purchase_inventory", 0, 14, items}

		if avatarTouched && len(b.Special[1]) > 0 {
			payload, err := inventory.EquipmentPayload(1, b.Special[1], false)
			if err != nil {
				return nil, err
			}
			avatarUpdate = &outboundPacket{"cera_purchase_avatar_inventory", 0, 14, payload}
		}

		if len(b.Special[7]) > 0 {
			hasCreature := creatureTouched
			if !hasCreature {
				for _, d := range receipt.Deliveries {
					if d.Product >= 3300000 && d.Product <= 3300011 {
						hasCreature = true
						break
					}
				}
			}
			if hasCreature {
				payload, err := inventory.EquipmentPayload(7, b.Special[7], false)
				if err != nil {
					return nil, err
				}
				creatureUpdate = &outboundPacket{"cera_purchase_creature_inventory", 0, 14, payload}
			}
		}
	}
	cera, e := protocol.CeraBalance(balance)
	if e != nil {
		return nil, e
	}
	packets := []outboundPacket{update}
	if vaultUpgrade != nil {
		// 扩容处理先于快照；角色金库使用 NOTI66，账号金库使用 CMD306 应答。
		packets = append([]outboundPacket{*vaultUpgrade}, packets...)
	}
	if avatarUpdate != nil {
		packets = append(packets, *avatarUpdate)
	}
	if creatureUpdate != nil {
		packets = append(packets, *creatureUpdate)
	}
	packets = append(packets, outboundPacket{"cera_purchase_balance", 0, 53, cera})
	if applied {
		if len(receipt.Deliveries) == 0 || len(receipt.Deliveries) > 32 {
			return nil, fmt.Errorf("purchase receipt needs1..32 deliveries")
		}
		for _, d := range receipt.Deliveries {
			ack, e := protocol.CeraPurchaseOrdinarySuccess(d.Product, d.Quantity)
			if e != nil {
				return nil, e
			}
			packets = append(packets, outboundPacket{"cera_purchase_success", 1, 64, ack})
		}
		hasLifeToken := false
		for _, d := range receipt.Deliveries {
			if d.Template == 1 {
				hasLifeToken = true
				break
			}
		}
		if hasLifeToken {
			restorePayload, err := protocol.InventoryRestore(b.Rows(), b.Expansion)
			if err == nil {
				packets = append(packets, outboundPacket{"cera_purchase_inventory_restored", 0, 13, restorePayload})
			}
		}
		// NOTI66 接收剩余秒数；回执 EndTime 是存档用的绝对到期时间。
		// 与开箱、背包契约使用相同编码，保留购买 ACK 之后的发送顺序。
		// 按发送时刻换算，既保留续费叠加期限，也不延长回执处理期间的时间。
		now := time.Now().Unix()
		for _, pr := range receipt.Premiums {
			remaining := pr.EndTime - now
			if remaining <= 0 {
				continue
			}
			notice, err := protocol.PremiumActivationNotice(pr.Type, remaining)
			if err != nil {
				return nil, err
			}
			packets = append(packets, outboundPacket{"cera_purchase_premium_activated", 0, 66, notice})
		}
	}
	return packets, nil
}
