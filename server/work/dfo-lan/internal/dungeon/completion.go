package dungeon

import (
	"dfolan/internal/game/protocol"
	"fmt"
)

// BossCheck may precede the target death. Retain its identity and wait for
// the actual room reports; do not manufacture story-dummy or actor deaths.
func (s *Session) BossCheck(r protocol.BossCheckRequest, actor uint16) error {
	if s == nil || !s.Loaded || r.Actor != actor || r.Target == 0 || r.Target == 65535 {
		return fmt.Errorf("boss check requires the owned loaded boss room")
	}
	position := [2]byte{s.Room.X, s.Room.Y}
	if s.Definition.Odyssey {
		for _, layer := range s.Maze.Layers {
			if layer.Position == position && len(layer.Maps) > 0 && s.Room.Map != layer.Maps[len(layer.Maps)-1] {
				return fmt.Errorf("boss scene sequence has not reached its final map")
			}
		}
	}
	found := false
	if s.Definition.Odyssey || s.Definition.ID == 100003126 {
		// 本仓库契约：奥德赛的 BossCheck 立即受理（见 TestOdysseyBossCheckImmediateCompletion，
		// 它用不存在的实体编号 9999 断言必须放行并直接完成）。
		//
		// ⚠️ 刻意分歧（不采纳修复包的收紧）：包里把这里换成「必须命中脚本声明的
		// [hunt boss] 模板 / 位于 maze.Boss」，用来防「奥德赛场景房里的 rank3 演出假 boss
		// 被客户端当 boss 上报 CMD117 → 副本在场景房里提前通关」。这条收紧会推翻上面的
		// 既有契约（单元测试与若干实机副本都依赖「立即受理」）。
		// 本仓库对同一风险已有**另一道**防护，就在本函数开头：
		//   boss scene sequence has not reached its final map
		// —— 只要当前格子挂着层图、而房间地图不是该层图序列的最后一张，就直接拒绝。
		// 因此这里维持原语义，不为同一问题再叠一层会破坏契约的判据。
		found = true

	} else {
		bossRoom := s.Room.Boss && position == s.Maze.Boss
		// Tutorial source data names the terminal coordinate in [boss] but
		// leaves that room's Boss marker false. Keep the coordinate and source
		// monster checks, waiving only the redundant room marker for tutorials.
		if s.Definition.Tutorial {
			bossRoom = position == s.Maze.Boss
		}
		for _, m := range s.Monsters {
			if m.Entity == r.Target && (m.Rank == 3 || m.APC && m.Rank >= 5 && m.Rank <= 8) {
				if s.Definition.Odyssey && s.Definition.HuntBoss != 0 {
					// Source hunt targets can finish an epilogue outside the map's
					// boss coordinate. A real owned target/death is still required.
					found = m.Template == s.Definition.HuntBoss
				} else {
					found = bossRoom || s.Room.Map == s.postBossQuestMap()
				}
			}
		}
	}
	// [MERGE-20260927-BOSS-ID-SKEW] 客户端上报的 boss 实体可能与服务端的 rank3
	// 标记错位：教程副本 7118 的 boss 房（91210）里，服务端 rank3 是 4126，客户端
	// 报的却是 4127（服务端视角的 rank0 小怪）。两边对不上就永远拿不到 boss 确认，
	// CMD117 被静默丢弃、客户端死等（实机 2026-09-27 黑屏卡死）。
	// 源已声明 boss 房位置、且房里确实存在可战斗的源领主时，接受客户端指定的任意
	// 本房间敌怪作为完成目标；目标仍必须是真实存在的源怪（team≠0 且非剧情 actor）。
	if !found && s.Room.Boss && position == s.Maze.Boss && s.hasFightableBoss() {
		for _, m := range s.Monsters {
			if m.Entity == r.Target && m.Team != 0 && !m.NonCombat {
				found = true
				break
			}
		}
	}
	if !found {
		return fmt.Errorf("boss check target is not a source boss in this room")
	}
	if s.completionTarget != 0 && s.completionTarget != r.Target {
		return fmt.Errorf("conflicting boss completion target")
	}
	s.completionTarget = r.Target
	s.tryComplete()
	return nil
}

// hasLayerEntry reports whether the current room belongs to a layered story
// sequence at all. A room in no layer entry has no scene sequence to wait for.
func (s *Session) hasLayerEntry() bool {
	position := [2]byte{s.Room.X, s.Room.Y}
	for _, layer := range s.Maze.Layers {
		if layer.Position == position && len(layer.Maps) > 0 {
			return true
		}
	}
	return false
}

// atLayerFinalMap reports whether the current room sits on the last map of
// every layered sequence entry it belongs to. A room outside all layer entries
// is reported false - callers keep hasLayerEntry alongside it for that case.
func (s *Session) atLayerFinalMap() bool {
	position := [2]byte{s.Room.X, s.Room.Y}
	final := false
	for _, layer := range s.Maze.Layers {
		if layer.Position != position || len(layer.Maps) == 0 {
			continue
		}
		if s.Room.Map != layer.Maps[len(layer.Maps)-1] {
			return false
		}
		final = true
	}
	return final
}

// atBossLayerMap reports whether the current room is a layered sequence entry
// sitting on the source maze's declared boss coordinate. Only that entry can be
// the run's closing scene: a layer entry elsewhere in the maze is a scene the
// client plays while the run continues - 100004944's (1,0) plays the
// 100016083_scene_0 interlude and then walks on to the boss room (3,2).
func (s *Session) atBossLayerMap() bool {
	if [2]byte{s.Room.X, s.Room.Y} != s.Maze.Boss {
		return false
	}
	return s.hasLayerEntry()
}

// huntTargetAbsent reports whether the dungeon declares a [hunt boss] completion
// target that is not standing in the current room. The declaration is the source's
// own clear condition ("kill it and the run is done"), so such a dungeon cannot be
// settled by a room clear while the target waits somewhere else - 100004944 marks
// (3,2) as its boss coordinate but leaves only a rank-0 monster there, and parks
// the declared 109019257 in the last cell's scene map 100016094. Dungeons with no
// declaration (HuntBoss == 0) are never affected.
func (s *Session) huntTargetAbsent() bool {
	if s.Definition.HuntBoss == 0 {
		return false
	}
	for _, m := range s.Monsters {
		if m.Template == s.Definition.HuntBoss {
			return false
		}
	}
	return true
}

// A source boss death clears its own room: the client removes the remaining
// ordinary monsters with the boss and never reports them individually, so a
// completion must not wait for those reports. Every source boss in the room
// still requires its own death report before the run is complete.
func (s *Session) tryComplete() {
	// Quest 23108's source maze continues past its boss room. The quest and
	// dungeon [clear condition] both name the final scene map, so settling on
	// the earlier boss map would strand the player before that objective.
	if target := s.postBossQuestMap(); target != 0 && s.Room.Map != target {
		return
	}
	if s.completionTarget == 0 {
		// Tutorial source maps do not consistently mark the terminal room as a
		// boss room, and some (for example swordman_m) contain no rank-3 actor at
		// all. The source maze's terminal coordinate plus a fully cleared room is
		// the only completion signal those routes provide.
		if s.Definition.Tutorial && s.Loaded && s.atTutorialTerminalMap() && s.RoomCleared() {
			s.completed = true
			return
		}
		// The source-matched final [CHANGE MAP] scene is the terminal event
		// for a layer with no reportable boss. It was accepted only after the
		// quest's objective room was actually cleared.
		if s.terminalSceneClosingReached && s.Loaded && s.atLayerFinalMap() && s.reportableDisplayBoss() == 0 {
			s.completed = true
			return
		}
		// Dungeon 26 maze 3's terminal layer is the opposite shape. Its last map
		// is entered with a live combat target, and the validated closing
		// [CHANGE MAP] cinematic returns to that cached final map once the
		// fighting ends. Only that transition may finish this story run, so the
		// map short-circuits every generic room-clear fallback below: they would
		// complete the run the moment the fighting stops, before the scene
		// plays. Live-verified, see analysis/tasks/lotus-terminal-layer-20260923.md.
		if s.Definition.ID == 26 && s.Maze.Index == 3 && s.Room.Map == 100008697 {
			if s.lotusClosingReached && s.RoomCleared() && s.reportableLotusTarget() != 0 {
				s.completed = true
			}
			return
		}
		// A plain source boss room can end without BOSS_CHECK when its only
		// boss is a non-combat display actor. The source map and boss position
		// must both match; layer scenes reuse the boss position while changing
		// maps. An actual fightable boss must still wait for its check.
		if s.Loaded && s.atSourceBossMap() && !s.hasFightableBoss() && s.roomEnemiesDead() && s.reportableDisplayBoss() != 0 {
			s.completed = true
			return
		}
		// [MERGE-20260928-BOSS-ROOM-ACTOR] 上一条要求房里存在 rank-3 的 display boss。
		// 但奥德赛里脚本声明的 boss 房也可能只摆 rank=0 的源怪（100004984..989 的 boss
		// 房就是 tpl=109019487 rank=0）：客户端不会为 rank0 发 CMD117，于是这里成了唯一
		// 能结算的入口。限定在 Odyssey，普通副本不因为「刚好有只敌怪」而多出结算路径
		// （TestSourceBossCompletionRequiresTheSourceBoss 守着这一点）。两道守卫与上一条
		// 同形（**在脚本声明的 boss 房间**、**房里可击杀目标已清空**）。
		//
		// [MERGE-20261001-ODYSSEY-HUNT-LAST-ROOM] 但它不看这只源怪是不是脚本声明的通关
		// 目标。奥德赛 100004944「向混乱的时空进发」的 boss 坐标 (3,2) 在源数据里标了
		// [boss]，那张地图 (100016091) 却是 [type] [normal]、房里只有一只 rank0 的
		// 109019087；脚本声明的 [hunt boss] 109019257 摆在最后一格 (4,2) 的 boss 演出图
		// 100016094 里（rank3/team100 可击杀）。玩家在 (3,2) 杀完那只 rank0 怪，这条兜底
		// 成立，副本当场结算：实机 2026-10-01 玩家收到「您已通关地下城」，而地图上还剩
		// 最后一格没打。
		//
		// 脚本的 [hunt boss] 就是它对通关条件的声明（"杀掉它就算通关"），所以声明了 hunt
		// 目标的副本不允许在「目标根本不在场」的房间里靠房间清空结算；目标所在的那一格
		// 照常结算（客户端为 rank3 目标发 CMD117，那条路先到）。没有声明 hunt 目标的
		// 奥德赛副本（100004984..989）行为不变。
		if s.Definition.Odyssey && s.Loaded && s.atSourceBossMap() && s.roomEnemiesDead() && s.reportableRoomActor() != 0 && !s.huntTargetAbsent() {
			s.completed = true
			return
		}
		// A layered story sequence whose only rank-3 actor is a display dummy
		// never yields a BOSS_CHECK - the client raises command 117 only for a
		// real boss - so completionTarget stays zero and no death report for
		// that dummy can ever close the run. The client ends the run on the map
		// itself instead: quest 3191 (dungeon 15 maze 6, palaceofload) walks
		// 100008695 -> 100008694 -> 100008684 -> 100008683 without a single
		// fight, so there is nothing to wait for but the arrival. Close on the
		// condition the layer does have - the sequence reached its final map and
		// every killable enemy is dead - and only there, so an ordinary room on
		// the way to the end cannot complete early. This is the layer-scene
		// counterpart of the source-boss room above: that one matches the boss
		// coordinate outside every layer entry, this one matches a layer's own
		// last map, and the two never both hold.
		//
		// [MERGE-20261001-ODYSSEY-SCENE-EARLY-CLEAR] 但**层图必须坐在源 maze 声明的
		// boss 坐标上**才算这一趟的收尾，中途的过场层图不算。奥德赛 100004944
		// 「向混乱的时空进发」的 (1,0) 也挂着一张单张层图 —— 演出过场
		// 100016083_scene_0，房里只有 1 只 rank3/team100 的 [displayhuntdummy]
		// （63821，NonCombat）。客户端进这张图、发 CMD37（加载完成）后，四项判据
		// 全成立，于是副本在**第二个房间**就结算了：实机 2026-10-01 玩家杀完 (1,0)
		// 的怪、过场一开就直接收到「您已通关地下城」。真正的地图终点是 maze.Boss
		// (3,2)，层图只是途中的剧情，要靠过场播完/点门继续往后走。
		if s.Loaded && s.atLayerFinalMap() && s.atBossLayerMap() && !s.hasFightableBoss() && s.roomEnemiesDead() && s.reportableDisplayBoss() != 0 {
			s.completed = true
			return
		}
		// 源领主是可战斗的 boss（rank 3 / Team 100），所以上面两条 display-boss
		// 路径（都要求 !hasFightableBoss()）对它不成立；而这些副本确认不发 CMD117，
		// completionTarget 恒为 0。脚本自己的 [clear condition] [hunt boss] 就是它对
		// 通关条件的声明（"杀掉它就算通关"），按它判定。
		//
		// 走到这里说明客户端没发 CMD117，所以这条对「客户端会发」的副本没有影响：
		// 那些副本在 boss 死时已经由 CMD117 置好 completionTarget，走不到这一行。
		// 两道守卫与上面两条同形（**在脚本声明的 boss 房间**、**房间里再没有活着的
		// 可击杀目标**），把这条限制在"该房间已清空且源领主已死"这一种状态：
		// runtime/sourcebossaudit 复核过，3200 个副本里 325 个声明了源领主，其中
		// 279 个的值能在自己地图里对上 rank3 [boss] 行（其余 46 个对不上 ⇒ 永远
		// 不满足，惰性）。「调律之边界」100005067/68 与「最终调律者」100005014
		// 都属前者；后者是那只 rank 3 的天平，此前完全没有结算路径。
		if s.Definition.SourceBoss != 0 && s.Loaded && s.atSourceBossMap() && s.roomEnemiesDead() {
			for _, m := range s.Monsters {
				if m.Template == s.Definition.SourceBoss && m.Rank == 3 && !m.NonCombat && m.Team != 0 && s.Dead[m.Entity] {
					s.completed = true
					return
				}
			}
		}
		return
	}
	if s.Definition.Odyssey || s.Definition.ID == 100003126 {
		s.completed = true
		return
	}
	if !s.Dead[s.completionTarget] {
		return
	}
	// Team-0 cinematic actors do not fight or report a death. A team-100
	// display dummy can report one and must still be confirmed before clear.
	for _, m := range s.Monsters {
		if m.Rank == 3 && m.Team != 0 && !s.Dead[m.Entity] {
			return
		}
	}
	// A layered sequence only completes on its final map; a room outside every
	// layer entry has no such sequence to wait for.
	if s.hasLayerEntry() && !s.atLayerFinalMap() {
		return
	}
	s.completed = true
}

func (s *Session) postBossQuestMap() uint32 {
	if s == nil || s.Definition.ID != 7123 || s.Maze.Quest != 23108 || s.Maze.Boss != [2]byte{4, 0} {
		return 0
	}
	for _, layer := range s.Maze.Layers {
		if layer.Position == [2]byte{6, 0} && len(layer.Maps) == 1 && layer.Maps[0] == 100008696 {
			return 100008696
		}
	}
	return 0
}

func (s *Session) TryComplete() { s.tryComplete() }

// SceneDiagnostic 记录最近一次场景换图走了哪条判定分支，仅供排查用（不影响行为）。
// nil 接收者安全。
func (s *Session) SceneDiagnostic() string {
	if s == nil {
		return ""
	}
	return s.sceneDiagnostic
}

func (s *Session) noteSceneDiagnostic(format string, a ...any) {
	s.sceneDiagnostic = fmt.Sprintf(format, a...)
}

// SceneEntryRecord 报告进入当前层图时客户端带来的换图记录。
//
// [MERGE-20260928-START-LAYER-EXIT] 起点层图格点门要**前进**，而前进那一包也得带上
// 换图记录：客户端靠 StartMap 的 Transition 安置角色。这份记录 scene_routes 里查不到
// （晦月湖 100004777 整条链都不在路由表里），只能从「客户端主动进层图」那一包里
// 原样留存 —— 留着默认全 f 会让角色落点错位，紧接的第二段剧情一开就崩。
func (s *Session) SceneEntryRecord() ([18]byte, bool) {
	if s == nil {
		return [18]byte{}, false
	}
	return s.layerRecord, s.hasLayerRecord
}

// [MERGE-20260928-SCENE-CLEAR-COMPLETE] MarkSceneCompleted 由「[clear map] 剧情副本」
// 的任务触发调用。
//
// 这类副本的结束信号是任务本身：脚本里没有可击杀的收尾 BOSS（实机 100004786
// 「墨色瘟疫之匣」的 BOSS 带 `[show boss hp percent gauge]`，但设计上永不死亡，
// 客户端血条显示 Immortal），客户端因此**不会**发 CMD117，玩家在剧情播完后由
// 客户端发 SET_QUEST_TRIGGER(33) 收尾。任务侧结算成功即代表本次攻略完成，
// 这里把副本一并标记完成，好让上层走正常的完成结算（NOTI34/37/35 等）。
func (s *Session) MarkSceneCompleted() {
	if s == nil {
		return
	}
	s.completed = true
}

func (s *Session) Completed() bool { return s != nil && s.completed }

// A source closing scene without a boss identity can enable dungeon clear
// directly; NOTI115 requires a real rank-3 entity from the current layer.
func (s *Session) CompletionNeedsBossCheck() bool {
	if s == nil || !s.Completed() || s.terminalSceneClosingReached {
		return false
	}
	// [CONFIRMED-20260929-QUEST3939-4851] 必须保留非零完成目标检查。
	// 旧写法 `!s.Definition.Tutorial || s.CompletionTarget() != 0` 会让普通
	// 剧情副本在没有可报告 Boss（CompletionTarget()==0）时仍发送 NOTI115。
	// 3939 实机已复现：CMD33 剧情触发后，BossCheckConfirmed(0) 报
	// "invalid boss identity"，整批任务刷新 NOTI291 和通关通知 NOTI31 被丢弃；
	// 玩家看不到结算，任务无法正常完成，即使数据库目标已经推进到 0。
	// 修复后 3939、4851 均已由用户实机确认完成：无 Boss 身份的已完成剧情
	// 直接走现有 NOTI31；真实 Boss 仍先发 NOTI115。不要恢复旧写法，也不要
	// 为绕过检查伪造 Boss 编号。修改此处必须保留这两种完成通知语义，回归
	// TestQuest3939SceneCompletionWithoutBossIdentity 和
	// TestSceneCompletionKeepsRealBossConfirmation。证据见
	// docs/protocol/luke-quest3939-scene-completion-20260929.md。
	return s.CompletionTarget() != 0
}

// CompletionTarget is the boss identity echoed back in the NOTI 115 payload. A
// story layer never raises a BOSS_CHECK, so no requested identity exists; the
// client is told about the room's display boss instead, which is the only
// rank-3 actor it knows there. The value must stay encodable - the wire
// encoder rejects 0 and 65535 - or the entire completion batch is dropped
// before the clear-enable ships, which looks exactly like nothing happening.
func (s *Session) CompletionTarget() uint16 {
	if !s.Completed() {
		return 0
	}
	if s.completionTarget == 0 {
		// 没有 CMD117 时 NOTI115 的身份要自己挑。按**源领主模板**取，
		// 而不是楼下「房间里第一个 rank3」—— 两者在出货脚本里恰好同值（房间只有一只
		// rank3），但只有按模板取才对得上 [clear condition] [hunt boss] 的语义。
		if s.Definition.SourceBoss != 0 {
			for _, m := range s.Monsters {
				if m.Template == s.Definition.SourceBoss && m.Rank == 3 && m.Team != 0 && m.Entity != 0 && m.Entity != 65535 {
					return m.Entity
				}
			}
		}
		if target := s.reportableDisplayBoss(); target != 0 && !s.lotusClosingReached {
			return target
		}
		// [MERGE-20260928-BOSS-ROOM-ACTOR] 没有 rank-3 领主的 boss 房（100004984..989）
		// 走不到上面两条，返回 0 会被编码器拒绝、整批完成数据被丢弃。挑一个可报告的
		// 源怪当身份，与 tryComplete 新增的那条 Odyssey 结算路径配套；限定 Odyssey，
		// 普通副本的身份选择不变。
		if s.Definition.Odyssey {
			if target := s.reportableRoomActor(); target != 0 && !s.lotusClosingReached {
				return target
			}
		}
		return s.reportableLotusTarget()
	}
	return s.completionTarget
}

func (s *Session) atSourceBossMap() bool {
	position := [2]byte{s.Room.X, s.Room.Y}
	if !s.Room.Boss || position != s.Maze.Boss {
		return false
	}
	for _, layer := range s.Maze.Layers {
		if layer.Position == position && len(layer.Maps) > 0 {
			return false
		}
	}
	for _, room := range s.Maze.Rooms {
		if room.Boss && [2]byte{room.X, room.Y} == position && room.Map == s.Room.Map {
			return true
		}
	}
	return false
}

func (s *Session) atTutorialTerminalMap() bool {
	position := [2]byte{s.Room.X, s.Room.Y}
	if position != s.Maze.Boss {
		return false
	}
	for _, room := range s.Maze.Rooms {
		if [2]byte{room.X, room.Y} == position && room.Map == s.Room.Map {
			return true
		}
	}
	return false
}

func (s *Session) hasFightableBoss() bool {
	for _, m := range s.Monsters {
		if !m.NonCombat && m.Team != 0 && (m.Rank == 3 || m.APC && m.Rank >= 5 && m.Rank <= 8) {
			return true
		}
	}
	return false
}

func (s *Session) roomEnemiesDead() bool {
	for _, m := range s.Monsters {
		if m.Team != 0 && !m.NonCombat && !s.Dead[m.Entity] {
			return false
		}
	}
	return true
}

func (s *Session) reportableDisplayBoss() uint16 {
	for _, m := range s.Monsters {
		if m.Rank == 3 && m.Team != 0 && m.Entity != 0 && m.Entity != 65535 {
			return m.Entity
		}
	}
	return 0
}

// A story display boss is present in the final map's NOTI29 rows. Use its
// actual entity as the confirmation identity when no CMD117 was sent.
// [MERGE-20260928-BOSS-ROOM-ACTOR] reportableRoomActor 在**没有 rank-3 领主**的
// boss 房里挑一个可报告的源怪，作为 NOTI115 的身份。
//
//	奥德赛的 100004984..100004989 这 5 个副本（正是 scenes 导出里没有 scene_routes
//	的那一批）的 boss 房只摆 rank=0 的源怪（tpl=109019487）。客户端不会为 rank0 发
//	CMD117，而 reportableDisplayBoss 要求 rank==3 故返回 0 —— 于是 tryComplete 的
//	兜底不成立、CompletionTarget 也返回 0，可编码器拒绝 0，整批完成数据被丢弃：
//	客户端看不到任何变化，玩家在 boss 房里点门只收到 door_ack。
//	实机 2026-09-28「前往阿拉德」（副本 100004986，boss 房 100016525）就是这样卡住的。
func (s *Session) reportableRoomActor() uint16 {
	for _, m := range s.Monsters {
		if m.Entity != 0 && m.Entity != 65535 && m.Team != 0 && !m.NonCombat {
			return m.Entity
		}
	}
	return 0
}

func (s *Session) reportableLotusTarget() uint16 {
	if s == nil || !s.lotusClosingReached || s.Definition.ID != 26 || s.Maze.Index != 3 || s.Room.Map != 100008697 {
		return 0
	}
	for _, m := range s.Monsters {
		if m.Rank == 3 && m.Team == 100 && m.Entity != 0 && m.Entity != 65535 {
			return m.Entity
		}
	}
	return 0
}
