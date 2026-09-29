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
	if w.channelType == 73 && !w.blackPurgatory.prepared &&
		old.Town == 85 && r.Town == 85 && r.PreviousTown == old.Town && uint32(r.PreviousArea) == old.Area &&
		(old.Area == 1 && r.Area == 2 && w.blackPurgatory.created ||
			old.Area == 2 && r.Area == 1 && (w.blackPurgatory.created || w.blackPurgatory.returnToLobby)) {
		// 原生建队/离队传送使用ETC的招募与等待坐标，不要求先走到地图边缘。
		entry := storage.WorldPosition{Town: 85, Area: 2, X: 350, Y: 220}
		if r.Area == 1 {
			entry = storage.WorldPosition{Town: 85, Area: 1, X: 680, Y: 130}
		}
		if r.X == entry.X && r.Y == entry.Y {
			if err := w.service.ValidatePosition(w.level, w.odyssey, entry); err != nil {
				return old, err
			}
			w.blackPurgatory.returnToLobby = false
			return entry, nil
		}
	}
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
	if w.npcMoveTeleport(r) || w.episodeTownReturn(r) {
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

func (w *worldSession) npcMoveTeleport(r protocol.AreaChangeRequest) bool {
	if !w.ownedTownTeleport(r) {
		return false
	}
	from, to := catalog.NPCPlace{Town: r.PreviousTown, Area: uint32(r.PreviousArea)}, catalog.NPCPlace{Town: r.Town, Area: r.Area}
	for _, move := range w.service.Catalog.NPCMoves {
		sources, targets := w.service.Catalog.NPCPlaces[move.NPCID], w.service.Catalog.NPCPlaces[move.TargetNPC]
		if len(targets) != 1 || targets[0] != to {
			continue
		}
		foundSource := false
		for _, source := range sources {
			if source == from {
				foundSource = true
				break
			}
		}
		if !foundSource {
			continue
		}
		if len(move.Quests) == 0 {
			return true
		}
		if w.npcMoveQuestAccepted(move.Quests) {
			return true
		}
	}
	return false
}

func (w *worldSession) npcMoveQuestAccepted(quests []uint32) bool {
	if w.quests == nil || w.quests.Store == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	states, err := w.quests.Store.Quests(ctx, w.account, w.role.ID)
	if err != nil {
		return false
	}
	status := make(map[uint32]string, len(states))
	accepted := make(map[uint32]bool, len(states))
	for _, state := range states {
		status[uint32(state.ID)] = state.Status
		accepted[uint32(state.ID)] = state.Status == "accepted" && state.ConfigVersion == w.quests.Catalog.Source.Checksum
	}
	for _, id := range quests {
		if !accepted[id] {
			continue
		}
		definition, ok := w.quests.Catalog.Quests[id]
		if !ok {
			continue
		}
		if len(definition.PrerequisiteGroups) == 0 {
			return true
		}
		for _, group := range definition.PrerequisiteGroups {
			met := len(group) > 0
			for _, previous := range group {
				if status[previous] != "completed" {
					met = false
					break
				}
			}
			if met {
				return true
			}
		}
	}
	return false
}

func (w *worldSession) episodeTownReturn(r protocol.AreaChangeRequest) bool {
	if !w.ownedTownTeleport(r) {
		return false
	}
	returnArea, ok := w.service.Catalog.EpisodeReturns[r.PreviousTown]
	return ok && returnArea.Town == r.Town && returnArea.Area == r.Area && r.Town != r.PreviousTown
}

func (w *worldSession) ownedTownTeleport(r protocol.AreaChangeRequest) bool {
	return w != nil && w.service != nil && w.activeDungeon == nil && !w.selectingDungeon &&
		w.role.ID != 0 && w.role.AccountID == w.account &&
		w.state.Position.Town == r.PreviousTown && w.state.Position.Area == uint32(r.PreviousArea) &&
		r.Flag == 5 && r.TailFlags == [2]byte{}
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
