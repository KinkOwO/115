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
	if e = w.service.ValidatePosition(w.level, w.state.Position); e != nil {
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
		creation, err := protocol.DecodeCreateRequest(w.role.Request)
		if err != nil || len(creation.Options) != 12 || creation.Options[10] != 2 {
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
	quests, e := w.service.Store.Quests(ctx, w.account, w.role.ID)
	if e != nil {
		return nil, nil, e
	}
	accepted := map[uint16]bool{}
	for _, q := range quests {
		// 客户端自己的门槛文案就是 "accepted **or** completed prerequisite quests"，
		// 只认 accepted 会让已完成的任务副本反而进不去。
		if (q.Status == "accepted" || q.Status == "completed") && q.ConfigVersion == w.dungeons.Source.Checksum {
			accepted[q.ID] = true
		}
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
	var seed uint32
	if e = binary.Read(rand.Reader, binary.LittleEndian, &seed); e != nil {
		return nil, nil, e
	}
	start, e := protocol.StartMap(protocol.StartMapState{Position: s.Maze.Start, Seed: seed, Map: s.Room.Map, Monsters: s.Monsters})
	if e != nil {
		return nil, nil, e
	}
	plan := []outboundPacket{{"dungeon_select_ack", 1, 16, []byte{1}}}
	if w.soloPartyBootstrap {
		party, e := protocol.SoloPartyInfo(w.role.WireID)
		if e != nil {
			return nil, nil, e
		}
		plan = append(plan, outboundPacket{"solo_party_initialized", 0, 9, party})
	}
	plan = append(plan, []outboundPacket{
		{"dungeon_info_sent", 0, 28, protocol.DungeonInfo(protocol.DungeonInfoState{ID: r.ID, Difficulty: r.Difficulty, Maze: s.Maze.Index, Boss: s.Maze.Boss})},
		{"dungeon_start_map_sent", 0, 29, start},
	}...)
	return s, plan, nil
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
	return []outboundPacket{{"dungeon_leave_ack", 1, 42, []byte{1}}, {"town_actor_state", 0, 3, state}, {"dungeon_return_area", 0, 23, ua}, {"dungeon_return_users", 0, 24, area}}, nil
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
				w.drops = loot.NewSession(w.loot.Catalog, w.loot.Tables, w.loot.Rules, w.loot.Equipment, w.activeDungeon.RunID, w.account, w.role.ID, w.role.WireID)
				if w.activeDungeon.Definition.Odyssey {
					w.drops.Currency = w.loot.Currency
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
