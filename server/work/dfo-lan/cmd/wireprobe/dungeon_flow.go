package main

import (
	"context"
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"
)

func randomSeed() (uint32, error) {
	var seed uint32
	e := binary.Read(rand.Reader, binary.LittleEndian, &seed)
	return seed, e
}

func (w *worldSession) dungeonGate(p []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("dungeon gate without selected character")
	}
	requested, e := protocol.DecodeDungeonGate(p)
	if e != nil {
		return nil, e
	}
	// The town gate sends zero. The client's own tutorial sender (146cce650)
	// supplies a source dungeon ID instead; accept that only when it is this
	// character's own starting route and the route is still owed.
	if requested != 0 {
		if e = w.authorizeTutorial(requested); e != nil {
			return nil, e
		}
		return []outboundPacket{
			{"tutorial_gate_ack", 1, 15, []byte{1}},
			{"dungeon_selection_sent", 0, 27, protocol.EnterDungeonSelection()},
		}, nil
	}
	a, ok := w.service.Catalog.Areas[catalog.AreaKey(w.state.Position.Town, w.state.Position.Area)]
	if !ok || a.Kind != "[dungeon gate]" {
		return nil, fmt.Errorf("source area is not a PVF dungeon gate")
	}
	// The gate belongs to the area the character already stands in.
	if e = w.service.ValidateRestoredPosition(w.level, w.odyssey, w.state.Position); e != nil {
		return nil, e
	}
	return []outboundPacket{
		{"dungeon_gate_ack", 1, 15, []byte{1}},
		{"dungeon_selection_sent", 0, 27, protocol.EnterDungeonSelection()},
	}, nil
}

func (w *worldSession) selectDungeon(p []byte) (*dungeon.Session, []outboundPacket, error) {
	if w == nil || w.dungeons == nil || w.role.ID == 0 {
		return nil, nil, fmt.Errorf("dungeon catalog or character unavailable")
	}
	if w.activeDungeon != nil {
		return nil, nil, fmt.Errorf("dungeon entry already active")
	}
	r, e := protocol.DecodeDungeonSelection(p)
	if e != nil {
		return nil, nil, e
	}
	if d, ok := w.dungeons.Dungeons[r.ID]; ok && d.Odyssey {
		if !character.OdysseyRole(w.role) {
			return nil, nil, fmt.Errorf("Odyssey dungeon requires an Odyssey character")
		}
	}
	// A starting route names its own dungeon and has no town gate to stand
	// at, so it is resolved before the ordinary gate check.
	if w.tutorialDungeons != nil {
		if _, ok := w.tutorialDungeons.Dungeons[r.ID]; ok {
			return w.selectTutorial(r.ID)
		}
	}
	if _, e := w.dungeonGate(make([]byte, 8)); e != nil {
		return nil, nil, e
	}
	if w.soloPartyReady && r.Party == 1 {
		// This connection owns the single-member bootstrap party. The dungeon
		// domain remains solo; never normalize arbitrary party IDs.
		r.Party = 65535
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	accepted, e := w.acceptedQuestIDs(ctx)
	if e != nil {
		return nil, nil, e
	}
	s, e := dungeon.Select(*w.dungeons, r, w.level, accepted)
	if e != nil {
		return nil, nil, e
	}
	if w.fatigue != nil && !s.Definition.NoFatigue && w.fatigue.Rules.RoomCost > 0 {
		fp, err := w.fatigue.State(ctx, w.account, w.role.ID, time.Now())
		if err != nil {
			return nil, nil, err
		}
		if fp.Used >= fp.Limit {
			return nil, nil, storage.ErrFatigueExhausted
		}
	}
	plan, e := w.dungeonEntryPlan("dungeon_select_ack", 16, r, s)
	if e != nil {
		return nil, nil, e
	}
	return s, plan, nil
}

// dungeonEntryPlan is the frame sequence the client needs to load a dungeon it
// has just asked for: the acknowledgement for that request, the actor display
// state, the solo party bootstrap, NOTI 28 (dungeon info) and NOTI 29 (start
// map). Both the town selection (CMD 16) and the post-clear "next story
// dungeon" gate (CMD 2062) replay it unchanged - only the ack id differs,
// because the client loads a whole new dungeon either way.
func (w *worldSession) dungeonEntryPlan(ackName string, ackID uint16, sel protocol.DungeonSelection, s *dungeon.Session) ([]outboundPacket, error) {
	var seed uint32
	if e := binary.Read(rand.Reader, binary.LittleEndian, &seed); e != nil {
		return nil, e
	}
	start, e := protocol.StartMap(protocol.StartMapState{Position: s.Maze.Start, Seed: seed, Map: s.Room.Map, Monsters: s.Monsters})
	if e != nil {
		return nil, e
	}
	plan := []outboundPacket{{ackName, 1, ackID, []byte{1}}}
	if w.characters != nil {
		visual, err := w.characters.EntryBasicProbe(w.role, [2]byte{})
		if err == nil {
			plan = append(plan, outboundPacket{"dungeon_actor_appearance_sent", 0, 2, visual})
		}
		addition, err := w.characters.EntryAddition(w.role)
		if err == nil {
			plan = append(plan, outboundPacket{"dungeon_actor_addition_sent", 0, 2, addition})
		}
		wornUpdate, err := inventory.WornSpaceUpdate(w.role.State)
		if err == nil && len(wornUpdate) > 0 {
			plan = append(plan, outboundPacket{"dungeon_worn_visuals_sent", 0, 14, wornUpdate})
		}
		if inventory.HasEquippedCreature(w.role.State) {
			clPayload, err := inventory.CreatureListPayload(w.role.State)
			if err == nil {
				plan = append(plan, outboundPacket{"dungeon_creature_list_sent", 0, 105, clPayload})
				plan = append(plan, outboundPacket{"dungeon_creature_growth_sent", 0, 102, []byte{1, 0, 0, 0, 0, 0}})
			}
		}
	}
	if w.soloPartyBootstrap {
		party, e := protocol.SoloPartyInfo(w.role.WireID)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"solo_party_initialized", 0, 9, party})
	}
	plan = append(plan, []outboundPacket{
		{"dungeon_info_sent", 0, 28, protocol.DungeonInfo(protocol.DungeonInfoState{ID: sel.ID, Difficulty: sel.Difficulty, Maze: s.Maze.Index, Boss: s.Maze.Boss})},
		{"dungeon_start_map_sent", 0, 29, start},
	}...)
	return plan, nil
}

// directMoveDungeon handles CMD 2062 (ENUM_CMDPACKET_DUNGEON_DIRECT_MOVE): the
// "next story dungeon" gate the client offers beside "return to town" after a
// clear. The finished run is replaced by the requested one, with the same
// source gate keeping as the town selection (level, difficulty, maze quest)
// and the same entry sequence, because the client loads a whole new dungeon
// from this request. The town-only checks of CMD 16 (standing on a PVF
// [dungeon gate] area) do not apply inside a run.
// acceptedQuestIDs is the prerequisite set the dungeon gate check consumes.
// The client's own wording is "accepted **or** completed prerequisite quests",
// so completed ones count too; only accepted would lock out a dungeon whose
// quest the character already finished.
func (w *worldSession) acceptedQuestIDs(ctx context.Context) (map[uint16]bool, error) {
	quests, e := w.service.Store.Quests(ctx, w.account, w.role.ID)
	if e != nil {
		return nil, e
	}
	accepted := map[uint16]bool{}
	for _, q := range quests {
		if (q.Status == "accepted" || q.Status == "completed") && q.ConfigVersion == w.dungeons.Source.Checksum {
			accepted[q.ID] = true
		}
	}
	return accepted, nil
}

func (w *worldSession) directMoveDungeon(p []byte) (*dungeon.Session, []outboundPacket, error) {
	if w == nil || w.dungeons == nil || w.role.ID == 0 {
		return nil, nil, fmt.Errorf("dungeon catalog or character unavailable")
	}
	if w.activeDungeon == nil {
		return nil, nil, fmt.Errorf("direct move without an active dungeon")
	}
	r, e := protocol.DecodeDungeonDirectMove(p)
	if e != nil {
		return nil, nil, e
	}
	d, ok := w.dungeons.Dungeons[r.ID]
	if !ok {
		return nil, nil, fmt.Errorf("direct move target absent from imported source")
	}
	if d.Odyssey && !character.OdysseyRole(w.role) {
		return nil, nil, fmt.Errorf("Odyssey dungeon requires an Odyssey character")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if w.fatigue != nil && !d.NoFatigue && w.fatigue.Rules.RoomCost > 0 {
		fp, err := w.fatigue.State(ctx, w.account, w.role.ID, time.Now())
		if err != nil {
			return nil, nil, err
		}
		if fp.Used >= fp.Limit {
			return nil, nil, storage.ErrFatigueExhausted
		}
	}
	accepted, e := w.acceptedQuestIDs(ctx)
	if e != nil {
		return nil, nil, e
	}
	// The direct-move body carries the dungeon id and difficulty only, so the
	// remaining selection fields take the values CMD 16 uses for a solo run.
	sel := protocol.DungeonSelection{ID: r.ID, Difficulty: r.Difficulty, Party: 65535}
	s, e := dungeon.Select(*w.dungeons, sel, w.level, accepted)
	if e != nil {
		return nil, nil, e
	}
	plan, e := w.directMoveEntryPlan(sel, s)
	if e != nil {
		return nil, nil, e
	}
	// 客户端在「清关 → 点下一个剧情关卡门」后不再接受普通房间门（实机 A/B：它收到的
	// 进图帧与城镇选图逐字节一致，却整个下一关都不发 MOVE_MAP，取证见
	// docs/protocol/next49-odyssey-direct-move.md）。清关后客户端另有一个「选择其他
	// 地下城」入口，走的是 gate_ack(15) + selection_sent(27) 这条 UI 层帧；先进这个
	// 界面再下发进图帧，避免用"回城帧"造成的场景切换与进图撞车（实测那会让客户端
	// 黑屏退出）。
	head := []outboundPacket{
		{"dungeon_gate_ack", 1, 15, []byte{1}},
		{"dungeon_selection_sent", 0, 27, protocol.EnterDungeonSelection()},
	}
	return s, append(head, plan...), nil
}

// directMoveEntryPlan is the CMD 2062 form of dungeonEntryPlan: the client
// starts this run from its own direct-move request, and the town selection
// sequence is what its dungeon context expects.
//
// Live A/B in one session (2026-09-21, dungeon 100004950 "安徒恩讨伐战"):
//
//	15:38:03 entered via CMD 2062 and the server answered
//	         dungeon_direct_move_ack(2062) + dungeon_select_ack(16):
//	         the first room advanced, but after the layer change to 100016165
//	         the client stopped sending room requests altogether and the run
//	         stalled there.
//	15:41:09 entered via the town gate (CMD 15/16), server answered
//	         dungeon_select_ack(16) only: first room, the same layer map and
//	         all ten rooms up to the boss room 100016175 advanced normally.
//
// The two entry sequences differ by nothing but that 2062 acknowledgement, so
// it is dropped here: the direct move replays the town selection entry exactly.
func (w *worldSession) directMoveEntryPlan(sel protocol.DungeonSelection, s *dungeon.Session) ([]outboundPacket, error) {
	return w.dungeonEntryPlan("dungeon_select_ack", 16, sel, s)
}

func (w *worldSession) finishDungeonLoading(p []byte) ([]outboundPacket, error) {
	if w == nil || w.activeDungeon == nil {
		return nil, fmt.Errorf("loading without dungeon session")
	}
	// Current solo loading request is a zeroed 16-byte record. Refuse other
	// options until their native send fields are supported.
	if len(p) != 16 {
		return nil, fmt.Errorf("unexpected loading record length")
	}
	for _, b := range p {
		if b != 0 {
			return nil, fmt.Errorf("unsupported loading option")
		}
	}
	state, e := protocol.UserState(w.role.WireID, protocol.UserStateDungeon)
	if e != nil {
		return nil, e
	}
	plan := []outboundPacket{{"dungeon_loading_ack", 1, 37, []byte{1}}, {"dungeon_actor_state", 0, 3, state}, {"dungeon_loading_complete", 0, 30, protocol.DungeonLoaded()}}
	if w.progression != nil {
		p, err := character.ExperiencePayload(w.role)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"dungeon_experience_restored", 0, 37, p})
	}
	if w.fatigue != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		d := w.activeDungeon
		fp, _, e := w.fatigue.EnterRoom(ctx, w.account, w.role.ID, d.RunID, d.Room.Map, d.Definition.NoFatigue, time.Now())
		if e != nil {
			return nil, e
		}
		p, e := protocol.Fatigue(fp.Used, fp.Limit, fp.UsedMax)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"dungeon_fatigue_updated", 0, 36, p})
	}
	if w.characters != nil {
		wornUpdate, err := inventory.WornSpaceUpdate(w.role.State)
		if err == nil && len(wornUpdate) > 0 {
			plan = append(plan, outboundPacket{"dungeon_worn_visuals_restored", 0, 14, wornUpdate})
		}
		if inventory.HasEquippedCreature(w.role.State) {
			clPayload, err := inventory.CreatureListPayload(w.role.State)
			if err == nil {
				plan = append(plan, outboundPacket{"dungeon_creature_list_restored", 0, 105, clPayload})
				plan = append(plan, outboundPacket{"dungeon_creature_growth_restored", 0, 102, []byte{1, 0, 0, 0, 0, 0}})
			}
		}
	}
	if w.activeDungeon.Definition.ID == 100003126 {
		// Elvenmere 初始化层数：根据进图选取的 Zone（Extra）设置当前层与最高已通关层。
		floor := byte(w.activeDungeon.Extra)
		if floor == 0 {
			floor = 1
		}
		var maxCleared byte
		if floor > 1 {
			maxCleared = floor - 1
		}
		plan = append(plan, outboundPacket{"elvenmere_info_sent", 0, 2193, protocol.ElvenmereInfo(floor, maxCleared, w.activeDungeon.WeeklyRewards, w.activeDungeon.SeasonRewards)})
	}
	return plan, nil
}

func (w *worldSession) elvenmereTeleport(p []byte) ([]outboundPacket, error) {
	if w == nil || w.activeDungeon == nil {
		return nil, fmt.Errorf("teleport without active dungeon session")
	}
	if w.activeDungeon.Definition.ID != 100003126 {
		return nil, fmt.Errorf("elvenmere teleport in non-elvenmere dungeon")
	}
	if len(p) < 25 {
		return nil, fmt.Errorf("short elvenmere teleport payload")
	}

	// 刚刚通关的层数
	clearedFloor := byte(w.activeDungeon.Extra)
	if clearedFloor == 0 {
		clearedFloor = 1
	}

	// 传送至下一层
	nextFloor := clearedFloor + 1
	if nextFloor > 100 {
		nextFloor = 100
	}
	w.activeDungeon.Extra = uint16(nextFloor)
	maxCleared := clearedFloor

	plan := []outboundPacket{
		{"elvenmere_teleport_ack", 1, 2015, []byte{1}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. 发放通关道具奖励（Seed Coin / 叶之金币及赛季奖励）
	// 周常奖励
	if clearedFloor >= 1 && clearedFloor <= 100 && w.activeDungeon.WeeklyRewards[clearedFloor-1] == 0 {
		w.activeDungeon.WeeklyRewards[clearedFloor-1] = 1
		itemTemplate, itemCount := dungeon.ElvenmereFloorWeeklyReward(clearedFloor)
		if itemTemplate != 0 && itemCount > 0 && w.loot != nil {
			awarder := &inventory.Awarder{
				Catalog:   w.loot.Catalog,
				Rules:     w.loot.BagRules,
				Equipment: w.loot.Equipment,
			}
			if updated, _, err := awarder.Grant(w.role.State, itemTemplate, itemCount); err == nil {
				w.role.State = updated
				if store := w.service.Store; store != nil {
					key := fmt.Sprintf("elvenmere-weekly:%s:%d", w.activeDungeon.RunID, clearedFloor)
					_, _, _ = store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "elvenmere-reward-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
						proof, _ := json.Marshal(map[string]any{"template": itemTemplate, "amount": itemCount, "floor": clearedFloor})
						return updated, proof, nil
					})
				}
				if b, err := inventory.ReadBag(updated); err == nil {
					if updatePayload, err := protocol.InventoryUpdate(b.Rows()); err == nil {
						plan = append(plan, outboundPacket{"elvenmere_inventory_updated", 0, 14, updatePayload})
					}
				}
			}
		}
	}

	// 逢5层检查发放赛季首通奖励
	if clearedFloor%5 == 0 && clearedFloor >= 5 && clearedFloor <= 100 && w.activeDungeon.SeasonRewards[clearedFloor-1] == 0 {
		w.activeDungeon.SeasonRewards[clearedFloor-1] = 1
		sTemplate, sCount := dungeon.ElvenmereFloorSeasonReward(clearedFloor)
		if sTemplate != 0 && sCount > 0 && w.loot != nil {
			awarder := &inventory.Awarder{
				Catalog:   w.loot.Catalog,
				Rules:     w.loot.BagRules,
				Equipment: w.loot.Equipment,
			}
			if updated, _, err := awarder.Grant(w.role.State, sTemplate, sCount); err == nil {
				w.role.State = updated
				if store := w.service.Store; store != nil {
					key := fmt.Sprintf("elvenmere-season:%s:%d", w.activeDungeon.RunID, clearedFloor)
					_, _, _ = store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "elvenmere-reward-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
						proof, _ := json.Marshal(map[string]any{"template": sTemplate, "amount": sCount, "floor": clearedFloor})
						return updated, proof, nil
					})
				}
				if b, err := inventory.ReadBag(updated); err == nil {
					if updatePayload, err := protocol.InventoryUpdate(b.Rows()); err == nil {
						plan = append(plan, outboundPacket{"elvenmere_inventory_updated", 0, 14, updatePayload})
					}
				}
			}
		}
	}

	// 2. 发放通关经验奖励（Floor Clear EXP）
	expGain := dungeon.ElvenmereFloorClearExp(clearedFloor)
	if expGain > 0 && w.progression != nil {
		previousLevel := w.level
		if updatedRole, _, err := w.progression.ApplyGain(w.role, expGain); err == nil {
			updatedRole.WireID = w.role.WireID
			w.role = updatedRole
			if store := w.service.Store; store != nil {
				key := fmt.Sprintf("elvenmere-exp:%s:%d", w.activeDungeon.RunID, clearedFloor)
				_, _, _ = store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "elvenmere-exp-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
					proof, _ := json.Marshal(map[string]any{"exp": expGain, "floor": clearedFloor})
					return updatedRole.State, proof, nil
				})
			}
			if expPayload, err := character.ExperiencePayload(updatedRole); err == nil {
				w.level = expPayload[0]
				plan = append(plan, outboundPacket{"elvenmere_experience_updated", 0, 37, expPayload})
				if w.level != previousLevel {
					if skills, err := w.automaticSkillRefresh(); err == nil {
						plan = append(plan, skills...)
					}
				}
			}
		}
	}

	// 3. 下发 NOTI 2193 同步 Elvenmere 进度
	plan = append(plan, outboundPacket{"elvenmere_info_sent", 0, 2193, protocol.ElvenmereInfo(nextFloor, maxCleared, w.activeDungeon.WeeklyRewards, w.activeDungeon.SeasonRewards)})

	return plan, nil
}

func (w *worldSession) leaveDungeon() ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("leave dungeon without selected character")
	}
	// Leaving the starting route settles it, whichever exit is taken, and
	// lands the character at the town position the source route names.
	if w.inTutorial {
		if e := w.settleTutorialReturn(); e != nil {
			return nil, e
		}
	}
	// Returning to town republishes this actor to the shared scene before the area
	// list is serialized, so the client learns who is standing there.
	w.enterArea()
	// Town state remains the last owned, persisted origin throughout the run.
	ua, e := w.userAreaPayload()
	if e != nil {
		return nil, e
	}
	area, e := w.areaPayload()
	if e != nil {
		return nil, e
	}
	state, e := protocol.UserState(w.role.WireID, protocol.UserStateTown)
	if e != nil {
		return nil, e
	}
	plan := []outboundPacket{{"dungeon_leave_ack", 1, 42, []byte{1}}, {"town_actor_state", 0, 3, state}, {"dungeon_return_area", 0, 23, ua}, {"dungeon_return_users", 0, 24, area}}
	if w.characters != nil {
		visual, err := w.characters.EntryBasicProbe(w.role, [2]byte{})
		if err == nil {
			plan = append(plan, outboundPacket{"town_actor_appearance_restored", 0, 2, visual})
		}
		wornUpdate, err := inventory.WornSpaceUpdate(w.role.State)
		if err == nil && len(wornUpdate) > 0 {
			plan = append(plan, outboundPacket{"town_worn_visuals_restored", 0, 14, wornUpdate})
		}
		if inventory.HasEquippedCreature(w.role.State) {
			clPayload, err := inventory.CreatureListPayload(w.role.State)
			if err == nil {
				plan = append(plan, outboundPacket{"town_creature_list_restored", 0, 105, clPayload})
				plan = append(plan, outboundPacket{"town_creature_growth_restored", 0, 102, []byte{1, 0, 0, 0, 0, 0}})
			}
		}
	}
	if w.pilotDeath != nil && w.pilotDeath.Dead {
		w.pilotDeath.Dead = false
		if reviveState, err := protocol.PlayerDeathState(w.role.WireID); err == nil {
			reviveState[2] = 1 // state 1: 恢复满血满蓝并解除死亡幽灵（Ghost）状态，使角色在城镇中正常恢复行动
			plan = append(plan, outboundPacket{"town_actor_revived", 0, 32, reviveState})
		}
	}
	return plan, nil
}

func (w *worldSession) returnFromDungeonSelection(p []byte) ([]outboundPacket, error) {
	if len(p) != 0 || !w.selectingDungeon || w.activeDungeon != nil {
		return nil, fmt.Errorf("return requires owned dungeon selection")
	}
	plan, e := w.leaveDungeon()
	if e != nil {
		return nil, e
	}
	plan[0] = outboundPacket{"dungeon_selection_return", 0, 132, protocol.DungeonSelectionReturn()}
	return plan, nil
}
func (w *worldSession) monsterDeath(p []byte) ([]outboundPacket, error) {
	if w.activeDungeon == nil {
		return nil, fmt.Errorf("death without active run")
	}
	r, e := protocol.DecodeMonsterDeath(p)
	if e != nil {
		return nil, e
	}
	if _, e = w.activeDungeon.ConfirmDeath(r.Entity, r.Killer, w.role.WireID); e != nil {
		return nil, e
	}
	// A monster the boss took with it is confirmed dead so the room can
	// clear, but it was not killed by this character: it pays no drops and
	// no experience. Both are skipped rather than rolled and discarded, so
	// nothing is charged against the run's drop budget either.
	unowned := w.activeDungeon.Unowned[uint16(r.Entity)]
	plan := []outboundPacket{{"monster_death_ack", 1, 39, []byte{1}}}
	if !w.deathSent[uint16(r.Entity)] {
		body := protocol.MonsterDeathConfirmed(uint16(r.Entity))
		if w.loot != nil && !unowned {
			if w.drops == nil || w.drops.Run != w.activeDungeon.RunID {
				// Drops span every job's gear at every level by design; that
				// breadth is a feature, not a bug, so the pool is not narrowed
				// to what this character can wear.
				dropCatalog := w.loot.Catalog
				if len(w.loot.DropCatalog.Items) > 0 {
					dropCatalog = w.loot.DropCatalog
				}
				w.drops = loot.NewSession(dropCatalog, w.loot.Tables, w.loot.Rules, w.loot.Equipment, w.activeDungeon.RunID, w.account, w.role.ID, w.role.WireID)
				if w.activeDungeon.Definition.Odyssey {
					w.drops.Currency = w.loot.Currency
				}
				store := w.service.Store
				if store == nil && w.characters != nil {
					store = w.characters.Store
				}
				if store != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					if hasGrowth, _ := store.HasActivePremium(ctx, w.account, storage.PremiumGrowth, time.Now()); hasGrowth {
						w.drops.QuestDropBonusPercent = 20
					}
					cancel()
				}
			}
			rows, err := w.drops.Death(w.activeDungeon, uint16(r.Entity))
			if err != nil {
				return nil, err
			}
			body, err = protocol.MonsterDeathDrops(uint16(r.Entity), rows)
			if err != nil {
				return nil, err
			}
		}
		plan = append(plan, outboundPacket{"monster_death_confirmed", 0, 38, body})
	}
	if w.progression != nil && !unowned {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		previousLevel := w.level
		saved, _, err := w.progression.Monster(ctx, w.role, w.activeDungeon, uint16(r.Entity))
		cancel()
		if err != nil {
			return nil, err
		}
		// Preserve the current scene identity; storage's wire column is a
		// durable roster identity rather than a new scene allocation.
		saved.WireID = w.role.WireID
		p, err := character.ExperiencePayload(saved)
		if err != nil {
			return nil, err
		}
		w.role = saved
		w.level = p[0]
		plan = append(plan, outboundPacket{"monster_experience_updated", 0, 37, p})
		if w.level != previousLevel {
			skills, err := w.automaticSkillRefresh()
			if err != nil {
				return nil, err
			}
			plan = append(plan, skills...)
		}
		if w.quests != nil && w.level != previousLevel {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			available, err := w.availableQuestPayload(ctx)
			cancel()
			if err != nil {
				return nil, err
			}
			plan = append(plan, outboundPacket{"level_available_quests", 0, 21, available})
		}
	}
	completed, err := w.completeDungeon()
	if err != nil {
		return nil, err
	}
	return append(plan, completed...), nil
}

func (w *worldSession) bossCheck(p []byte) ([]outboundPacket, error) {
	r, err := protocol.DecodeBossCheck(p)
	if err != nil {
		return nil, err
	}
	if err = w.activeDungeon.BossCheck(r, w.role.WireID); err != nil {
		return nil, err
	}
	return w.completeDungeon()
}

func (w *worldSession) completeDungeon() ([]outboundPacket, error) {
	if !w.activeDungeon.Completed() || w.completionSent {
		return nil, nil
	}
	var plan []outboundPacket
	if w.progression != nil && w.progression.Odyssey != nil && w.activeDungeon.Definition.Odyssey {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		saved, _, e := w.progression.OdysseyClear(ctx, w.role, w.activeDungeon)
		cancel()
		if e != nil {
			return nil, e
		}
		saved.WireID = w.role.WireID
		body, e := character.ExperiencePayload(saved)
		if e != nil {
			return nil, e
		}
		w.role, w.level = saved, body[0]
		plan = append(plan, outboundPacket{"odyssey_clear_target_level", 0, 37, body})
		skills, e := w.automaticSkillRefresh()
		if e != nil {
			return nil, e
		}
		plan = append(plan, skills...)
		progress, e := w.progression.OdysseyProgressPayload(saved)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"odyssey_journal_updated", 0, 2856, progress})
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		gifted, applied, _ := w.progression.OdysseyGifts(ctx, w.role)
		cancel()
		w.role = gifted
		if applied {
			bag, e := inventory.ReadBag(gifted.State)
			if e != nil {
				return nil, e
			}
			update, e := protocol.InventoryRestore(bag.Rows())
			if e != nil {
				return nil, e
			}
			plan = append(plan, outboundPacket{"odyssey_milestone_inventory", 0, 13, update})
		}
		// 本关可能刚解锁了扩展装备槽（安徒恩→support / 卢克→魔法石 / 盖波加→耳环）。
		// 把新的槽位字节随 USERINFO1 再发一次：客户端的槽位状态在装备栏行对象
		// 构造期决定、运行期只读（取证见 next50），所以这里只保证它手上是最新值。
		if _, unlocks := character.OdysseyExpandEquipMask(w.activeDungeon.Definition.ID); unlocks && w.characters != nil {
			if addition, e := w.characters.EntryAddition(w.role); e == nil {
				plan = append(plan, outboundPacket{"dungeon_actor_addition_sent", 0, 2, addition})
			}
		}
	}
	if w.quests != nil && w.dungeons != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		active, err := w.quests.MapClear(ctx, w.role, w.activeDungeon, w.dungeons.Source.Checksum)
		if err != nil {
			return nil, err
		}
		triggers, err := protocol.QuestTriggers(active)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"map_clear_quest_triggers", 0, 291, triggers})
	}
	body, err := protocol.BossCheckConfirmed(w.activeDungeon.CompletionTarget())
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"boss_check_confirmed", 0, 115, body}, outboundPacket{"dungeon_clear_enabled", 0, 31, protocol.DungeonClearEnabled()})
	return plan, nil
}

func (w *worldSession) interactDoor(p []byte) (*dungeon.Session, []outboundPacket, error) {
	if w.activeDungeon == nil {
		return nil, nil, fmt.Errorf("door interaction without active dungeon")
	}
	if w.activeDungeon.Room.Map == 100016294 {
		// Sirocco cutscene room 100016294: synthesize a 160-byte transition to boss room (4,1)
		req := make([]byte, 160)
		req[0] = 4
		req[1] = 1
		binary.LittleEndian.PutUint32(req[151:155], w.activeDungeon.Definition.ID)
		next, movePlan, err := w.moveDungeonRoom(req)
		if err != nil {
			return nil, nil, err
		}
		plan := append([]outboundPacket{{"door_ack", 1, 38, []byte{1}}}, movePlan...)
		return next, plan, nil
	}
	return nil, []outboundPacket{{"door_ack", 1, 38, []byte{1}}}, nil
}

func (w *worldSession) moveDungeonRoom(p []byte) (*dungeon.Session, []outboundPacket, error) {
	if w.activeDungeon == nil || w.dungeons == nil {
		return nil, nil, fmt.Errorf("room transition without active run")
	}
	r, e := protocol.DecodeDungeonRoomTransition(p)
	if e != nil {
		return nil, nil, e
	}
	var next *dungeon.Session
	if r.LayerChange {
		next, e = w.activeDungeon.MoveScene(*w.dungeons, r)
	} else if r.Record[0] == 1 {
		next, e = w.activeDungeon.MoveScript(*w.dungeons, r)
		if e != nil {
			next, e = w.activeDungeon.Move(*w.dungeons, r.Position)
		}
	} else {
		next, e = w.activeDungeon.Move(*w.dungeons, r.Position)
	}
	if e != nil {
		return nil, nil, e
	}
	var seed uint32
	if e = binary.Read(rand.Reader, binary.LittleEndian, &seed); e != nil {
		return nil, nil, e
	}
	state := protocol.StartMapState{Position: r.Position, Seed: seed, Map: next.Room.Map, Monsters: next.LivingMonsters(), LayerChange: r.LayerChange}
	if _, visited := w.activeDungeon.Visited[next.Room.Map]; visited && next.Definition.Odyssey && !r.LayerChange {
		state.ReuseRoom = true
		state.Monsters = nil
	}
	if r.LayerChange || r.Record[0] == 1 {
		state.Transition = &r.Record
	}
	body, e := protocol.StartMap(state)
	if e != nil {
		return nil, nil, e
	}
	return next, []outboundPacket{{"dungeon_move_ack", 1, 45, []byte{1}}, {"dungeon_next_map_sent", 0, 29, body}}, nil
}
