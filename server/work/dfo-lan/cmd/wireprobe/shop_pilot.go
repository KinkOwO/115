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
)

type shopPilotSession struct {
	prefix     string
	keys       []byte
	vaultRules *inventory.VaultRules
}

type preparedBagLedger struct {
	ledger cashshop.BagLedger
	keys   []byte
}

func (l preparedBagLedger) PurchaseCashPremium(ctx context.Context, o storage.CashOrder, premiumType uint8, durationSecond int64) (storage.CashReceipt, bool, error) {
	ledger, ok := l.ledger.(cashshop.PremiumLedger)
	if !ok {
		return storage.CashReceipt{}, false, fmt.Errorf("premium purchase ledger missing")
	}
	receipt, applied, err := ledger.PurchaseCashPremium(ctx, o, premiumType, durationSecond)
	if err != nil || !applied {
		return receipt, applied, err
	}
	packets, err := shopPilotPackets(receipt, 0, true)
	if err != nil {
		return storage.CashReceipt{}, false, err
	}
	if len(l.keys) != wire.SessionKeyBytes {
		return storage.CashReceipt{}, false, fmt.Errorf("purchase cipher not initialized")
	}
	if _, err = preparePackets(l.keys, packets); err != nil {
		return storage.CashReceipt{}, false, err
	}
	return receipt, applied, nil
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
		packets, e := shopPilotPackets(storage.CashReceipt{CharacterState: state, Deliveries: lines}, 0, true)
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
	prepared := preparedBagLedger{store, s.keys}
	if receipt, applied, handled, err := p.TryPurchaseContract(ctx, prepared, account, character, key, cart); handled || err != nil {
		return receipt, applied, err
	}
	r, applied, e := p.Purchase(ctx, prepared, account, character, key, cart)
	return r, applied, e
}

func shopPilotPackets(receipt storage.CashReceipt, balance uint64, applied bool) ([]outboundPacket, error) {
	var update outboundPacket
	var creatureUpdate *outboundPacket
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
	} else if receipt.Vault != nil {
		payload, err := inventory.VaultPayload(*receipt.Vault)
		if err != nil {
			return nil, err
		}
		update = outboundPacket{"cera_purchase_vault", 0, 13, payload}
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

		if len(b.Special[7]) > 0 {
			hasCreature := false
			for _, d := range receipt.Deliveries {
				if d.Product >= 3300000 && d.Product <= 3300011 {
					hasCreature = true
					break
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
	}
	return packets, nil
}
