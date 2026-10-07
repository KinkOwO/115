package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/world"
	"errors"
	"time"
)

func (w *worldSession) areaTransition(r protocol.AreaChangeRequest) (database.WorldPosition, error) {
	specialWarp := w.specialWarpPending
	w.specialWarpPending = false
	old := w.state.Position
	// Starter Boost 662：活动城镇里的区域移动只认源里当前关的授权房
	// （[check event condition]/[event id]/[condition]），不走普通门控。
	if pos, handled, e := w.boostAreaTransition(r); handled {
		return pos, e
	}
	if w.channelType == 73 && !w.blackPurgatory.prepared &&
		old.Town == 85 && r.Town == 85 && r.PreviousTown == old.Town && uint32(r.PreviousArea) == old.Area &&
		(old.Area == 1 && r.Area == 2 && w.blackPurgatory.created ||
			old.Area == 2 && r.Area == 1 && (w.blackPurgatory.created || w.blackPurgatory.returnToLobby)) {
		// 原生建队/离队传送使用ETC的招募与等待坐标，不要求先走到地图边缘。
		entry := database.WorldPosition{Town: 85, Area: 2, X: 350, Y: 220}
		if r.Area == 1 {
			entry = database.WorldPosition{Town: 85, Area: 1, X: 680, Y: 130}
		}
		if r.X == entry.X && r.Y == entry.Y {
			if err := w.service.ValidatePosition(w.level, w.odyssey, entry); err != nil {
				return old, err
			}
			w.blackPurgatory.returnToLobby = false
			return entry, nil
		}
	}
	if w.channelType == 101 && w.moonConfig != nil &&
		old.Town == 215 && r.Town == 215 && r.PreviousTown == old.Town && uint32(r.PreviousArea) == old.Area &&
		(old.Area == 1 && r.Area == 2 || old.Area == 2 && r.Area == 1) {
		// 月湖红门等候区往返（2026-10-04 实测：PVF 直读世界目录的 215/1 门矩形
		// 与客户端红门落点不一致，area_refused「no authorized source portal」挡死
		// 整条月湖流程）。按黑鸦 85/1↔85/2 同款先例：Moon 频道上 215/1↔215/2
		// 直接接受，落点优先用客户端坐标，不可走时回退 moonlake.cos 官方坐标
		//（等候区红门 217,212 / 招募区 1264,325，见修复记录-20261001 §3.1）。
		next := database.WorldPosition{Town: 215, Area: r.Area, X: r.X, Y: r.Y}
		if err := w.service.ValidatePosition(w.level, w.odyssey, next); err != nil {
			if r.Area == 2 {
				next = database.WorldPosition{Town: 215, Area: 2, X: 217, Y: 212}
			} else {
				next = database.WorldPosition{Town: 215, Area: 1, X: 1264, Y: 325}
			}
			if err := w.service.ValidatePosition(w.level, w.odyssey, next); err != nil {
				return old, err
			}
		}
		return next, nil
	}
	if _, isContent := w.channelSpawns[w.channelType]; isContent &&
		old.Town == r.Town && r.PreviousTown == old.Town && uint32(r.PreviousArea) == old.Area && uint32(r.Area) != old.Area {
		// 内容频道的城镇门（2026-10-04 幽暗岛实测：178/1→178/2 被拒
		//「no authorized source portal」——PVF 直读的门矩形与客户端实际门位置
		// 不一致，同月湖红门问题）。客户端自己知道门在哪：同城镇相邻区域的
		// 走门请求直接接受。
		//
		// 落点（2026-10-04 第二轮实测）：不能用客户端在**旧图**的坐标当新图落点
		// ——幽暗岛把角色放进了对方阵营起始区，客户端弹「You cannot enter to
		// the Opposing Faction's starting point」。正确落点 = 目的区域里指向
		// 来路的门的矩形中心（门的另一侧，天然合法站位）。目的区域不存在或
		// 没有回程门、或落点不可走时维持拒绝。
		dest, exists := w.service.Catalog.Areas[catalog.AreaKey(r.Town, r.Area)]
		if !exists {
			return old, errors.New("unknown destination area")
		}
		lx, ly, have := 0, 0, false
		for _, p := range dest.Portals {
			if p.Town == old.Town && p.Area == uint32(old.Area) {
				lx = int((p.Bounds[0] + p.Bounds[2]) / 2)
				ly = int((p.Bounds[1] + p.Bounds[3]) / 2)
				have = true
				break
			}
		}
		if !have {
			return old, errors.New("destination area has no return portal")
		}
		next := database.WorldPosition{Town: r.Town, Area: r.Area, X: uint16(lx), Y: uint16(ly)}
		if err := w.service.ValidatePosition(w.level, w.odyssey, next); err != nil {
			return old, err
		}
		return next, nil
	}
	if w.activeDungeon == nil && w.progression != nil && r.PreviousTown == old.Town && uint32(r.PreviousArea) == old.Area && w.progression.OdysseyJournalTeleport(w.role, r) {
		// The journal route is the source's own progression ladder, but the
		// destination still has to pass the gate the client applies: Storm Pass
		// is [odyssey enter level] 45 for an Odyssey character, not the
		// [need level] 50 that an ordinary character needs.
		if err := w.service.ValidatePosition(w.level, w.odyssey, old); err != nil {
			return old, err
		}
		next := database.WorldPosition{Town: r.Town, Area: r.Area, X: r.X, Y: r.Y}
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
	if pos, handled, e := w.boostChallengeGoTeleport(r, specialWarp); handled {
		return pos, e
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
		accepted[uint32(state.ID)] = state.Status == "accepted" && state.ConfigVersion == w.quests.Catalog.Source.SaveIdentity()
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

func (w *worldSession) teleportTransition(old database.WorldPosition, r protocol.AreaChangeRequest) (database.WorldPosition, error) {
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
	next := database.WorldPosition{Town: r.Town, Area: r.Area, X: r.X, Y: r.Y}
	if err := w.service.ValidatePosition(w.level, w.odyssey, next); err != nil {
		return old, err
	}
	if dest.SeriaReturnWarp {
		next.Return = &database.WorldReturn{Town: old.Town, Area: old.Area, X: old.X, Y: old.Y}
	}
	return next, nil
}
