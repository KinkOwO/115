package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"dfolan/internal/world"
	"errors"
	"time"
)

func (w *worldSession) areaTransition(r protocol.AreaChangeRequest) (storage.WorldPosition, error) {
	specialWarp := w.specialWarpPending
	w.specialWarpPending = false
	old := w.state.Position
	if w.activeDungeon == nil && w.progression != nil && r.PreviousTown == old.Town && uint32(r.PreviousArea) == old.Area && w.progression.OdysseyJournalTeleport(w.role, r) {
		// The journal route is the source's own progression ladder, but the
		// destination still has to pass the gate the client applies: Storm Pass
		// is [odyssey enter level] 45 for an Odyssey character, not the
		// [need level] 50 that an ordinary character needs.
		if err := w.service.ValidatePosition(w.level, w.odyssey, old); err != nil {
			return old, err
		}
		next := storage.WorldPosition{Town: r.Town, Area: r.Area, X: r.X, Y: r.Y}
		if err := w.service.ValidatePosition(w.level, w.odyssey, next); err != nil {
			return old, err
		}
		return next, nil
	}
	src, _ := w.service.Catalog.Areas[catalog.AreaKey(old.Town, old.Area)]
	dest, _ := w.service.Catalog.Areas[catalog.AreaKey(r.Town, r.Area)]
	isMapTeleport := r.Flag == 5 && (r.TailFlags[0] == 5 || r.TailFlags[1] == 5)
	// Seria's right-hand map selector uses the ordinary map teleport path.
	// Only the exit gate uses the stamped origin as its destination.
	isSeriaReturn := old.Return != nil && src.SeriaReturnWarp && !isMapTeleport
	isSeriaRoomTeleport := dest.SeriaReturnWarp && !src.SeriaReturnWarp
	if w.finalDefenseLineTeleport(r) {
		if err := w.service.ValidateRestoredPosition(w.level, w.odyssey, old); err != nil {
			return old, err
		}
		return w.teleportTransition(old, r)
	}
	if !isSeriaReturn && (specialWarp || isMapTeleport || isSeriaRoomTeleport) {
		return w.teleportTransition(old, r)
	}
	if character.OdysseyRole(w.role) {
		return w.service.TransitionStrict(w.level, w.odyssey, old, r)
	}
	return w.service.Transition(w.level, w.odyssey, old, r)
}

// The quest book sends an ordinary CMD36 from 38/0 to the Fiendwar episode
// town, with no source portal, map-teleport tail, or CMD2261 preparation.
// Its destination requires quest 8645 in the source data; quest 8646 meets
// NPC 619 at this town. Keep this exception scoped to the observed route and
// the character's persisted quest progression.
func (w *worldSession) finalDefenseLineTeleport(r protocol.AreaChangeRequest) bool {
	if w == nil || w.quests == nil || w.quests.Store == nil || w.activeDungeon != nil || w.selectingDungeon ||
		w.role.ID == 0 || w.role.AccountID != w.account ||
		w.state.Position.Town != 38 || w.state.Position.Area != 0 ||
		r.PreviousTown != 38 || r.PreviousArea != 0 || r.Town != 55 || r.Area != 0 ||
		r.Flag != 5 || r.TailFlags != [2]byte{} {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	states, err := w.quests.Store.Quests(ctx, w.account, w.role.ID)
	if err != nil {
		return false
	}
	completed, active := false, false
	for _, state := range states {
		switch state.ID {
		case 8645:
			completed = state.Status == "completed"
		case 8646:
			active = state.Status == "accepted" && state.ConfigVersion == w.quests.Catalog.Source.Checksum
		}
	}
	return completed && active
}

func (w *worldSession) teleportTransition(old storage.WorldPosition, r protocol.AreaChangeRequest) (storage.WorldPosition, error) {
	if w.activeDungeon != nil || w.selectingDungeon {
		return old, errors.New("teleport requires town character")
	}
	if r.PreviousTown != old.Town || uint32(r.PreviousArea) != old.Area {
		return old, errors.New("stale source area")
	}
	dest, exists := w.service.Catalog.Areas[catalog.AreaKey(r.Town, r.Area)]
	if !exists {
		return old, errors.New("unknown destination area")
	}
	if uint32(w.level) < world.RequiredLevel(dest, w.odyssey) {
		return old, world.ErrLevel
	}
	next := storage.WorldPosition{Town: r.Town, Area: r.Area, X: r.X, Y: r.Y}
	if err := w.service.ValidatePosition(w.level, w.odyssey, next); err != nil {
		return old, err
	}
	if dest.SeriaReturnWarp {
		next.Return = &storage.WorldReturn{Town: old.Town, Area: old.Area, X: old.X, Y: old.Y}
	}
	return next, nil
}
