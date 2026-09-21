package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"dfolan/internal/world"
	"errors"
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
	isSeriaReturn := old.Return != nil && src.SeriaReturnWarp && old.Return.Town == r.Town && old.Return.Area == r.Area
	isMapTeleport := r.Flag == 5 && (r.TailFlags[0] == 5 || r.TailFlags[1] == 5)
	isSeriaRoomTeleport := dest.SeriaReturnWarp && !src.SeriaReturnWarp
	if !isSeriaReturn && (specialWarp || isMapTeleport || isSeriaRoomTeleport) {
		return w.teleportTransition(old, r)
	}
	if character.OdysseyRole(w.role) {
		return w.service.TransitionStrict(w.level, w.odyssey, old, r)
	}
	return w.service.Transition(w.level, w.odyssey, old, r)
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
