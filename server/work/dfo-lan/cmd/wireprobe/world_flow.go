package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/quest"
	"dfolan/internal/storage"
	"dfolan/internal/world"
	"encoding/json"
	"errors"
	"time"
)

type worldSession struct {
	characters         *character.Service
	pilotDeath         *odysseyDeath
	service            *world.Service
	account            int64
	role               storage.Character
	level              byte
	state              storage.WorldState
	flags              [3]byte
	dungeons           *catalog.DungeonCatalog
	tutorials          *catalog.TutorialCatalog
	tutorialDungeons   *catalog.DungeonCatalog
	professions        catalog.Characters
	inTutorial         bool
	fatigue            *character.FatigueService
	lastFatigueDay     string
	quests             *quest.Service
	progression        *character.ProgressionService
	loot               *loot.Service
	vault              *inventory.VaultService
	drops              *loot.Session
	deathSent          map[uint16]bool
	activeDungeon      *dungeon.Session
	selectingDungeon   bool
	completionSent     bool
	resultSent         bool
	cardPlan           *loot.CardPlan
	cardScrolled       bool
	cardLayoutSent     bool
	cardReceipt        *loot.CardReceipt
	answeredQuests     map[uint16]bool
	soloPartyBootstrap bool
	soloPartyReady     bool
	specialWarpPending bool
}

func (w *worldSession) enter(role storage.Character, spawn storage.WorldPosition) error {
	var state character.State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, e := w.service.Enter(ctx, w.account, role.ID, state.Level, spawn)
	if e != nil {
		return e
	}
	w.role, w.level, w.state = role, state.Level, saved
	w.lastFatigueDay = ""
	w.activeDungeon = nil
	w.pilotDeath = nil
	w.soloPartyReady = false
	w.specialWarpPending = false
	w.selectingDungeon = false
	w.completionSent = false
	w.resultSent = false
	w.resetCards()
	w.answeredQuests = nil
	w.drops = nil
	w.deathSent = nil
	return nil
}
func (w *worldSession) areaPayload() ([]byte, error) {
	p := w.state.Position
	return protocol.AreaUsers(p.Town, p.Area, []protocol.AreaUser{{ActorServerID: w.role.WireID, X: p.X, Y: p.Y, Flags: w.flags}})
}
func (w *worldSession) userAreaPayload() ([]byte, error) {
	p := w.state.Position
	return protocol.UserArea(p.Town, p.Area, protocol.AreaUser{ActorServerID: w.role.WireID, X: p.X, Y: p.Y, Flags: w.flags})
}
func (w *worldSession) handle(id uint16, p []byte, send func(byte, uint16, []byte) error, event func(map[string]any)) error {
	if w.role.ID == 0 {
		return errors.New("world request before character selection")
	}
	old := w.state
	next := old.Position
	switch id {
	case 35:
		r, e := protocol.DecodePositionRequest(p)
		if e != nil {
			return e
		}
		next.X, next.Y = r.X, r.Y
		if e = w.service.ValidatePosition(w.level, next); e != nil {
			return e
		}
	case 36:
		w.specialWarpPending = false
		r, e := protocol.DecodeAreaChangeRequest(p)
		if e != nil {
			return e
		}
		next, e = w.areaTransition(r)
		if e != nil {
			event(map[string]any{"kind": "area_refused", "town": r.Town, "area": r.Area, "reason": e.Error()})
			// Code 8 is the native level refusal; code 4 reaches the generic refusal
			// and clears the client's in-flight transition at 145297a9b.
			code := uint16(4)
			if errors.Is(e, world.ErrLevel) {
				code = 8
			}
			refusal, err := protocol.AreaChangeFailure(code, r.Town, r.Area)
			if err != nil {
				return err
			}
			return send(1, 36, refusal)
		}
	default:
		return errors.New("unknown world request")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, e := w.service.Store.SaveWorld(ctx, w.account, w.role.ID, old, next)
	if e != nil {
		return e
	}
	w.state = saved
	event(map[string]any{"kind": "world_position_saved", "character_id": w.role.ID, "position": next, "revision": saved.Revision, "request": id})
	if id == 36 {
		if e = send(1, 36, protocol.AreaChangeSuccess()); e != nil {
			return e
		}
		// NOTI23 is a distinct transition stage: its self branch invokes
		// 146d12cd0 to refresh area-specific warp state and 144f400a0 for
		// area objectives. NOTI24 alone only populated the destination scene.
		userArea, e := w.userAreaPayload()
		if e != nil {
			return e
		}
		if e = send(0, 23, userArea); e != nil {
			return e
		}
		payload, e := w.areaPayload()
		if e != nil {
			return e
		}
		if e = send(0, 24, payload); e != nil {
			return e
		}
		a := w.service.Catalog.Areas[catalog.AreaKey(next.Town, next.Area)]
		event(map[string]any{"kind": "area_change_sent", "town": next.Town, "area": next.Area, "map": a.Map.Path, "sha256": a.Map.SHA256, "client_acceptance": "pending"})
	}
	return w.settleProximityObjectives(ctx, send, event)
}

// Objectives decided by where the character stands settle after the move or
// transition is acknowledged: reaching a source rectangle, and standing at the
// NPC a quest names. Without this a [meet npc] or [reach the range] step stays
// pending forever and its whole chain halts, which reads in game as "the quest
// will not complete" and then "there are no further quests".
func (w *worldSession) settleProximityObjectives(ctx context.Context, send func(byte, uint16, []byte) error, event func(map[string]any)) error {
	if w.quests == nil || w.role.ID == 0 {
		return nil
	}
	advanced, e := w.quests.ProximityProgress(ctx, w.role, w.state.Position, func(npc uint32) ([2]uint16, bool) {
		return w.service.NPCPosition(w.state.Position, npc)
	})
	if e != nil {
		event(map[string]any{"kind": "quest_proximity_error", "character_id": w.role.ID, "error": e.Error()})
		return nil
	}
	if len(advanced) == 0 {
		return nil
	}
	active, e := w.quests.Active(ctx, w.role)
	if e != nil {
		return e
	}
	body, e := protocol.QuestTriggers(active)
	if e != nil {
		return e
	}
	if e = send(0, 291, body); e != nil {
		return e
	}
	event(map[string]any{"kind": "quest_proximity_advanced", "character_id": w.role.ID, "quests": advanced, "position": w.state.Position})
	return nil
}
