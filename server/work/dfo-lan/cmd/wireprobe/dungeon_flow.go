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
	"errors"
	"fmt"
	"log"
	"os"
	"time"
)

// monsterCreateTriggerEnabled 让进图包带上 [monster create trigger] 的真值
// （怪物记录里 Rank 之后那一格，原本恒为 0）。该字段语义尚未确认，
// 所以由环境变量控制：DFO_MONSTER_CREATE_TRIGGER=1 时编码真值，
// 否则编 0，输出与原先完全一致。
func monsterCreateTriggerEnabled() bool {
	return os.Getenv("DFO_MONSTER_CREATE_TRIGGER") == "1"
}

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
		if scene, ok := w.townArrivalScenes[requested]; ok {
			if w.state.Position.Town != scene.Town || w.state.Position.Area != scene.Area {
				return nil, fmt.Errorf("town arrival scene %d requires town area %d/%d", requested, scene.Town, scene.Area)
			}
			if err := w.service.ValidateRestoredPosition(w.level, w.odyssey, w.state.Position); err != nil {
				return nil, err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			accepted, err := w.acceptedQuestIDs(ctx)
			cancel()
			if err != nil {
				return nil, err
			}
			if scene.QuestID > 65535 || !accepted[uint16(scene.QuestID)] {
				return nil, fmt.Errorf("town arrival scene %d requires accepted source quest %d", requested, scene.QuestID)
			}
			return []outboundPacket{
				{"town_arrival_scene_gate_ack", 1, 15, []byte{1}},
				{"town_arrival_scene_selection_sent", 0, 27, protocol.EnterDungeonSelection()},
			}, nil
		}
		if w.dungeons != nil && dungeon.IsTrainingRoom(*w.dungeons, requested) {
			return []outboundPacket{
				{"training_room_gate_ack", 1, 15, []byte{1}},
				{"dungeon_selection_sent", 0, 27, protocol.EnterDungeonSelection()},
			}, nil
		}
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

func (w *worldSession) isTownArrivalOriginSync(id uint16, p []byte) bool {
	if w == nil || w.pendingTownArrival == nil || id != 36 {
		return false
	}
	r, err := protocol.DecodeAreaChangeRequest(p)
	if err != nil {
		return false
	}
	pos := w.state.Position
	return r.Town == pos.Town && r.Area == pos.Area && r.X == pos.X && r.Y == pos.Y &&
		r.PreviousTown == pos.Town && uint32(r.PreviousArea) == pos.Area && r.Flag == 0 && r.TailFlags == [2]byte{}
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
	if scene, ok := w.townArrivalScenes[r.ID]; ok {
		// A quest's arrival trigger is a town event. A rejected CMD15 must not
		// become a private dungeon session through the following CMD16.
		if r.Quest != scene.QuestID || w.state.Position.Town != scene.Town || w.state.Position.Area != scene.Area || !w.selectingDungeon || w.approvedDungeonGate != r.ID {
			return nil, nil, fmt.Errorf("town arrival scene %d has no approved matching gate for quest %d at %d/%d", r.ID, r.Quest, w.state.Position.Town, w.state.Position.Area)
		}
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
	trainingRoom := dungeon.IsTrainingRoom(*w.dungeons, r.ID)
	if _, townArrival := w.townArrivalScenes[r.ID]; !trainingRoom && !townArrival {
		if _, e := w.dungeonGate(make([]byte, 8)); e != nil {
			return nil, nil, e
		}
	}
	if w.soloPartyReady && r.Party == 1 {
		// This connection owns the single-member bootstrap party. The dungeon
		// domain remains solo; never normalize arbitrary party IDs.
		r.Party = 65535
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var s *dungeon.Session
	if trainingRoom {
		s, e = dungeon.SelectTrainingRoom(*w.dungeons, r, w.level)
	} else {
		var accepted map[uint16]bool
		accepted, e = w.acceptedQuestIDs(ctx)
		if e == nil {
			s, e = dungeon.Select(*w.dungeons, r, w.level, accepted)
		}
	}
	if e != nil {
		return nil, nil, e
	}
	noteMazeEntry(s)
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
	if s.Tournament != nil {
		seed = s.Tournament.Seed
	}
	start, e := protocol.StartMap(protocol.StartMapState{Position: s.Maze.Start, Seed: seed, Map: s.Room.Map, Monsters: s.Monsters, EncodeCreateTrigger: monsterCreateTriggerEnabled()})
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
				growth, err := inventory.CreatureGrowthPayload(w.role.State)
				if err == nil {
					plan = append(plan, outboundPacket{"dungeon_creature_growth_sent", 0, 102, growth})
				}
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
	if s.Tournament != nil {
		info, err := protocol.TournamentInfo(s.Tournament.Opening)
		if err != nil {
			return nil, err
		}
		mapInfo, err := protocol.TournamentMapInfo(s.Maze.Start, seed, s.Room.Map)
		if err != nil {
			return nil, err
		}
		plan = append(plan,
			outboundPacket{"tournament_info_sent", 0, 372, info},
			outboundPacket{"tournament_map_info_sent", 0, 373, mapInfo},
		)
	}
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
	// The direct-move body carries the dungeon id and difficulty only, so the
	// remaining selection fields take the values CMD 16 uses for a solo run.
	sel := protocol.DungeonSelection{ID: r.ID, Difficulty: r.Difficulty, Party: 65535}
	var s *dungeon.Session
	if dungeon.IsTrainingRoom(*w.dungeons, r.ID) {
		s, e = dungeon.SelectTrainingRoom(*w.dungeons, sel, w.level)
	} else {
		var accepted map[uint16]bool
		accepted, e = w.acceptedQuestIDs(ctx)
		if e == nil {
			s, e = dungeon.Select(*w.dungeons, sel, w.level, accepted)
		}
	}
	if e != nil {
		return nil, nil, e
	}
	noteMazeEntry(s)
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
	return s, append(dungeonSelectionHead(), plan...), nil
}

// dungeonSelectionHead is the client's dungeon-select UI layer: the gate
// acknowledgement and NOTI 27. Whenever an entry sequence is sent to a client
// that is not already standing in the town selection flow - the post-clear
// "next story dungeon" gate and the settlement panel's "again" button - it has
// to be preceded by this pair, or the entry frames land on a scene the client
// has already torn down and it exits (0xC0000005).
func dungeonSelectionHead() []outboundPacket {
	return []outboundPacket{
		{"dungeon_gate_ack", 1, 15, []byte{1}},
		{"dungeon_selection_sent", 0, 27, protocol.EnterDungeonSelection()},
	}
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
	// 征兆存档必须先读出来：它同时决定 noti 2836 的队伍状态（omen_info.go）与
	// noti 2838 的档位（oath_info.go —— 隐藏 BOSS 现在由「满档结算」驱动）。
	if e := w.loadOmenRunState(w.oathProgressDungeon()); e != nil {
		return nil, e
	}
	plan := []outboundPacket{{"dungeon_loading_ack", 1, 37, []byte{1}}, {"dungeon_actor_state", 0, 3, state}, {"dungeon_loading_complete", 0, 30, protocol.DungeonLoaded()}}
	// 常驻状态：把两个档位在客户端读 getter 之前下发（见 oath_info.go）。
	grades, e := w.oathInfoPackets()
	if e != nil {
		return nil, e
	}
	plan = append(plan, grades...)
	// 征兆队伍状态（noti 2836）。客户端进 EOO 副本时自己已经把两个征兆窗开好，
	// 这里只负责把每个座位的状态填进去。见 omen_info.go。
	omen, e := w.omenInfoPackets()
	if e != nil {
		return nil, e
	}
	plan = append(plan, omen...)
	// 诊断注入器：把候选通知塞在副本加载应答里，天平开场读 getter 之前就到位。
	plan = append(plan, w.oathInjectNext()...)
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
		reset, full, enabled, err := w.characters.CloneReattachPackets(w.role)
		if err != nil {
			return nil, err
		}
		if enabled {
			restore, restoreErr := inventory.NonAvatarWornSpaceUpdate(w.role.State)
			if restoreErr != nil {
				return nil, restoreErr
			}
			plan = append(plan,
				outboundPacket{"dungeon_clone_detached", 0, 2, reset},
				outboundPacket{"dungeon_clone_reattached", 0, 2, full},
			)
			if len(restore) > 0 {
				plan = append(plan, outboundPacket{"dungeon_nonavatar_worn_restored", 0, 14, restore})
			}
		}
		if inventory.HasEquippedCreature(w.role.State) {
			clPayload, err := inventory.CreatureListPayload(w.role.State)
			if err == nil {
				plan = append(plan, outboundPacket{"dungeon_creature_list_restored", 0, 105, clPayload})
				growth, err := inventory.CreatureGrowthPayload(w.role.State)
				if err == nil {
					plan = append(plan, outboundPacket{"dungeon_creature_growth_restored", 0, 102, growth})
				}
			}
		}
		// The damage font the player applied in town is state the rebuilt actor
		// never asks the warehouse for, so the owned page and the selection go
		// back here the way the worn visuals do.
		plan = append(plan, w.damageFontRestore()...)
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
			before, _ := inventory.ReadBag(w.role.State)
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
					if updatePayload, err := protocol.InventoryUpdate(inventory.ChangedItemRows(before, b)); err == nil {
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
			before, _ := inventory.ReadBag(w.role.State)
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
					if updatePayload, err := protocol.InventoryUpdate(inventory.ChangedItemRows(before, b)); err == nil {
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
				growth, err := inventory.CreatureGrowthPayload(w.role.State)
				if err == nil {
					plan = append(plan, outboundPacket{"town_creature_growth_restored", 0, 102, growth})
				}
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

// fatalDropFailure classifies a failed drop roll for an already-confirmed death.
//
// A coverage gap - the imported model has no row for this monster's level - is not
// fatal: the death is a fact the client reported, while the drop is the server's own
// choice, and withholding the report over a gap leaves the client's monsters alive and
// the gate unopened (2026-09-26 border-of-attunement, and the FFFF killer arm in
// dungeon.ConfirmDeath before it). Every other failure is a defect of the model
// itself and must still fail the request, so that a real bug cannot hide behind
// "this monster simply paid nothing".
func fatalDropFailure(err error) error {
	if err == nil || errors.Is(err, loot.ErrOutOfDropRange) {
		return nil
	}
	return err
}

// noteDropGap records that a confirmed death paid nothing because the imported drop
// model does not cover the monster. The death itself is already applied; this only
// keeps the coverage gap visible instead of letting loot vanish quietly.
func (w *worldSession) noteDropGap(event func(map[string]any), entity uint16, e error) {
	if event == nil {
		return
	}
	fields := map[string]any{"kind": "drop_roll_skipped", "entity": entity, "reason": e.Error()}
	if w.activeDungeon != nil {
		for _, m := range w.activeDungeon.Monsters {
			if m.Entity == entity {
				fields["level"] = m.Level
				fields["rank"] = m.Rank
				fields["template"] = m.Template
				break
			}
		}
	}
	event(fields)
}

// noteOmenClear 把最近一次征兆结算记进事件流。玩家报告的「这把给了什么」应当
// 能从日志直接读出来，而不是靠反推掉落物属于哪一档。账本按自增序号去重，所以
// 同一场里其余怪物的死亡不会重复报同一条。
func (w *worldSession) noteOmenClear(event func(map[string]any)) error {
	if w.drops == nil || w.drops.Omen == nil {
		return nil
	}
	outcome, ok := w.drops.Omen.Last(w.role.ID)
	if !ok || outcome.Seq == w.omenReported {
		return nil
	}
	w.omenReported = outcome.Seq
	// 落库：持有数写回角色存档，满档结算额外置「下一场该出隐藏 BOSS」（见 omen_state.go）。
	// 失败往上抛 —— 掉落已经按结算结果算出来了，静默丢掉这次推进会让存档和玩家看到的
	// 东西互相矛盾。
	if err := w.noteOmenSettlement(outcome); err != nil {
		return err
	}
	if event == nil {
		return nil
	}
	fields := map[string]any{
		"kind":    "omen_clear",
		"dungeon": outcome.Dungeon,
		"held":    outcome.Held,
		"after":   outcome.After,
		"gained":  outcome.Gained,
		"paid":    outcome.Paid,
		"stage":   outcome.Stage,
	}
	if len(outcome.Awards) > 0 {
		ids := make([]uint32, 0, len(outcome.Awards))
		for _, a := range outcome.Awards {
			ids = append(ids, a.Template)
		}
		fields["templates"] = ids
	}
	event(fields)
	return nil
}

func (w *worldSession) monsterDeath(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w.activeDungeon == nil {
		return nil, fmt.Errorf("death without active run")
	}
	r, e := protocol.DecodeMonsterDeath(p)
	if e != nil {
		return nil, e
	}
	confirmed, e := w.activeDungeon.ConfirmDeath(r.Entity, r.Killer, w.role.WireID)
	if e != nil {
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
					w.drops.ChapterDrop = w.loot.ChapterDrop
				}
				w.drops.Attunement = w.loot.Attunement
				w.drops.RewardBoxes = w.loot.RewardBoxes
				w.drops.Omen = w.loot.Omen
				if w.loot.Omen != nil && w.omenHeldReady {
					// 本场开始时的持有数：-omen-state 时来自角色存档
					// （loadOmenRunState），否则来自 -omen-hold 诊断。账本本身是内存的，
					// 所以新的一场必须重新预载，否则会沿用上一场结算后的值。
					w.loot.Omen.Set(w.role.ID, w.omenHeldRun)
				} else if w.loot.Omen != nil && !w.omenHoldApplied && w.omenHold >= 0 {
					// 诊断入口，每个会话只应用一次：放到指定阶段后就交回正常的
					// 累积/结算路径，免得每进一次副本都被拽回同一格。
					w.loot.Omen.Set(w.role.ID, uint32(w.omenHold))
					w.omenHoldApplied = true
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
			if fatal := fatalDropFailure(err); fatal != nil {
				return nil, fatal
			}
			if err != nil {
				// Coverage gap: the imported model does not reach this monster, so the
				// death stands and pays nothing. See fatalDropFailure.
				w.noteDropGap(event, uint16(r.Entity), err)
				rows = nil
			}
			body, err = protocol.MonsterDeathDrops(uint16(r.Entity), rows)
			if err != nil {
				return nil, err
			}
			if err := w.noteOmenClear(event); err != nil {
				return nil, err
			}
		}
		plan = append(plan, outboundPacket{"monster_death_confirmed", 0, 38, body})
	}
	if confirmed && w.quests != nil && !unowned {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		grant, err := w.quests.GrantSeekingMonsterItems(ctx, w.role, w.activeDungeon, uint16(r.Entity))
		if err != nil {
			return nil, err
		}
		if len(grant.Items) > 0 {
			grant.Role.WireID = w.role.WireID
			w.role = grant.Role
			bag, err := inventory.ReadBag(w.role.State)
			if err != nil {
				return nil, err
			}
			seen := map[uint16]bool{}
			var rows [][protocol.CurrentItemRecordSize]byte
			for _, item := range grant.Items {
				for _, slot := range item.Slots {
					if seen[slot] {
						continue
					}
					row, ok := bag.RowAt(slot)
					if !ok {
						return nil, fmt.Errorf("quest item destination slot %d missing", slot)
					}
					seen[slot] = true
					rows = append(rows, row)
				}
			}
			update, err := protocol.InventoryUpdate(rows)
			if err != nil {
				return nil, err
			}
			plan = append(plan, outboundPacket{"seeking_items_granted", 0, 14, update})
		}
		if grant.Advanced {
			active, err := w.quests.Active(ctx, w.role)
			if err != nil {
				return nil, err
			}
			triggers, err := protocol.QuestTriggers(active)
			if err != nil {
				return nil, err
			}
			plan = append(plan, outboundPacket{"seeking_quest_triggers", 0, 291, triggers})
		}
	}
	if confirmed && w.quests != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		advanced, err := w.quests.EnemyDeath(ctx, w.role, w.activeDungeon, uint16(r.Entity))
		if err == nil && advanced {
			var active []protocol.ActiveQuest
			active, err = w.quests.Active(ctx, w.role)
			if err == nil {
				var triggers []byte
				triggers, err = protocol.QuestTriggers(active)
				if err == nil {
					plan = append(plan, outboundPacket{"enemy_hunt_quest_triggers", 0, 291, triggers})
				}
			}
		}
		cancel()
		if err != nil {
			return nil, err
		}
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
		// The death acknowledgement and confirmation are already in plan.
		// Completion failure must not discard the client's death evidence.
		w.completionErr = err
		return plan, nil
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
		if w.progression != nil && w.progression.Chapters != nil {
			chaptered, chapterApplied, _ := w.progression.OdysseyChapterRewards(ctx, gifted)
			gifted, applied = chaptered, applied || chapterApplied
		}
		cancel()
		w.role = gifted
		if applied {
			bag, e := inventory.ReadBag(gifted.State)
			if e != nil {
				return nil, e
			}
			update, e := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
			if e != nil {
				return nil, e
			}
			plan = append(plan, outboundPacket{"odyssey_milestone_inventory", 0, 13, update})
		}
		// 本关解锁了扩展装备槽时，就地重喂一次装备栏，让客户端**当场**重建装备栏 ——
		// 否则玩家必须重登或重选角色才看得到解锁（槽位状态由装备栏行对象在构造期
		// 决定、运行期只读，取证见 analysis/tasks/next50-odyssey-expanded-equip-slot.md）。
		//
		// 帧组合与顺序完全复用 entry / 装备变更（equipment_flow.go）那两套通道：
		// id-13 重喂背包网格与 worn 模型，id-14 重喂 worn 槽窗口；**两者必须成对**，
		// 实测只发 id-13 会让槽位窗口变空。注意不要发 entry_addition（NOTI 2）——
		// 那是进图/登录帧，在副本内发会让客户端把装备栏显示清空（见该文档「重发的坑」）。
		if _, unlocks := character.OdysseyExpandEquipMask(w.activeDungeon.Definition.ID); unlocks {
			if bag, e := inventory.ReadBag(w.role.State); e == nil {
				if bagBody, e := protocol.InventoryRestore(bag.Rows()); e == nil {
					plan = append(plan, outboundPacket{"equipment_bag_resynced", 0, 13, bagBody})
				}
				if wornBody, e := inventory.WornPayload(w.role.State); e == nil && len(wornBody) > 0 {
					plan = append(plan, outboundPacket{"equipment_worn_resynced", 0, 13, wornBody})
				}
				if slots, e := inventory.EquipmentPayload(3, bag.Worn, false); e == nil {
					plan = append(plan, outboundPacket{"equipment_slots_updated", 0, 14, slots})
				}
			}
			if upd, e := inventory.WornSpaceUpdate(w.role.State); e == nil && len(upd) > 0 {
				plan = append(plan, outboundPacket{"equipment_worn_window_refreshed", 0, 14, upd})
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
	if w.activeDungeon.CompletionNeedsBossCheck() {
		body, err := protocol.BossCheckConfirmed(w.activeDungeon.CompletionTarget())
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"boss_check_confirmed", 0, 115, body})
	}
	plan = append(plan, outboundPacket{"dungeon_clear_enabled", 0, 31, protocol.DungeonClearEnabled()})
	if w.activeDungeon.Tournament != nil {
		reward, e := w.tournamentClear()
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"tournament_clear_reward", 0, 374, reward})
	}
	// 隐藏 BOSS 的两种来源各自归位，都必须在「通关确认」之后 —— 掉线或退出不该
	// 吞掉已经攒到的那一次。
	cleared := w.oathProgressDungeon()
	if w.omenState {
		// 征兆线：这一场把进本时读到的「满档结算待出」标记兑现掉。见 omen_state.go。
		if e := w.clearOmenOrthaier(cleared); e != nil {
			return nil, e
		}
	} else if before, after, e := w.noteOathProgressClear(cleared); e != nil {
		// 旧通关保底（-oath-progress-clears，诊断保留）。
		return nil, e
	} else if w.oathProgressEnabled(cleared) {
		log.Printf("oath progress: dungeon %d clears %d -> %d (pity every %d)",
			cleared, before, after, w.oathProgressClears)
	}
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
	state := protocol.StartMapState{Position: r.Position, Seed: seed, Map: next.Room.Map, Monsters: next.LivingMonsters(), LayerChange: r.LayerChange, EncodeCreateTrigger: monsterCreateTriggerEnabled()}
	if r.LayerChange && next.Room.Map == w.activeDungeon.Room.Map {
		state.ReuseRoom = true
		state.Monsters = nil
	}
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
