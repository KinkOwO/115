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

// coinReviveTarget carries a session-only companion death without impersonating
// its owner in persistence. Scope is a random room generation, empty for self.
type coinReviveTarget struct {
	actor uint16
	death *odysseyDeath
	scope string
}

func (w *worldSession) ownCoinTarget() coinReviveTarget {
	if w == nil {
		return coinReviveTarget{}
	}
	return coinReviveTarget{actor: w.role.WireID, death: w.pilotDeath}
}
func (v coinReviveTarget) run(w *worldSession) string {
	if v.scope != "" {
		return v.scope
	}
	return w.activeDungeon.RunID
}
func (v coinReviveTarget) key(prefix string) string {
	if v.scope != "" {
		return fmt.Sprintf("%s:robot:%s:%d:%d", prefix, v.scope, v.actor, v.death.Sequence)
	}
	return fmt.Sprintf("%s:%s:%d", prefix, v.death.Run, v.death.Sequence)
}
func (w *worldSession) lifeTokenReviveAllowed(p []byte) error {
	return w.lifeTokenReviveAllowedFor(p, w.ownCoinTarget())
}
func (w *worldSession) lifeTokenReviveAllowedFor(p []byte, target coinReviveTarget) error {
	if w == nil || w.role.ID == 0 || w.activeDungeon == nil || !w.activeDungeon.Loaded || w.resultSent {
		return fmt.Errorf("life token revive requires owned loaded dungeon")
	}
	if len(p) != 8 || target.actor == 0 || target.actor == 65535 || binary.LittleEndian.Uint16(p) != target.actor {
		return fmt.Errorf("revive actor mismatch")
	}
	for _, v := range p[2:] {
		if v != 0 {
			return fmt.Errorf("unsupported coin request options")
		}
	}
	if target.death == nil || target.death.Run != target.run(w) || !target.death.Dead {
		return fmt.Errorf("revive requires confirmed player death")
	}
	if w.dungeons == nil {
		return fmt.Errorf("missing map revive rules")
	}
	if err := w.checkIspinsCoinUse(); err != nil {
		return err
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
	return w.lifeTokenReviveFor(ctx, store, p, frame, w.ownCoinTarget())
}
func (w *worldSession) lifeTokenReviveFor(ctx context.Context, store dungeonReviveStore, p, frame []byte, target coinReviveTarget) ([]outboundPacket, error) {
	hash := sha256.Sum256(frame)
	if w != nil && w.activeDungeon != nil && target.death != nil && target.death.Run == target.run(w) && target.death.Revives[hash] {
		return nil, nil
	}
	if e := w.lifeTokenReviveAllowedFor(p, target); e != nil {
		return nil, e
	}
	if store == nil || w.loot == nil {
		return nil, fmt.Errorf("life token storage unavailable")
	}

	key := target.key("dungeon-life-token-revive")
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
	death, e := protocol.PlayerDeathState(target.actor)
	if e != nil {
		return nil, e
	}
	// Native1452aadea restores full HP/MP for state1; state2 restores one third.
	death[2] = 1
	if target.scope != "" {
		death, _ = protocol.PlayerReviveState(target.actor)
	}
	ack := binary.LittleEndian.AppendUint16([]byte{1}, target.actor)
	saved.WireID = w.role.WireID
	w.role = saved
	if target.death.Revives == nil {
		target.death.Revives = map[[32]byte]bool{}
	}
	target.death.Dead = false
	target.death.Revives[hash] = true
	return []outboundPacket{
		{"life_token_revive_ack", 1, 41, ack},
		{"life_token_revived", 0, 32, death},
		{"life_token_inventory_updated", 0, 14, update},
	}, nil
}

// ceraRevive charges the account once per death when the bag has no Life Token.
func (w *worldSession) ceraRevive(ctx context.Context, store ceraReviveStore, p, frame []byte) ([]outboundPacket, error) {
	return w.ceraReviveFor(ctx, store, p, frame, w.ownCoinTarget())
}
func (w *worldSession) ceraReviveFor(ctx context.Context, store ceraReviveStore, p, frame []byte, target coinReviveTarget) ([]outboundPacket, error) {
	hash := sha256.Sum256(frame)
	if w != nil && w.activeDungeon != nil && target.death != nil && target.death.Run == target.run(w) && target.death.Revives[hash] {
		return nil, nil
	}
	if e := w.lifeTokenReviveAllowedFor(p, target); e != nil {
		return nil, e
	}
	if store == nil {
		return nil, fmt.Errorf("cera revive storage unavailable")
	}
	// Prepare the native packets before the charge. The transaction ceiling below
	// guarantees that the final balance can also be encoded after commit.
	death, e := protocol.PlayerDeathState(target.actor)
	if e != nil {
		return nil, e
	}
	death[2] = 1
	if target.scope != "" {
		death, _ = protocol.PlayerReviveState(target.actor)
	}
	ack := binary.LittleEndian.AppendUint16([]byte{1}, target.actor)
	key := target.key("cera-revive")
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
	if target.death.Revives == nil {
		target.death.Revives = map[[32]byte]bool{}
	}
	target.death.Dead = false
	target.death.Revives[hash] = true
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
	if err := w.checkIspinsCoinUse(); err != nil {
		return nil, err
	}
	// 奥德赛模式口径（业主 2026-10-06，由服务端 mod 打开）：死亡不可复活 ——
	// 这一处挡住下面三级回退全部（测试额度 / 背包复活币 / CERA），
	// 不留"换一档还能复活"的缝。拒绝后走既有死亡超时流程判负回城。
	if e := w.odysseyReviveGate(); e != nil {
		return nil, e
	}
	raid := w != nil && w.bakal != nil && w.activeDungeon != nil && w.activeDungeon.RaidManaged
	if raid {
		if err := w.bakal.CheckCoinBudget(); err != nil {
			return nil, err
		}
	}
	// 末世录（内容 107 / 频道 Type 119）：复活币由**所选作战的 `[allow coin]`** 决定，
	// 且按**关卡**计数 —— PVF `apocalypse.ctp` 的 `[operation data set]` 里只有作战①
	// （难度1）带该列、值 `-1, 8`，作战②/③/④ 都没有该列。业主 2026-10-08 定调：
	// **最后一个数 = 每关上限；无该列 = 禁止复活**。
	//
	// 此前该字段只被解析进 plan.AllowCoin 并写进日志事件，**从未执行**，实机表现就是
	// 业主报的「难度1 能用掉全部 99 个币、难度2 也照样能用」。
	// 与巴卡尔团本同型：先判预算，复活真的成功后再扣。
	apocalypse, apocalypseErr := w.apocalypseCheckCoinBudget()
	if apocalypseErr != nil {
		return nil, apocalypseErr
	}
	plan, err := w.resolveCoinRevive(ctx, store, p, frame, pilotEnabled)
	if apocalypse && err == nil && len(plan) > 0 {
		w.apocalypseSpendCoinBudget()
	}
	if raid && err == nil && len(plan) > 0 {
		w.bakal.SpendCoinBudget()
		plan = append(plan, outboundPacket{"bakal_revive_budget", 0, 2285, w.bakal.PartyFrame(w.bakalLocation, time.Now())})
	}
	if err == nil && len(plan) > 0 {
		// 沉月湖（与蔚蓝号并列的征讨支线）：上限走源 [coin limit]，见 next188。
		plan = w.afterMoonCoinRevive(plan)
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
	if e == nil && len(plan) > 0 && w != nil && w.ispins != nil && w.activeDungeon != nil {
		w.ispins.coinsUsed++
	}
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
