package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"fmt"
)

// The closing q3215_14949.cmt [CHANGE MAP] changes the player's position
// within the final cached layer. Its native CMD45 record carries the scene's
// X/Y bounds; NOTI29 mode 0 with layer flag 1 retains the last layer in the
// client (1452b77ee..1452b788b). This is not a request for another map.
func (s *Session) lotusClosingRevisit(r protocol.DungeonRoomTransition) bool {
	if s.Definition.ID != 26 || s.Maze.Index != 3 || s.Room.Map != 100008697 || !s.RoomCleared() {
		return false
	}
	record := r.Record
	if record[0] != 0 || record[1] != 0 || record[2] != 0 || record[3] != 0 || record[4] != 4 || record[5] != 5 ||
		record[10] != 0 || record[11] != 0 || record[12] != 3 || record[13] != 0 || record[14] != 2 || record[15] != 0 || record[16] != 0 || record[17] != 0 {
		return false
	}
	x := binary.LittleEndian.Uint16(record[6:8])
	y := binary.LittleEndian.Uint16(record[8:10])
	return x >= 701 && x <= 704 && y >= 229 && y <= 231
}

func (s *Session) MoveScene(c catalog.DungeonCatalog, r protocol.DungeonRoomTransition) (*Session, error) {
	if s == nil || !s.Loaded || !r.LayerChange || s.Completed() {
		return nil, fmt.Errorf("scene transition requires owned cleared loaded room")
	}
	// [MERGE-20260928-DIAG] 每次换图重新采集决策路径。
	s.sceneDiagnostic = ""
	// [MERGE-20260928-START-LAYER-EXIT] 留存客户端带来的换图记录：起点层图格点门要
	// 前进，那一包得原样带上它，否则角色落点错位（见 SceneEntryRecord）。
	if r.Record != ([18]byte{}) {
		s.layerRecord, s.hasLayerRecord = r.Record, true
	}
	if r.Dungeon != s.Definition.ID || r.Position != [2]byte{s.Room.X, s.Room.Y} {
		return nil, fmt.Errorf("scene transition dungeon/position mismatch")
	}
	if s.Definition.Odyssey && len(c.SceneRoutes) > 0 {
		if !s.RoomCleared() {
			return nil, fmt.Errorf("scene transition requires owned cleared loaded room")
		}
		if !s.scriptStageReady() {
			return nil, fmt.Errorf("scene requires preceding source script warp")
		}
		for _, route := range c.SceneRoutes {
			if route.Dungeon != s.Definition.ID || route.Maze != s.Maze.Index || route.From != s.Room.Map || route.Position != r.Position {
				continue
			}
			if route.Source != c.Source.Checksum || route.DungeonSHA256 != s.Definition.Script.SHA256 || route.MapSHA256 != c.Maps[s.Room.Map].SHA256 || !catalog.SceneRouteInMaze(s.Definition, route) {
				return nil, fmt.Errorf("scene transition source mismatch")
			}
			if r.Record != route.Record {
				continue
			}
			// [MERGE-20260928-SCENE-REVISIT] 原来这里对「目标层图已访问」
			// （含已访问但怪列表为空）一律报 "scene layer missing or already
			// visited"。但层图是演出场景、本来就没有可刷的怪，s.Visited[to] 记的
			// 怪列表对它们恒为空，于是第二次进入必被判重放。
			// 实机 2026-09-28 安图恩讨伐战 100004950：客户端在 Start (0,5) 的层图
			// 164 ↔ 165 之间往返（anton_00.cmt 的 [CHANGE MAP]），第二次进 165 就被
			// 拒，客户端停在门的蓝圈上过不去。层图往返是客户端的正常行为，不拒绝。
			room := s.Room
			room.Map = route.To
			return s.enterRoom(c, room)
		}
		// [MERGE-20260927-SCENE-EXIT] scene_routes 只声明「进入」场景房的条目，没有
		// 「出来」的那一条；客户端在场景房里点门也只发 id=38（interactDoor），不走
		// layer 切换。没有这段时，玩家进了场景房（实机 100016083_scene_0）就再也出
		// 不去。把「没有下一张可用路由」视为从场景房退出，回到该位置的 base 房间。
		//
		// 只认「不带落点记录」的形态（客户端在场景房点门正是这种）**或**服务端自己
		// 补上的那份源记录：interactDoor 合成请求时会先把 LayerRouteRecord 填进
		// Record，因为客户端要靠 StartMap 的 Transition 记录安置角色（record[6:10]
		// 是落点）—— 缺了它角色会卡在场景左上角（实机 2026-09-28 贵族机要）。
		// 带**其它** Record 的 layer 切换仍走上面的严格匹配，篡改落点照旧被拒。
		serviceRecord, hasServiceRecord := s.LayerRouteRecord(c, s.Room.Map)
		exitShape := r.Record == ([18]byte{}) || (hasServiceRecord && r.Record == serviceRecord)
		// [MERGE-20260928-LAYER-SEQUENCE-FINAL] 不再要求「房里怪已清」——剧情层图的
		// 怪全是布景（100008950 的 59 行 team=1），客户端播完剧情就自己发 CMD45 走人。
		// exitShape 保留：奥德赛的落点记录必须对得上，防篡改。
		if exitShape && s.AtLayerLastMap() {
			// [MERGE-20260928-LAYER-SEQUENCE-EXIT] 序列走完一律**前进**（同下方通用分支）。
			// 不能回 base：回 base 会重置客户端的层索引，它又自动播第一张/这一格的层图，
			// 形成 base↔层图 死循环（实机 100004968 贵族机要 973→974→356→973）。
			// [MERGE-20260928-START-LAYER-EXIT] 层图格**就是起点**时更要前进：
			// 100004782 的 (0,0) 既是 Start 又挂着层图 [100015938]，回 base 等于回到
			// 起点、客户端进这一格时本来就会自动播那张层图，于是 675↔938 来回走，
			// 最后退化成「同图再进同图」直接黑屏闪退（实机 2026-09-28）。
			if next, e := s.layerSequenceAdvance(c, r.Position); e == nil {
				return next, nil
			}
			if base, ok := s.layerExitBase(r.Position); ok {
				room := s.Room
				room.Map = base
				return s.enterRoom(c, room)
			}
		}
		return nil, fmt.Errorf("no next source scene")
	}

	for _, layer := range s.Maze.Layers {
		if layer.Position != r.Position {
			continue
		}
		currentIdx := -1
		for i, mapID := range layer.Maps {
			if mapID == s.Room.Map {
				currentIdx = i
				break
			}
		}
		nextIdx := currentIdx + 1
		if nextIdx >= len(layer.Maps) {
			if currentIdx == len(layer.Maps)-1 && s.lotusClosingRevisit(r) {
				next, err := s.enterRoom(c, s.Room)
				if err != nil {
					return nil, err
				}
				next.lotusClosingReached = true
				return next, nil
			}
			// [KEEP-LOCAL] 本仓库自有的「源匹配终末演出」路径（53ffd56，见 oculus_terminal_layer_test.go），
			// 包内以 LAYER-SEQUENCE-FINAL 取代；这里保留作为**前置精确匹配**，两者并存不冲突。
			if currentIdx == len(layer.Maps)-1 && s.terminalSceneRevisit(c, r) {
				next, err := s.enterRoom(c, s.Room)
				if err != nil {
					return nil, err
				}
				next.terminalSceneClosingReached = true
				return next, nil
			}
			// [MERGE-20260927-SCENE-EXIT] 同奥德赛分支：layer 走到末尾、该位置有 base
			// 房间时视为从场景房退出。
			// [MERGE-20260928-LAYER-SEQUENCE-FINAL] 收紧又放宽的经过，别再回退：
			//   1) 只认「房里没有可战斗怪」(LayerRoomIsCinematic) → 漏掉打完的战斗层图；
			//   2) 改认「房里怪已清」(roomEnemiesDead) → 又漏掉**剧情层图**：实机
			//      2026-09-28「晦月湖」100004777 的 (1,0) 层图 100008950 是带剧情演出的
			//      图，[monster] 段 59 行 team=1 全是布景，客户端进图 0.2 秒后自己发
			//      CMD45 要去下一张，服务端却因「怪没清」拒成 "no next layer map"，
			//      角色卡在图上不再显示。
			// 真正的信号是**客户端自己在序列末尾请求换图**：CMD45 的 layer 切换 + 该
			// 位置层图序列已走到最后一张，就该放行。Record 不能参与判据 —— 剧情层图
			// 的请求带着客户端自己的落点记录（如 000100000405640064…），和 route 表里
			// 服务端补的那份不相等，加了它这里永远匹配不上。
			if currentIdx == len(layer.Maps)-1 && s.AtLayerLastMap() {
				// [MERGE-20260928-DIAG] 把这一步的决策落进诊断串，便于实机取证。
				s.noteSceneDiagnostic(
					"general layer exit: room=%d pos=%v maps=%d idx=%d record=%x",
					s.Room.Map, layer.Position, len(layer.Maps), currentIdx, r.Record[:])
				// [MERGE-20260928-LAYER-SEQUENCE-EXIT] 序列走完一律**前进**到相邻格，
				// **不能回 base**：回 base 会重置客户端的层索引，进这一格时它又自动播
				// 那张层图，于是 base↔层图 来回走 —— 实机 2026-09-28「晦月湖」
				// 100004777 的 (1,0) 就是 100015634↔100008950 每秒来回一次、一直重看剧情。
				// 原条件只在「多张序列 || 层图格就是起点」时前进（见下方 START-LAYER-EXIT），
				// 单张的仍回 base —— 那条限制正是这次死循环的来源，已去掉。
				// [MERGE-20260928-START-LAYER-EXIT] 起点层图格更要前进（回 base 就是回
				// 起点自己，客户端进这一格时本来就会自动播那张层图，形成 675↔938 来回走，
				// 最后退化成「同图再进同图」直接黑屏闪退）。
				if next, e := s.layerSequenceAdvance(c, layer.Position); e == nil {
					return next, nil
				}
				if base, ok := s.layerExitBase(layer.Position); ok {
					room := s.Room
					room.Map = base
					return s.enterRoom(c, room)
				}
			}
			// [MERGE-20260928-DIAG] 走到这里说明前面前进/回 base 都没成，记下原因。
			s.noteSceneDiagnostic(
				"no next layer map: room=%d pos=%v maps=%d idx=%d AtLayerLastMap=%v record=%x layerCount=%d",
				s.Room.Map, layer.Position, len(layer.Maps), currentIdx,
				s.AtLayerLastMap(), r.Record[:], s.layerMapCount(layer.Position))
			return nil, fmt.Errorf("no next layer map")
		}
		nextMap := layer.Maps[nextIdx]
		if _, ok := c.Maps[nextMap]; !ok {
			return nil, fmt.Errorf("layer target map not imported %d", nextMap)
		}
		room := s.Room
		room.Map = nextMap
		return s.enterRoom(c, room)
	}

	return nil, fmt.Errorf("no layer configured for room position")
}

// [MERGE-20260928-CINEMATIC-LAYER] LayerRoomIsCinematic 报告当前层图是否为
// 「演出层图」：坐在 layer 位置上，且**没有任何战斗迹象** —— 既没有可战斗的怪，
// 也没有怪被确认打死过。
//
//	演出层图（如 100004944 的 100016083_scene_0）：房里的 display actor 是
//	noncombat，客户端从不打它、也不会报告它死亡，点门后不会自己推进，没有出口
//	兜底就永远出不去 —— 这种要兜底。
//	安图恩讨伐战 100004950 的 100016165：4 只 team=100 的可战斗怪，要打。
//	德洛斯矿山 100004959 的 100016270：3 只 team=0 noncombat 的**可击败**演员，
//	客户端照样会打并发 CMD39 —— 只看 noncombat 会把它误判成演出图，兜底把玩家弹回
//	base 100016541（剧情 _start 图），客户端再进层图、再被弹回，来回循环（实机
//	2026-09-28 的症状就是「放一次技能就重看一次剧情」）。所以还要看死亡记录。
func (s *Session) LayerRoomIsCinematic() bool {
	if s == nil {
		return false
	}
	for _, m := range s.Monsters {
		if m.Team != 0 && !m.NonCombat {
			return false
		}
		if s.Dead[m.Entity] {
			return false
		}
	}
	return true
}

// [MERGE-20260928-LAYER-SEQUENCE-EXIT] layerMapCount 报告某位置的 layer 序列张数。
func (s *Session) layerMapCount(pos [2]byte) int {
	if s == nil {
		return 0
	}
	for _, layer := range s.Maze.Layers {
		if layer.Position == pos {
			return len(layer.Maps)
		}
	}
	return 0
}

// layerSequenceExit 报告「多张 layer 的序列走完后」应该前进到的相邻房间坐标。
//
// 多张序列（如实机 100004968 贵族机要的 (0,2): [100015974, 100016356]）走完最后
// 一张后**不能回 base**：回 base 会重置客户端的层索引，它又从第一张重播，形成
// 973→974→356→973 的死循环 —— 症状是「放一次技能就重看一次剧情」。
// 前进到相邻房间即可（Move 会再按 latestLayer 决定那一格要不要进层图）。
func (s *Session) layerSequenceExit(pos [2]byte) ([2]byte, bool) {
	if s == nil {
		return [2]byte{}, false
	}
	room := s.layerSequenceRoom(pos)
	if room == nil {
		return [2]byte{}, false
	}
	// [MERGE-20260928-LAYER-SEQUENCE-FINAL] 优先选**不带层图**的相邻格。
	// 落进另一个挂层图的格子，客户端进去又会自动播那张层图 —— 等于换了个地方
	// 继续循环（实机 100004777 的 (1,0) 若前进到 (0,0)，那里挂着 [100015633]）。
	// 没有干净的相邻格时才退而求其次。
	var fallback [2]byte
	hasFallback := false
	for _, r := range s.Maze.Rooms {
		dx := int(r.X) - int(room.X)
		dy := int(r.Y) - int(room.Y)
		if dx*dx+dy*dy != 1 {
			continue
		}
		cand := [2]byte{r.X, r.Y}
		if s.layerMapCount(cand) == 0 {
			return cand, true
		}
		if !hasFallback {
			fallback, hasFallback = cand, true
		}
	}
	if hasFallback {
		return fallback, true
	}
	return [2]byte{}, false
}

// layerSequenceRoom 按坐标取源 maze 里的房间，取不到返回 nil。
func (s *Session) layerSequenceRoom(pos [2]byte) *catalog.DungeonRoom {
	if s == nil {
		return nil
	}
	for _, r := range s.Maze.Rooms {
		if [2]byte{r.X, r.Y} == pos {
			v := r
			return &v
		}
	}
	return nil
}

// layerSequenceAdvance 是「层图序列走完 → 前进到相邻格」的统一入口。
//
// 它**不能**走 Session.Move：Move 开头有 `if !s.RoomCleared()`，要求当前房间的怪
// 全部清掉。剧情层图的怪是布景（100008950 的 59 行全是 team=1），永远清不掉 ——
// 实机 2026-09-28 就是因为 Move 失败而回落到「回 base」，而 base 这一格客户端进去
// 又会自动播那张层图，于是 100015634↔100008950 每秒来回一次、一直重看剧情。
// 客户端已经在序列末尾主动发了 CMD45，这里直接按目标格建房间即可。
func (s *Session) layerSequenceAdvance(c catalog.DungeonCatalog, pos [2]byte) (*Session, error) {
	target, ok := s.layerSequenceExit(pos)
	if !ok {
		s.noteSceneDiagnostic("%s | advance: no adjacent room for pos=%v layers=%+v",
			s.sceneDiagnostic, pos, s.Maze.Layers)
		return nil, fmt.Errorf("no adjacent room for layer sequence exit")
	}
	room := s.layerSequenceRoom(target)
	if room == nil {
		s.noteSceneDiagnostic("%s | advance: target %v absent from maze", s.sceneDiagnostic, target)
		return nil, fmt.Errorf("adjacent room absent from source maze")
	}
	next, err := s.enterRoom(c, s.latestLayer(*room))
	if err != nil {
		s.noteSceneDiagnostic("%s | advance: enterRoom(%v map=%d) failed: %v",
			s.sceneDiagnostic, target, s.latestLayer(*room).Map, err)
		return nil, err
	}
	s.noteSceneDiagnostic("%s | advance ok: %v -> %v map=%d (layerMaps=%d)",
		s.sceneDiagnostic, pos, target, next.Room.Map, s.layerMapCount(target))
	return next, nil
}

// ExitSceneRoom 处理「场景房点门」的出口。
//
// 与 layerSequenceAdvance 是**两条语义相反**的路，别再合并：
//   - 场景房点门（客户端只发 CMD38）→ **回 base**。实机 100004944 的 100016083_scene_0、
//     100004968 的贵族机要场景房都走这条。
//   - 序列末尾换图（客户端主动发 CMD45）→ 前进。实机 100004777 的 100008950 走这条。
//
// [MERGE-20260928-START-LAYER-EXIT] 但**层图格就是迷宫起点**时例外：回 base 等于回到
// 起点自己，客户端进这一格时本来就会自动播那张层图 → 又放一次剧情 → 再点门 → 再回
// base，循环到黑屏闪退（实机 2026-09-28「晦月湖」100004777 的 (0,0)：base 100008953
// 既是 Start 又挂着层图 [100015633]，进图 3 秒必闪退）。这种格必须**前进**。
//
// 同样不能走 Session.Move（RoomCleared 会拦住剧情层图的布景怪）。
func (s *Session) ExitSceneRoom(c catalog.DungeonCatalog, pos [2]byte) (*Session, error) {
	if s.LayerAtStart(pos) {
		if next, err := s.layerSequenceAdvance(c, pos); err == nil {
			return next, nil
		}
	}
	base, ok := s.layerExitBase(pos)
	if !ok {
		s.noteSceneDiagnostic("%s | scene exit: no base at %v", s.sceneDiagnostic, pos)
		return nil, fmt.Errorf("scene room has no base at %v", pos)
	}
	room := s.Room
	room.Map = base
	s.noteSceneDiagnostic("%s | scene exit: %v layer=%d -> base=%d", s.sceneDiagnostic, pos, s.Room.Map, base)
	return s.enterRoom(c, room)
}

// [MERGE-20260928-LAYER-SEQUENCE-EXIT] LayerRouteRecord 取某层图所在位置最后一条
// 场景路由的换图记录。
//
// 客户端在层图里点门只发 id=38（不带换图记录），服务端要自己补。而 StartMap 的
// Transition 记录里**带落点**（record[6:10]，与 lotusClosingRevisit 读法一致）：
// 100004968 的记录是 [0,0,0,0,4,5,127,1,20,1,0,0,3,0,2,0,0,0]，而 100004944 的
// 是全零（那张图自带 [dungeon start area]，不需要）。补全零等于让客户端用默认落点，
// 角色会卡在场景左上角（实机 2026-09-28 贵族机要）。
//
// 按**位置**而不是按 from 查：序列末尾的图（100004968 的 100016356）没有出边，
// 它自己的那张图就是靠同位置最后一条记录进来的。
func (s *Session) LayerRouteRecord(c catalog.DungeonCatalog, mapID uint32) ([18]byte, bool) {
	if s == nil {
		return [18]byte{}, false
	}
	var pos [2]byte
	onLayer := false
	for _, layer := range s.Maze.Layers {
		for _, m := range layer.Maps {
			if m == mapID {
				pos, onLayer = layer.Position, true
			}
		}
	}
	if !onLayer {
		return [18]byte{}, false
	}
	var rec [18]byte
	ok := false
	for _, route := range c.SceneRoutes {
		if route.Dungeon != s.Definition.ID || route.Maze != s.Maze.Index || route.Position != pos {
			continue
		}
		rec, ok = route.Record, true
	}
	return rec, ok
}

// [MERGE-20260928-LAYER-SEQUENCE-EXIT] atLayerLastMap 报告当前房间是否是所在层图序列的
// **最后一张**。
//
// 出口兜底（回 base / 前进）只允许在这里生效。序列中途的层图客户端会自己往下播，
// 替它兜底会把玩家弹回上一个房间、客户端又从头重播，来回循环；多张序列还会退化成
// 「同图再进同图」，StartMap 发 ReuseRoom 而客户端没有该图的缓存 —— 直接崩溃。
// 实机 2026-09-28 苏醒之森之后的那批副本就是这样闪退的（100004981 的 100001054，
// 4 张层图里的第 2 张，房里 9 个全是 noncombat 演员，被误判成演出图）。
func (s *Session) AtLayerLastMap() bool {
	if s == nil {
		return false
	}
	for _, layer := range s.Maze.Layers {
		for i, m := range layer.Maps {
			if m == s.Room.Map {
				return i == len(layer.Maps)-1
			}
		}
	}
	return false
}

// [MERGE-20260928-START-LAYER-EXIT] layerAtStart 报告某位置是否既是 layer 格又是迷宫起点。
//
// 这种格子上「回 base」没有任何意义 —— base 就是它自己，而客户端进入该位置时本来
// 就会自动播那张层图，于是形成 起点↔层图 的死循环（实机 2026-09-28 100004782 的
// (0,0)：675↔938 来回，最后黑屏闪退）。这类格必须**前进**。
func (s *Session) LayerAtStart(pos [2]byte) bool {
	if s == nil {
		return false
	}
	if s.Maze.Start != pos {
		return false
	}
	for _, layer := range s.Maze.Layers {
		if layer.Position == pos && len(layer.Maps) > 0 {
			return true
		}
	}
	return false
}

// layerExitBase 报告某位置是否有可回的 base（非 layer）房间地图。
//
// [MERGE-20260927-SCENE-EXIT] 场景房（layer 地图，如 100016083_scene_0）的出口在
// scene_routes 里没有条目 —— 那张表只描述「从 base 进入场景房」。客户端在场景房里
// 点门只发 id=38（不是 layer 切换），因此服务端要自己补这条出口：回到该位置的
// base 房间。找不到 base 时返回 false，调用方维持原来的报错行为。
func (s *Session) layerExitBase(pos [2]byte) (uint32, bool) {
	if s == nil {
		return 0, false
	}
	for _, layer := range s.Maze.Layers {
		if layer.Position != pos {
			continue
		}
		for _, room := range s.Maze.Rooms {
			if [2]byte{room.X, room.Y} == pos && room.Map != 0 {
				return room.Map, true
			}
		}
	}
	return 0, false
}

// A terminal cinematic without a boss identity may revisit its cached layer
// after clearing the quest's source objective room. Match the exact source
// CMT landing area imported for this maze; cinematic actors in the last map
// are not a room-clear precondition.
func (s *Session) terminalSceneRevisit(c catalog.DungeonCatalog, r protocol.DungeonRoomTransition) bool {
	if s.completionTarget != 0 || s.hasFightableBoss() || s.reportableDisplayBoss() != 0 ||
		r.Record[0] != 0 || r.Record[1] != 0 || r.Record[2] != 0 ||
		r.Record[3] != 0 || r.Record[4] != 4 || r.Record[5] != 5 {
		return false
	}
	x := binary.LittleEndian.Uint16(r.Record[6:8])
	y := binary.LittleEndian.Uint16(r.Record[8:10])
	for _, scene := range c.TerminalScenes {
		if scene.Source != c.Source.Checksum || scene.Dungeon != s.Definition.ID || scene.Maze != s.Maze.Index ||
			scene.Quest != s.Maze.Quest || scene.Position != r.Position || scene.FinalMap != s.Room.Map ||
			scene.DungeonSHA256 != s.Definition.Script.SHA256 || scene.MapSHA256 != c.Maps[s.Room.Map].SHA256 ||
			x < scene.XMin || x > scene.XMax || y < scene.YMin || y > scene.YMax {
			continue
		}
		objective, seen := s.Visited[scene.ObjectiveMap]
		if !seen {
			return false
		}
		for _, m := range objective {
			if m.Team != 0 && !m.NonCombat && !s.Dead[m.Entity] {
				// The source cinematic can destroy its sole boss without a CMD39.
				// This exception is imported only when that exact source action
				// and cinematic contain the matching monster DESTROY event.
				if len(objective) != 1 || scene.ObjectiveCinematicDestroyTemplate == 0 ||
					m.Template != scene.ObjectiveCinematicDestroyTemplate {
					return false
				}
			}
		}
		return true
	}
	return false
}
