package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

func (w *worldSession) oathInfo(ctx context.Context, level, option int) ([]byte, error) {
	truth, err := w.store.OathTruth(ctx, w.account, w.role.ID)
	if err != nil {
		return nil, err
	}
	if w.items != nil && w.items.Truth != nil {
		if err = inventory.ValidateOathTruth(truth, w.items.Truth); err != nil {
			return nil, err
		}
	}
	return protocol.OathSystemInfoWithTruth(level, option, truth.Level, truth.Options)
}
func (w *worldSession) oathTruthPackets(ctx context.Context, id uint16, body []byte) ([]outboundPacket, error) {
	failure := []outboundPacket{{"oath_truth_rejected", 1, id, protocol.OathTruthFailure()}}
	if w == nil || w.store == nil || w.items == nil || w.items.Truth == nil || w.role.ID == 0 {
		return failure, fmt.Errorf("truth service unavailable")
	}
	if w.activeDungeon != nil {
		return failure, fmt.Errorf("truth changes require town")
	}
	if w.items.Truth.Source != w.items.Catalog.Source.Checksum {
		return failure, fmt.Errorf("truth PVF source mismatch")
	}
	var ack []byte
	if id == 2420 {
		r, err := protocol.DecodeOathTruthUpgrade(body)
		if err != nil {
			return failure, err
		}
		saved, _, err := w.store.UpgradeOathTruth(ctx, w.account, w.role.ID, w.role.ConfigVersion, w.items.Truth, r.Level, r.Payment)
		if err != nil {
			return failure, err
		}
		saved.WireID = w.role.WireID
		w.role = saved
		ack = protocol.OathTruthUpgradeReply(r.Level, r.Payment)
	} else {
		r, err := protocol.DecodeOathTruthSelect(body)
		if err != nil {
			return failure, err
		}
		saved, err := w.store.SelectOathTruth(ctx, w.account, w.role.ID, w.role.ConfigVersion, w.items.Truth, r.Slot, r.Option)
		if err != nil {
			return failure, err
		}
		saved.WireID = w.role.WireID
		w.role = saved
		ack = protocol.OathTruthSelectReply(r.Slot, r.Option)
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return failure, err
	}
	payload, err := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	if err != nil {
		return failure, err
	}
	plan := []outboundPacket{{"oath_truth_inventory_refreshed", 0, 13, payload}}
	// Publish authoritative state before the ack callback starts its animation.
	plan = append(plan, w.oathDisplayRefresh(ctx, "oath_truth_state_refreshed")...)
	if id == 2420 && w.vault != nil && w.vault.Rules.Account != nil {
		vault, e := w.store.LoadAccountVault(ctx, w.account, w.role.ID)
		if e != nil {
			return failure, e
		}
		payload, e := inventory.AccountVaultPayload(vault, *w.vault.Rules.Account)
		if e != nil {
			return failure, e
		}
		plan = append(plan, outboundPacket{"oath_truth_vault_refreshed", 0, 13, payload})
		personal, e := w.store.OathTruthVaults(ctx, w.account, w.role.ID)
		if e != nil {
			return failure, e
		}
		for i, v := range personal {
			if v.Slots == 0 {
				continue
			}
			var items []inventory.VaultItem
			if e = json.Unmarshal(v.Items, &items); e != nil {
				return failure, e
			}
			rows := make([][protocol.CurrentItemRecordSize]byte, 0, len(items))
			for _, item := range items {
				rows = append(rows, item.Row())
			}
			space := byte(2)
			if i == 1 {
				space = 45
			}
			payload, e := protocol.PersonalVaultSpace(space, v.Slots, rows)
			if e != nil {
				return failure, e
			}
			plan = append(plan, outboundPacket{"oath_truth_personal_vault_refreshed", 0, 13, payload})
		}
	}
	plan = w.appendFameUpdate(plan, nil)
	return append(plan, outboundPacket{"oath_truth_accepted", 1, id, ack}), nil
}
