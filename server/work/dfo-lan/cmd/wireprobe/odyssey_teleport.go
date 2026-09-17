package main

import (
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
)

func (w *worldSession) areaTransition(r protocol.AreaChangeRequest) (storage.WorldPosition, error) {
	old := w.state.Position
	if w.activeDungeon == nil && w.progression != nil && r.PreviousTown == old.Town && uint32(r.PreviousArea) == old.Area && w.progression.OdysseyJournalTeleport(w.role, r) {
		if err := w.service.ValidatePosition(w.level, old); err != nil {
			return old, err
		}
		next := storage.WorldPosition{Town: r.Town, Area: r.Area, X: r.X, Y: r.Y}
		if err := w.service.ValidatePosition(w.level, next); err != nil {
			return old, err
		}
		return next, nil
	}
	if character.OdysseyRole(w.role) {
		return w.service.TransitionStrict(w.level, old, r)
	}
	return w.service.Transition(w.level, old, r)
}
