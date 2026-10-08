package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

type dungeonReviveStore interface {
	CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(database.Character) (json.RawMessage, json.RawMessage, error)) (database.Character, bool, error)
}

type ceraReviveStore interface {
	dungeonReviveStore
	ApplyGrant(context.Context, database.Grant, func(database.Character) (json.RawMessage, json.RawMessage, error)) (database.GrantResult, error)
}

// The current client's Life Token shop text (dstr 39335) prices one token at 15 CERA.
const lifeTokenCeraCost int64 = 15

func (w *worldSession) lifeTokenReviveAllowed(p []byte) error {
	if w == nil || w.role.ID == 0 || w.activeDungeon == nil || !w.activeDungeon.Loaded || w.resultSent {
		return fmt.Errorf("life token revive requires owned loaded dungeon")
	}
	if len(p) != 8 || w.role.WireID == 0 || w.role.WireID == 65535 || binary.LittleEndian.Uint16(p) != w.role.WireID {
		return fmt.Errorf("revive actor mismatch")
	}
	for _, v := range p[2:] {
		if v != 0 {
			return fmt.Errorf("unsupported coin request options")
		}
	}
	if w.pilotDeath == nil || w.pilotDeath.Run != w.activeDungeon.RunID || !w.pilotDeath.Dead {
		return fmt.Errorf("revive requires confirmed player death")
	}
	if w.dungeons == nil {
		return fmt.Errorf("missing map revive rules")
	}
	script, err := w.dungeons.MapScript(w.activeDungeon.Room.Map)
	if err != nil {
		return err
	}
	for _, v := range script.Cells {
		if v.Type == 3 && v.Text == "[cannot use coin map]" {
			return fmt.Errorf("source map forbids coin revival")
		}
	}
	return nil
}

// lifeTokenRevive consumes one ordinary bag life token and restores the actor.
// The event key is scoped to the dungeon death sequence so a repeated CMD41
// cannot spend a second token, even when the character enters another run.
func (w *worldSession) lifeTokenRevive(ctx context.Context, store dungeonReviveStore, p, frame []byte) ([]outboundPacket, error) {
	hash := sha256.Sum256(frame)
	if w != nil && w.activeDungeon != nil && w.pilotDeath != nil && w.pilotDeath.Run == w.activeDungeon.RunID && w.pilotDeath.Revives[hash] {
		return nil, nil
	}
	if e := w.lifeTokenReviveAllowed(p); e != nil {
		return nil, e
	}
	if store == nil || w.loot == nil {
		return nil, fmt.Errorf("life token storage unavailable")
	}

	key := fmt.Sprintf("dungeon-life-token-revive:%s:%d", w.pilotDeath.Run, w.pilotDeath.Sequence)
	saved, _, e := store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "dungeon-life-token-revive-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		bag, err := inventory.ReadBag(current.State)
		if err != nil {
			return nil, nil, err
		}
		bag, remaining, err := bag.Consume(w.loot.Catalog, 1, 1)
		if err != nil {
			return nil, nil, err
		}
		updated, err := inventory.SaveBag(current.State, bag)
		if err != nil {
			return nil, nil, err
		}
		receipt, err := json.Marshal(map[string]uint32{"before": remaining + 1, "after": remaining})
		return updated, receipt, err
	})
	if e != nil {
		return nil, e
	}

	beforeBag, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, e
	}
	bag, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	update, e := protocol.InventoryUpdate(inventory.ChangedItemRows(beforeBag, bag))
	if e != nil {
		return nil, e
	}
	death, e := protocol.PlayerDeathState(w.role.WireID)
	if e != nil {
		return nil, e
	}
	// Native1452aadea restores full HP/MP for state1; state2 restores one third.
	death[2] = 1
	ack := binary.LittleEndian.AppendUint16([]byte{1}, w.role.WireID)
	saved.WireID = w.role.WireID
	w.role = saved
	if w.pilotDeath.Revives == nil {
		w.pilotDeath.Revives = map[[32]byte]bool{}
	}
	w.pilotDeath.Dead = false
	w.pilotDeath.Revives[hash] = true
	return []outboundPacket{
		{"life_token_revive_ack", 1, 41, ack},
		{"life_token_revived", 0, 32, death},
		{"life_token_inventory_updated", 0, 14, update},
	}, nil
}

// ceraRevive charges the account once per death when the bag has no Life Token.
func (w *worldSession) ceraRevive(ctx context.Context, store ceraReviveStore, p, frame []byte) ([]outboundPacket, error) {
	hash := sha256.Sum256(frame)
	if w != nil && w.activeDungeon != nil && w.pilotDeath != nil && w.pilotDeath.Run == w.activeDungeon.RunID && w.pilotDeath.Revives[hash] {
		return nil, nil
	}
	if e := w.lifeTokenReviveAllowed(p); e != nil {
		return nil, e
	}
	if store == nil {
		return nil, fmt.Errorf("cera revive storage unavailable")
	}
	// Prepare the native packets before the charge. The transaction ceiling below
	// guarantees that the final balance can also be encoded after commit.
	death, e := protocol.PlayerDeathState(w.role.WireID)
	if e != nil {
		return nil, e
	}
	death[2] = 1
	ack := binary.LittleEndian.AppendUint16([]byte{1}, w.role.WireID)
	key := fmt.Sprintf("cera-revive:%s:%d", w.pilotDeath.Run, w.pilotDeath.Sequence)
	charged, e := store.ApplyGrant(ctx, database.Grant{
		ID: key, AccountID: w.role.AccountID, Cera: -lifeTokenCeraCost,
		Operator: "system", Reason: "life-token-less revive (dstr 7689)", MaxCera: math.MaxInt32,
	}, nil)
	if e != nil {
		return nil, e
	}
	balance, e := protocol.CeraBalance(charged.Cera)
	if e != nil {
		// ApplyGrant commits only a client-encodable balance for this operation.
		return nil, e
	}
	if w.pilotDeath.Revives == nil {
		w.pilotDeath.Revives = map[[32]byte]bool{}
	}
	w.pilotDeath.Dead = false
	w.pilotDeath.Revives[hash] = true
	return []outboundPacket{
		{"cera_revive_ack", 1, 41, ack},
		{"cera_revived", 0, 32, death},
		{"cera_balance_after_revive", 0, 53, balance},
	}, nil
}

// useCoinRevive only falls through on a confirmed empty credit or coin stack.
func (w *worldSession) useCoinRevive(ctx context.Context, store ceraReviveStore, p, frame []byte, pilotEnabled bool) ([]outboundPacket, error) {
	if w != nil && w.activeDungeon != nil && w.pilotDeath != nil && w.pilotDeath.Run == w.activeDungeon.RunID && w.pilotDeath.Revives[sha256.Sum256(frame)] {
		return nil, nil
	}
	if e := w.odysseyReviveGate(); e != nil {
		return nil, e
	}
	raid := w != nil && w.bakal != nil && w.activeDungeon != nil && w.activeDungeon.RaidManaged
	if raid {
		if err := w.bakal.CheckCoinBudget(); err != nil {
			return nil, err
		}
	}
	plan, err := w.resolveCoinRevive(ctx, store, p, frame, pilotEnabled)
	if raid && err == nil && len(plan) > 0 {
		w.bakal.SpendCoinBudget()
		plan = append(plan, outboundPacket{"bakal_revive_budget", 0, 2285, w.bakal.PartyFrame(w.bakalLocation, time.Now())})
	}
	return plan, err
}

func (w *worldSession) resolveCoinRevive(ctx context.Context, store ceraReviveStore, p, frame []byte, pilotEnabled bool) ([]outboundPacket, error) {
	if w != nil && w.activeDungeon != nil && w.pilotDeath != nil && w.pilotDeath.Run == w.activeDungeon.RunID && w.pilotDeath.Revives[sha256.Sum256(frame)] {
		return nil, nil
	}
	if pilotEnabled {
		plan, e := w.pilotRevive(ctx, store, p, frame)
		if e == nil || !errors.Is(e, errOdysseyCreditsExhausted) {
			return w.afterAzureCoinRevive(plan, e)
		}
	}
	plan, e := w.lifeTokenRevive(ctx, store, p, frame)
	if e == nil || !errors.Is(e, inventory.ErrCoinStackEmpty) {
		return w.afterAzureCoinRevive(plan, e)
	}
	return w.afterAzureCoinRevive(w.ceraRevive(ctx, store, p, frame))
}

// afterAzureCoinRevive 在蔚蓝号里为「用币复活成功」扣一次额度并把 N2621 刷出去。
//
// N2621 的 [32:36] 是**剩余复活次数**（上限 8）。此前写死成 8，所以用币复活后客户端
// 看到的次数不动（业主实机 2026-10-04）。这里只在真的复活成功（plan 非空、无错）时扣，
// 且只在蔚蓝号频道生效；扣到 0 之后仍会回帧（客户端据此把入口灰掉）。
func (w *worldSession) afterAzureCoinRevive(plan []outboundPacket, e error) ([]outboundPacket, error) {
	if e != nil || len(plan) == 0 || w == nil || w.channelType != azureMainChannelType {
		return plan, e
	}
	if w.azure.revivesLeft == 0 {
		return plan, e
	}
	w.azure.revivesLeft--
	return append(plan, w.azureMainInfoNow(azureMainPhasePlaying)), nil
}

func boosterActionRefusal(id uint16) []byte {
	if id == 41 {
		return protocol.Refusal(22) // The native USE_COIN failure arm silently ignores code 4.
	}
	return protocol.Refusal(4)
}
