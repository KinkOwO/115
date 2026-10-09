package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/cashshop"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// DFO_CONTRACT_PURCHASE_CRASH_FIX=1 skips the immediate premium notice sent
// after a contract purchase ACK for clients that crash on that notice.
func contractPurchaseCrashFixEnabled() bool {
	return os.Getenv("DFO_CONTRACT_PURCHASE_CRASH_FIX") != "0"
}

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
func (l preparedBagLedger) PurchaseCashMixed(ctx context.Context, o database.CashOrder, deliver func(json.RawMessage) (json.RawMessage, error), premiums map[int]database.CashPremiumActivation) (database.CashReceipt, bool, error) {
	ledger, ok := l.ledger.(cashshop.ContractCartLedger)
	if !ok {
		return database.CashReceipt{}, false, fmt.Errorf("contract cart ledger missing")
	}
	receipt, applied, err := ledger.PurchaseCashMixed(ctx, o, func(raw json.RawMessage) (json.RawMessage, error) {
		state, e := deliver(raw)
		if e != nil {
			return nil, e
		}
		// The stored receipt projects every order line (contract lines carry
		// no inventory row but still receive their per-line ACK), so the
		// encode check mirrors that without filtering.
		lines := make([]database.CashDelivery, 0, len(o.Lines))
		for _, line := range o.Lines {
			lines = append(lines, database.CashDelivery{Product: line.Product, Template: line.Template, Amount: line.Quantity * line.Units, Quantity: line.Quantity})
		}
		goldCost, e := o.GoldTotal()
		if e != nil {
			return nil, e
		}
		packets, e := shopPilotSpaces(l.pilot, database.CashReceipt{CharacterState: state, GoldCharged: goldCost, Deliveries: lines, Premiums: receiptPremiumsFor(premiums)}, 0, true)
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
func receiptPremiumsFor(premiums map[int]database.CashPremiumActivation) []database.CashPremium {
	if len(premiums) == 0 {
		return nil
	}
	out := make([]database.CashPremium, 0, len(premiums))
	for _, act := range premiums {
		out = append(out, database.CashPremium{Type: act.Type})
	}
	return out
}

func (l preparedBagLedger) PurchaseCashToBag(ctx context.Context, o database.CashOrder, deliver func(json.RawMessage) (json.RawMessage, error)) (database.CashReceipt, bool, error) {
	return l.ledger.PurchaseCashToBag(ctx, o, func(raw json.RawMessage) (json.RawMessage, error) {
		state, e := deliver(raw)
		if e != nil {
			return nil, e
		}
		lines := []database.CashDelivery{}
		for _, line := range o.Lines {
			lines = append(lines, database.CashDelivery{Product: line.Product, Template: line.Template, Amount: line.Quantity * line.Units, Quantity: line.Quantity})
		}
		goldCost, e := o.GoldTotal()
		if e != nil {
			return nil, e
		}
		packets, e := shopPilotSpaces(l.pilot, database.CashReceipt{CharacterState: state, GoldCharged: goldCost, Deliveries: lines}, 0, true)
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

// PurchaseCashCharacterSlots grants the account-level character-slot bonus
// (Character Slot Extension Kit, 购买即生效) inside the same cash transaction
// and emits the purchase response packets (CERA debit + roster refresh). No bag
// item is delivered; the wrapped ledger (Store) owns the atomicity.
func (l preparedBagLedger) PurchaseCashCharacterSlots(ctx context.Context, o database.CashOrder, bonus int) (database.CashReceipt, bool, error) {
	ledger, ok := l.ledger.(cashshop.CharacterSlotLedger)
	if !ok {
		return database.CashReceipt{}, false, fmt.Errorf("character slot ledger missing")
	}
	receipt, applied, err := ledger.PurchaseCashCharacterSlots(ctx, o, bonus)
	if err != nil {
		return receipt, applied, err
	}
	packets, e := shopPilotSpaces(l.pilot, receipt, 0, true)
	if e != nil {
		return database.CashReceipt{}, false, e
	}
	if len(l.keys) != wire.SessionKeyBytes {
		return database.CashReceipt{}, false, fmt.Errorf("purchase cipher not initialized")
	}
	if _, e = preparePackets(l.keys, packets); e != nil {
		return database.CashReceipt{}, false, e
	}
	return receipt, applied, nil
}

// PurchaseCashSkillTreeExpansion unlocks the character's second skill type
// (Skill Type Extension Ticket, product 3000150 / template 821) inside the same
// cash transaction. The dispatcher appends the authoritative USERINFO1 refresh
// because the world session lives there, not in this ledger.
func (l preparedBagLedger) PurchaseCashSkillTreeExpansion(ctx context.Context, o database.CashOrder) (database.CashReceipt, bool, error) {
	ledger, ok := l.ledger.(cashshop.SkillTreeLedger)
	if !ok {
		return database.CashReceipt{}, false, fmt.Errorf("skill tree ledger missing")
	}
	receipt, applied, err := ledger.PurchaseCashSkillTreeExpansion(ctx, o)
	if err != nil {
		return receipt, applied, err
	}
	packets, e := shopPilotSpaces(l.pilot, receipt, 0, true)
	if e != nil {
		return database.CashReceipt{}, false, e
	}
	if len(l.keys) != wire.SessionKeyBytes {
		return database.CashReceipt{}, false, fmt.Errorf("purchase cipher not initialized")
	}
	if _, e = preparePackets(l.keys, packets); e != nil {
		return database.CashReceipt{}, false, e
	}
	return receipt, applied, nil
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
func (s *shopPilotSession) purchase(ctx context.Context, p *cashshop.Pilot, store cashshop.BagLedger, account, character int64, plain, frame []byte) (database.CashReceipt, bool, error) {
	var r database.CashReceipt
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
			ledger, ok := store.(workflow.VaultLedger)
			if !ok {
				return r, false, fmt.Errorf("vault purchase ledger missing")
			}
			return workflow.PurchaseCashVault(p, ctx, ledger, *s.vaultRules, account, character, key, cart, func(receipt database.CashReceipt) error {
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

func shopPilotPackets(receipt database.CashReceipt, balance uint64, applied bool) ([]outboundPacket, error) {
	return shopPilotSpaces(nil, receipt, balance, applied)
}

// shopPilotSpaces builds the purchase response; the pilot refines which
// special equipment spaces the delivery touched so the avatar wardrobe and
// creature tab refresh alongside the ordinary bag. Without a pilot the
// legacy pet-egg SKU fallback still refreshes creatures.
func shopPilotSpaces(p *cashshop.Pilot, receipt database.CashReceipt, balance uint64, applied bool) ([]outboundPacket, error) {
	avatarTouched, creatureTouched := p.DeliverySpaces(receipt)
	var update outboundPacket
	var vaultUpgrade *outboundPacket
	var creatureUpdate *outboundPacket
	var avatarUpdate *outboundPacket
	var avatarExpansionNotice *outboundPacket
	var avatarExpansionMessage *outboundPacket
	var b inventory.Bag
	expansion := false
	avatarExpansion := false
	for _, delivery := range receipt.Deliveries {
		if _, found, err := p.AvatarInventoryExpansion(delivery.Template); err != nil {
			return nil, err
		} else if found {
			avatarExpansion = true
		}
		if cashshop.InventoryExpansionTier(delivery.Template) != 0 {
			expansion = true
		}
	}
	if avatarExpansion {
		var err error
		b, err = inventory.ReadBag(receipt.CharacterState)
		if err != nil {
			return nil, err
		}
		body, err := inventory.SpecialEquipmentRestorePayload(receipt.CharacterState, 1)
		if err != nil {
			return nil, err
		}
		update = outboundPacket{"cera_purchase_avatar_capacity_restored", 0, 13, body}
		body, err = protocol.AvatarInventoryExpansionNotice(b.AvatarExpansion)
		if err != nil {
			return nil, err
		}
		avatarExpansionNotice = &outboundPacket{"cera_purchase_avatar_expansion", 0, 66, body}
		if applied {
			body, err = protocol.AvatarInventoryExpansionPurchaseMessage(b.AvatarExpansion)
			if err != nil {
				return nil, err
			}
			avatarExpansionMessage = &outboundPacket{"cera_purchase_avatar_expansion_success_message", 0, 488, body}
		}
	} else if expansion {
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

		if len(b.Special[7])+len(b.PetItems) > 0 {
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
				payload, err := inventory.PetContainerBody(b, false)
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
	if avatarExpansionNotice != nil {
		packets = append([]outboundPacket{*avatarExpansionNotice}, packets...)
	}
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
		if !contractPurchaseCrashFixEnabled() {
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
	}
	if receipt.GoldCharged > 0 {
		body, err := protocol.InventoryRestore(b.Rows(), b.Expansion)
		if err != nil {
			return nil, err
		}
		packets = append(packets, outboundPacket{"cera_purchase_gold_restored", 0, 13, body})
	}
	if avatarExpansionMessage != nil {
		// CMD64 closes the native confirmation dialog; show success only
		// after the ACK and absolute snapshots, never on receipt replay.
		packets = append(packets, *avatarExpansionMessage)
	}
	return packets, nil
}
