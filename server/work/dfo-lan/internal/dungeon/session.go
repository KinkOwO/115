package dungeon

import (
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"time"
)

type Session struct {
	RunID     string
	StartedAt time.Time
	// Difficulty 是这次进图实际用的难度，沿用客户端 1 起算的编号
	// （1=普通 2=专家 3=达人 4=王者 5=英雄；奥德赛恒等于副本的
	// [designated difficulty]）。掉落按它取难度加成，见 loot.Session.Death。
	Difficulty byte
	// HellPosition is the current DGN's sealed Hell Party room, advertised in
	// NOTI28. Nil retains the native absent sentinel (255,255).
	HellPosition   *[2]byte
	HellParty      *HellPartyRun
	HellClearSent  bool
	hellLastDeaths map[[2]uint16]uint16
	Definition     catalog.DungeonDefinition
	Maze           catalog.DungeonMaze
	Room           catalog.DungeonRoom
	Monsters       []protocol.DungeonMonster
	Tournament     *TournamentRun
	Loaded         bool
	Dead           map[uint16]bool
	// 沉月湖（Moon Lake）单人攻坚状态。只有该频道启用了 moonConfig 时才会被写入，
	// 普通副本与军团全程保持零值。
	MoonFirstGrid                     [5]byte
	MoonZermioSpawnedAt               time.Time
	MoonZermioReportRoom              uint64
	MoonZermioHealth                  uint64
	MoonZermioMeter                   uint32
	MoonRetired                       map[uint16]bool
	MoonFeverUntil, MoonFeverCooldown time.Time
	MoonDynamic                       map[uint16]protocol.UnassignedMonster115
	MoonGridReady                     bool
	MoonGrid                          [5][3]byte
	MoonZermioGrid                    [2]int32
	MoonZermioDefeated                bool
	MoonNamed                         map[uint32]MoonNamedState
	MoonCleared                       [5]bool
	MoonScored                        map[uint16]bool
	MoonTroopScore, MoonFeverScore    uint32
	PhaseHistory                      []MoonCompletedPhase
	// Unowned marks a monster this character did not kill. It dies and the
	// room clears, but it pays no loot and no experience.
	Unowned       map[uint16]bool
	Visited       map[uint32][]protocol.DungeonMonster
	ScriptWarps   map[uint32]bool
	NextEntity    uint16
	Extra         uint16
	WeeklyRewards [100]byte
	SeasonRewards [100]byte
	// companions are friendly map-native APCs already encountered in this
	// run. A later room that does not declare the same AIC receives a dynamic
	// NOTI29 row through the client's native SourceIndex=10000 branch.
	companions                  []protocol.DungeonMonster
	completionTarget            uint16
	completed                   bool
	lotusClosingReached         bool
	terminalSceneClosingReached bool
	// ArenaBoss 声明「进图房间本身就是这场战斗的 boss 竞技场」：军团阶段本
	// （伊斯大陆 nemaug/nagor/ashcore/itrenog）的源迷宫里 [boss map] 是未被
	// 使用的源元数据（100002987 迷宫 0 的 boss 坐标在 (0,0)/100006472），
	// 而官服实证（s4 帧 451/550）把玩家放进 [start map] 房间 (1,1)/100006476
	// 并在那里开打、在那里结算 —— N28 仍回源迷宫的 boss 坐标。该标志只放宽
	// BossCheck/完成判定的「房间归属」守卫，目标仍必须是房内真实存在的源领主
	// （rank3 / APC，team≠0），不会把任意小怪当 boss 放行。只由军团阶段入场
	// 路径（cmd/wireprobe ispins_flow）置位，普通副本恒为零值。
	ArenaBoss                  bool
	// sceneDiagnostic 记录最近一次场景换图走了哪条判定分支，仅供排查（见 SceneDiagnostic）。
	sceneDiagnostic string
	// layerRecord 是客户端主动进当前层图时带来的换图记录（见 SceneEntryRecord）。
	layerRecord    [18]byte
	hasLayerRecord bool
	// sceneBaseRooms retains the original combat room after a verified scene
	// return. Later doorway visits must not select its already-finished layer.
	sceneBaseRooms map[[2]byte]uint32
}

func Select(c catalog.DungeonCatalog, r protocol.DungeonSelection, level byte, accepted map[uint16]bool) (*Session, error) {
	d, ok := c.Dungeons[r.ID]
	if !ok {
		return nil, fmt.Errorf("dungeon absent from imported source")
	}
	if uint32(level) < d.MinimumLevel {
		return nil, fmt.Errorf("dungeon minimum level not met")
	}
	// 客户端的难度是 1 起算的（1=普通 2=专家 3=达人 4=王者 5=英雄），
	// 原来这里要求 Difficulty==0，结果正常选图全被拒
	// （实测日志：dungeon_request_refused / unsupported dungeon option，
	//   请求字节 Difficulty=1 而客户端界面显示的就是 Normal）。
	// 现在放宽到 0..5，其余字段仍然严格校验。
	if r.Difficulty > 5 || (d.Odyssey && r.Difficulty != d.DesignatedDifficulty) {
		return nil, fmt.Errorf("unsupported dungeon difficulty %d", r.Difficulty)
	}
	// Elvenmere 特殊副本（ID 100003126）支持在选图界面选择 Zone（Extra 为选中的初始层数 1..100，如 1、36、61、86），
	// 其余普通副本严格要求 r.Extra == 0。
	extraValid := r.Extra == 0
	if d.ID == 100003126 && r.Extra <= 100 {
		extraValid = true
	}
	if d.Tutorial || !extraValid || r.Mode > 1 || r.Flag != 0 || r.Party != 65535 || r.Reserved != 0 || r.Tail != 0 || r.Options != [2]byte{} || r.Event != 0 {
		return nil, fmt.Errorf("unsupported dungeon option")
	}
	if r.Mode == 1 {
		if d.HellParty == nil || d.HellParty.SealMap == 0 {
			return nil, fmt.Errorf("Hell Party is absent from this dungeon source")
		}
		if _, ok := c.Maps[d.HellParty.SealMap]; !ok {
			return nil, fmt.Errorf("Hell Party seal map is not imported")
		}
	}
	if r.Quest > 65535 || r.Quest != 0 && !accepted[uint16(r.Quest)] {
		return nil, fmt.Errorf("quest is not accepted by this character")
	}
	mazeQuest, err := selectionMazeQuest(d, r.Quest, accepted)
	if err != nil {
		return nil, err
	}
	chosen, err := chooseMaze(d, mazeQuest)
	if err != nil {
		return nil, err
	}
	selected := chosen
	if r.Mode == 1 {
		selected, err = hellPartyMaze(chosen, *d.HellParty)
		if err != nil {
			return nil, err
		}
	}
	s, err := newSession(c, d, selected)
	if err != nil {
		return nil, err
	}
	s.Extra = r.Extra
	s.Difficulty = r.Difficulty
	if r.Mode == 1 {
		position := d.HellParty.SealPosition
		s.HellPosition = &position
		if c.HellRules != nil {
			s.HellParty, err = newHellPartyRun(c, s, func(total uint64) (uint64, error) { n, e := randomUint64(); return n % total, e })
			if err != nil {
				return nil, err
			}
			if s.Room.Map == s.HellParty.Map {
				s.Monsters = append(s.Monsters, s.HellParty.Rows...)
				if len(s.Monsters) > 255 {
					return nil, fmt.Errorf("Hell room roster exceeds native count")
				}
				s.Visited[s.Room.Map] = s.Monsters
			}
		}
	}
	if tournamentDungeon(d) {
		script, err := c.MapScript(s.Room.Map)
		if err != nil {
			return nil, err
		}
		run, actors, err := newTournamentRun(d, script, r.Difficulty)
		if err != nil {
			return nil, err
		}
		s.Tournament = run
		s.Monsters = actors
		s.Visited[s.Room.Map] = actors
		s.NextEntity = uint16(4096 + len(actors))
	}
	if d.ID == 100003126 && s.Extra > 1 {
		// 跳区入场（如从第 36、61、86 层开始），前面的层数标记为已通关/已领奖
		start := int(s.Extra)
		if start > 100 {
			start = 100
		}
		for i := 0; i < start-1; i++ {
			s.WeeklyRewards[i] = 1
			if (i+1)%5 == 0 {
				s.SeasonRewards[i] = 1
			}
		}
	}
	return s, nil
}

// azureMainDungeonID / azureMainMazeIndex 把蔚蓝号（Azure Main）的迷宫钉死。
//
// 官服实机用的是 maze 1（抓包 F16-s2c.txt #414 的 N28 `[7]` = 1）；三张迷宫只有
// (0,2)/(1,1)/(1,3) 三格的房间图不同，而官服那条主路径上唯一会踩到的是 (1,1)：
// maze 1 = 100012705（官服实测 2 只怪），maze 0 = 100012707（本服实测 27 只）。
// 默认规则取 index 最小（= 0），与抓包不一致，所以这里显式钉 1 以便逐帧对照。
const (
	azureMainDungeonID = 100004131
	azureMainMazeIndex = 1
)

// chooseMaze 在候选 maze 里挑一张。
//
// 默认规则（副本没有声明 MazeChanceRates）是「同 quest 里 index 最小者」——
// 这是原来的确定性行为，288 个多 maze 副本靠它保持稳定。声明了权重的副本
// （目前只有小深渊，见 configs/dungeons.maze-chance-rates.json）改为按权重
// 掷骰：源里的 [maze chance rate] 表达的就是「进哪张迷宫图」，而小深渊的两张
// 图一张是普通、一张是 `_special`（官方叫异空间）。
func chooseMaze(d catalog.DungeonDefinition, quest uint32) (catalog.DungeonMaze, error) {
	var candidates []catalog.DungeonMaze
	for _, m := range d.Mazes {
		// 源里同一个 quest 经常对应多张 maze（实测 3200 个副本里有 288 个副本的
		// quest==0 有多张，通常是不同难度或随机版本）。原来只要匹配到第二张就
		// 直接报 ambiguous source maze，玩家点图完全没反应。
		// 现在先跳过解析不完整的 maze，再在剩下的候选里决定。
		if uint32(m.Quest) != quest || len(m.Pending) > 0 {
			continue
		}
		candidates = append(candidates, m)
	}
	if len(candidates) == 0 {
		return catalog.DungeonMaze{}, fmt.Errorf("no resolved source maze for requested quest")
	}
	// 蔚蓝号：钉死 maze 1（见 azureMainMazeIndex 的说明）。
	if d.ID == azureMainDungeonID {
		for _, m := range candidates {
			if m.Index == azureMainMazeIndex {
				return m, nil
			}
		}
	}
	if len(d.MazeChanceRates) == len(d.Mazes) && len(candidates) > 1 {
		var total uint64
		for _, m := range candidates {
			total += uint64(d.MazeChanceRates[m.Index])
		}
		if total > 0 {
			roll, err := randomUint64()
			if err != nil {
				return catalog.DungeonMaze{}, err
			}
			return pickWeightedMaze(candidates, d.MazeChanceRates, roll%total), nil
		}
	}
	best := candidates[0]
	for _, m := range candidates[1:] {
		if m.Index < best.Index {
			best = m
		}
	}
	return best, nil
}

// pickWeightedMaze 是掷骰的纯函数部分：roll 必须落在 [0, 权重和)。
// 抽出来是为了让「边界落在哪一张」能被直接测，而不必驱动整个选图流程。
func pickWeightedMaze(candidates []catalog.DungeonMaze, rates []uint32, roll uint64) catalog.DungeonMaze {
	var acc uint64
	for _, m := range candidates {
		acc += uint64(rates[m.Index])
		if roll < acc {
			return m
		}
	}
	return candidates[len(candidates)-1]
}

func randomUint64() (uint64, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(raw[:]), nil
}

// resolveRoomMap 取该房间可用的地图脚本：先用主地图，主地图不在目录里时
// 按源里给出的顺序退到备选地图。源列出多张候选地图表示这张房可以是其中任意
// 一张（零售端按权重随机），因此选到任意一张已导入的都是合法结果。
func resolveRoomMap(c catalog.DungeonCatalog, r catalog.DungeonRoom) (catalog.DungeonRoom, catalog.ScriptRecord, bool, error) {
	if _, ok := c.Maps[r.Map]; ok {
		s, err := c.MapScript(r.Map)
		return r, s, err == nil, err
	}
	for _, alt := range r.Alternates {
		if _, ok := c.Maps[alt]; ok {
			r.Map = alt
			s, err := c.MapScript(alt)
			return r, s, err == nil, err
		}
	}
	return r, catalog.ScriptRecord{}, false, nil
}

// newSession builds the owned run for an already-resolved maze. Both the
// ordinary quest/town entry and the job tutorial entry share it so a tutorial
// room is spawned by exactly the same verified source rules.
func newSession(c catalog.DungeonCatalog, d catalog.DungeonDefinition, chosen catalog.DungeonMaze) (*Session, error) {
	s := &Session{Definition: d, Maze: chosen, StartedAt: time.Now()}
	var run [16]byte
	if _, e := rand.Read(run[:]); e != nil {
		return nil, e
	}
	s.RunID = hex.EncodeToString(run[:])
	// 源里同一个节点可以列多张候选地图（boss_selection_probability 是带权重的
	// 随机 BOSS 房），同一个起始坐标也可能出现多次。运行时的规则：
	// 优先用地图已导入的那一张；都没有才报错。
	// 原来只要坐标出现两次就报 ambiguous start room，只要首选地图没导入就报
	// start map not imported —— 于是这些图在客户端表现为"点了没反应"。
	var start catalog.DungeonRoom
	var script catalog.ScriptRecord
	found := false
	for _, room := range chosen.Rooms {
		if [2]byte{room.X, room.Y} != chosen.Start {
			continue
		}
		resolved, sc, ok, err := resolveRoomMap(c, room)
		if err != nil {
			return nil, err
		}
		if !ok {
			if start.Map == 0 {
				start = room
			}
			continue
		}
		start, script, found = resolved, sc, true
		break
	}
	if !found {
		if start.Map == 0 {
			return nil, fmt.Errorf("missing source start room")
		}
		return nil, fmt.Errorf("start map not imported")
	}
	s.Room = start
	monsters, e := fixedMonsters(script, d.BasisLevel)
	if e != nil {
		return nil, e
	}
	if triggeredSpawnsSuppressed() {
		monsters = dropTriggeredMonsters(script, monsters)
	}
	s.Monsters = monsters
	s.companions = rememberCompanions(nil, monsters)
	s.Dead = map[uint16]bool{}
	s.Visited = map[uint32][]protocol.DungeonMonster{s.Room.Map: monsters}
	s.NextEntity = uint16(4096 + len(monsters))
	return s, nil
}

func (s *Session) ConfirmDeath(entity uint32, killer, actor uint16) (bool, error) {
	if s == nil || !s.Loaded || actor == 0 || actor == 65535 {
		return false, fmt.Errorf("death does not belong to loaded solo actor")
	}
	for _, m := range s.Monsters {
		if uint32(m.Entity) == entity {
			if s.Tournament != nil && s.Dead[m.Entity] {
				return false, nil
			}
			if s.Tournament != nil && (s.Tournament.CurrentRound < 1 || s.Tournament.CurrentRound > 4 || m.Entity != s.Tournament.Opening.Path[s.Tournament.CurrentRound-1].Entity) {
				return false, fmt.Errorf("tournament opponent is outside current round")
			}
			// killerFFFF means the client attributes the death to nobody.
			// Live capture 20260912T001341 shows the whole boss room report
			// six deaths inside three milliseconds: 0x1015, 0x1019 and 0x101a
			// with this actor as killer, and 0x1018, 0x101b and 0x101c with
			// FFFF, because the boss dying clears the rest of its room. The
			// same sentinel carries the cinematic despawn already seen in
			// live76123. Refusing it withheld the death confirmation, so the
			// client kept those monsters alive, the gate never opened, and
			// the room could only be left by returning to town.
			//
			// A death with no killer is therefore accepted and clears the
			// room, but it stays unowned: no loot, no experience. A foreign
			// killer that names some other actor is still refused.
			unowned := killer == 65535
			// 奥德赛章节目标（[hunt boss]）是玩家亲手击杀的，但客户端在过场/清场
			// 阶段会把这次死亡归给「无人」（killer FFFF）—— 2026-09-23 实机三场里
			// 有两场的最终领主就是这样。它是本章金币与章节装备盒的唯一来源，判成
			// 无主就等于本章一件不掉。只对「奥德赛 + Rank3 + 与 [hunt boss] 同模板」
			// 这一只怪生效；被 BOSS 连带清场的杂兵仍是 FFFF、按原规则不给掉落。
			if unowned && s.Definition.Odyssey && s.Definition.HuntBoss != 0 && m.Rank == 3 && m.Template == s.Definition.HuntBoss {
				unowned = false
			}
			// 只有「点名了别人的击杀者」才算外来击杀；FFFF 是客户端自己放弃归属。
			if killer != actor && killer != 65535 {
				return false, fmt.Errorf("foreign combat killer")
			}
			if s.Dead[m.Entity] {
				return false, nil
			}
			if unowned {
				if s.Unowned == nil {
					s.Unowned = map[uint16]bool{}
				}
				s.Unowned[m.Entity] = true
			}
			s.Dead[m.Entity] = true
			s.recordHellDeath(m.Entity)
			if s.Tournament != nil {
				s.Tournament.CurrentRound++
			}
			s.tryComplete()
			return true, nil
		}
	}
	// 客户端本地召唤的怪不在投放列表里：`[SUMMON MONSTER]` 是客户端动作脚本里的行为，
	// 服务端从不参与，所以这类 entity 永远查不到。但它的死亡必须被确认 —— 不回确认，
	// 客户端就认为它还活着，脚本链条随之断掉。2026-09-26 小深渊实机即是此形态：
	// 天平（109019266）召唤的真 BOSS 打死后，天平的伤害箱 [SET DAMAGE BOX] 永远回不到 1，
	// 于是天平永远打不死。
	// 这类死亡一律按「无主」处理：回确认、不给掉落与经验。它不参与房间清空与通关判定，
	// 因为它的 entity 不在 s.Monsters 里，RoomCleared/tryComplete 本来就看不见它。
	// 这一段原先是 Elvenmere（100003126）的专用分支，现按同一理由泛化到所有副本，
	// 只加一道守卫：值必须落在正常的 entity 空间里。服务端投放的怪从 4096 起
	// （fixedMonsters 用 4096 + index），客户端本地召唤的怪共享同一空间；明显无效的
	// 值（如 100）仍然拒绝，既有契约不变。
	if s.HellParty != nil && entity <= 65535 {
		if _, reserved := s.HellParty.Actors[uint16(entity)]; reserved {
			return false, fmt.Errorf("Hell actor outside current seal room")
		}
	}
	if entity < 4096 {
		return false, fmt.Errorf("monster absent from current source room")
	}
	unowned := killer == 65535
	if killer != actor && !unowned {
		return false, fmt.Errorf("foreign combat killer")
	}
	if s.Dead == nil {
		s.Dead = map[uint16]bool{}
	}
	if s.Dead[uint16(entity)] {
		return false, nil
	}
	if s.Unowned == nil {
		s.Unowned = map[uint16]bool{}
	}
	s.Unowned[uint16(entity)] = true
	s.Dead[uint16(entity)] = true
	s.tryComplete()
	return true, nil
}

func (s *Session) RoomCleared() bool {
	if s == nil || !s.Loaded {
		return false
	}
	if s.Tournament != nil {
		return s.Tournament.CurrentRound > 4
	}
	if s.Room.Map == 100016294 {
		return true
	}
	if s.Definition.Odyssey {
		return true
	}
	keyRoom := s.warpKeyRoom()
	for _, m := range s.Monsters {
		if (!m.NonCombat || m.Rank == 3 && keyRoom) && !s.Dead[m.Entity] {
			return false
		}
	}
	return true
}
func (s *Session) Move(c catalog.DungeonCatalog, target [2]byte) (*Session, error) {
	if s.Completed() || s.completionTarget != 0 {
		// [MERGE-20260928-POSTBOSS-SCENE] 完成之后仍允许走向「还有剧情层图的相邻格」。
		// 苏醒之森 100004977 在 boss 房 (5,0) 就判完成，但后面还有 (6,0) 的过场
		// （层图 100017263，scene_route 100017262→100017263）。一律拒会把这最后一段
		// 挡在外面 —— 实机 2026-09-28 玩家打完 boss、点地图上的传送阵，服务端回
		// "boss completion is pending or already accepted"，传送阵过不去。
		if !s.adjacentLayerPending(c, target) {
			return nil, fmt.Errorf("boss completion is pending or already accepted")
		}
	}
	if !s.Loaded || (!s.Definition.MoveMapEvenEnemy && !s.RoomCleared()) {
		return nil, fmt.Errorf("current room not loaded or still has live enemies")
	}
	dx, dy := int(target[0])-int(s.Room.X), int(target[1])-int(s.Room.Y)
	if dx*dx+dy*dy != 1 {
		return nil, fmt.Errorf("target is not adjacent")
	}
	var room *catalog.DungeonRoom
	for _, r := range s.Maze.Rooms {
		if [2]byte{r.X, r.Y} == target {
			v := r
			room = &v
			break
		}
	}
	if room == nil {
		return nil, fmt.Errorf("target absent from source maze")
	}
	// Return to the last entered scene, including rooms with zero monsters.
	return s.enterRoom(c, s.latestLayer(*room))
}

// [MERGE-20260928-POSTBOSS-SCENE] adjacentLayerPending 报告目标格是否是「还没播完的
// 剧情层图」所在格：该格在 maze 里、位于当前房间的相邻位、且挂有 layer 地图。
//
// boss 房之后还接一段过场的副本（苏醒之森 100004977：(5,0) → (6,0) 的 100017263）
// 需要它 —— 否则 Move 的完成守卫会把这段挡在外面。
func (s *Session) adjacentLayerPending(c catalog.DungeonCatalog, target [2]byte) bool {
	if s == nil {
		return false
	}
	dx := int(target[0]) - int(s.Room.X)
	dy := int(target[1]) - int(s.Room.Y)
	if dx*dx+dy*dy != 1 {
		return false
	}
	for _, layer := range s.Maze.Layers {
		if layer.Position == target && len(layer.Maps) > 0 {
			return true
		}
	}
	return false
}

func (s *Session) enterRoom(c catalog.DungeonCatalog, room catalog.DungeonRoom) (*Session, error) {
	next := *s
	next.Room = room
	next.Loaded = false
	next.Visited = map[uint32][]protocol.DungeonMonster{}
	for id, m := range s.Visited {
		next.Visited[id] = m
	}
	monsters, seen := next.Visited[room.Map]
	if !seen {
		script, err := c.MapScript(room.Map)
		if err != nil {
			return nil, err
		}
		var e error
		monsters, e = fixedMonsters(script, s.Definition.BasisLevel)
		if e != nil {
			return nil, e
		}
		if triggeredSpawnsSuppressed() {
			monsters = dropTriggeredMonsters(script, monsters)
		}
		if gate, ok := s.MoonNamed[room.Map]; s.Definition.ID == 100004136 && ok && gate.Slot == 0 && gate.Planned {
			monsters = nil
		}
		for i := range monsters {
			if next.NextEntity >= 65535 {
				return nil, fmt.Errorf("monster identity exhausted")
			}
			monsters[i].Entity = next.NextEntity
			next.NextEntity++
		}
	}
	if !seen && next.HellParty != nil && room.Map == next.HellParty.Map {
		monsters = append(monsters, next.HellParty.Rows...)
		if len(monsters) > 255 {
			return nil, fmt.Errorf("Hell room roster exceeds native count")
		}
	}
	// Harvest only map-native friendly APCs. Ordinary team-0 cinematic
	// monsters and hostile/neutral APCs are intentionally excluded. A native
	// row in this room wins; otherwise add one dynamic row with the sentinel
	// source index proven in native145b20dc0.
	next.companions = rememberCompanions(s.companions, monsters)
	var e error
	monsters, e = appendMissingCompanions(monsters, next.companions, &next.NextEntity)
	if e != nil {
		return nil, e
	}
	next.Visited[room.Map] = monsters
	next.Monsters = monsters
	return &next, nil
}

const dynamicAPCSourceIndex uint32 = 10000

func rememberCompanions(known, monsters []protocol.DungeonMonster) []protocol.DungeonMonster {
	out := append([]protocol.DungeonMonster(nil), known...)
	byTemplate := make(map[uint32]int, len(out))
	for i, m := range out {
		byTemplate[m.Template] = i
	}
	for _, m := range monsters {
		if !m.APC || m.Team != 0 || m.SourceIndex == dynamicAPCSourceIndex {
			continue
		}
		m.Entity = 0
		m.SourceIndex = dynamicAPCSourceIndex
		m.NonCombat = true
		m.SourceTail = [2]int32{}
		if i, ok := byTemplate[m.Template]; ok {
			out[i] = m
			continue
		}
		byTemplate[m.Template] = len(out)
		out = append(out, m)
	}
	return out
}

func appendMissingCompanions(monsters, companions []protocol.DungeonMonster, nextEntity *uint16) ([]protocol.DungeonMonster, error) {
	present := make(map[uint32]bool, len(monsters))
	for _, m := range monsters {
		if m.APC && m.Team == 0 {
			present[m.Template] = true
		}
	}
	for _, companion := range companions {
		if present[companion.Template] {
			continue
		}
		if len(monsters) >= 255 || *nextEntity == 0 || *nextEntity >= 65535 {
			return nil, fmt.Errorf("companion identity exhausted")
		}
		companion.Entity = *nextEntity
		*nextEntity++
		companion.SourceIndex = dynamicAPCSourceIndex
		companion.Team = 0
		companion.NonCombat = true
		companion.APC = true
		if companion.Rank < 5 || companion.Rank > 8 {
			companion.Rank = 5
		}
		monsters = append(monsters, companion)
		present[companion.Template] = true
	}
	return monsters, nil
}

// ClearedMaps lists every source map this run actually entered. A room
// transition requires the previous room to be cleared, so a completed run has
// cleared each of them; a source [clear map] objective may name any one, not
// only the final boss room.
func (s *Session) ClearedMaps() []uint32 {
	if s == nil {
		return nil
	}
	seen := map[uint32]bool{}
	for id := range s.Visited {
		seen[id] = true
	}
	// A run that never moved has no visit history but has still cleared the
	// room it is standing in.
	if s.Room.Map != 0 {
		seen[s.Room.Map] = true
	}
	out := make([]uint32, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (s *Session) LivingMonsters() []protocol.DungeonMonster {
	var out []protocol.DungeonMonster
	for _, m := range s.Monsters {
		if !s.Dead[m.Entity] {
			out = append(out, m)
		}
	}
	return out
}

// Preserve source indices: the native map parser uses them to retrieve spawn
// coordinates, behavior and cinematic flags from its matching PVF map row.
// Only fixed, exactly-one spawns are supported here; random rows are refused.
// triggeredSpawnsSuppressed 报告运营者是否要求服务端停止投放
// [monster create trigger] 序号大于 1 的怪。
//
// 该序号的语义尚未确立（全目录 369 张带该段的 map 里 117 张非单调，
// 且它的最大值与副本 maze 数对不上），所以这是一条诊断开关，不是规则。
// 小深渊（100005014）的两张地图各自带 11 只 trigger=1 的杂兵，
// 另加 trigger=2 的天平 109019266 与 109019280（同坐标 3168,191）。
// 用这条开关实机对照一次，就能判定这两只该不该由服务端投。
// 默认关闭，行为与原先一致。
func triggeredSpawnsSuppressed() bool {
	return os.Getenv("DFO_SKIP_TRIGGERED_SPAWNS") == "1"
}

// dropTriggeredMonsters 剔除 [monster create trigger] 序号大于 1 的行。
// SourceIndex 就是行序号，而 trigger 段与怪物行并行，两者可按下标对齐。
func dropTriggeredMonsters(script catalog.ScriptRecord, monsters []protocol.DungeonMonster) []protocol.DungeonMonster {
	ordinals := createTriggerOrdinals(script.Cells)
	if len(ordinals) == 0 {
		return monsters
	}
	out := make([]protocol.DungeonMonster, 0, len(monsters))
	for _, m := range monsters {
		if int(m.SourceIndex) < len(ordinals) && ordinals[m.SourceIndex] > 1 {
			continue
		}
		out = append(out, m)
	}
	return out
}

// createTriggerAt 取第 index 行的 trigger 原值，越界或超出 byte 范围时返回 0。
func createTriggerAt(ordinals []int32, index int) byte {
	if index < 0 || index >= len(ordinals) {
		return 0
	}
	v := ordinals[index]
	if v < 0 || v > 255 {
		return 0
	}
	return byte(v)
}

func createTriggerOrdinals(cells []pvf.Token) []int32 {
	var out []int32
	active := false
	for _, c := range cells {
		if c.Type == 3 {
			active = c.Text == "[monster create trigger]"
			continue
		}
		if active && c.Type == 0 {
			out = append(out, c.Value)
		}
	}
	return out
}

func fixedMonsters(script catalog.ScriptRecord, basis uint32) ([]protocol.DungeonMonster, error) {
	var out []protocol.DungeonMonster
	active := false
	c := script.Cells
	// [monster create trigger] 与怪物行并行，按 SourceIndex 对齐后保留原值。
	ordinals := createTriggerOrdinals(c)
	for i := 0; i < len(c); i++ {
		if c[i].Type == 3 {
			active = c[i].Text == "[monster]"
			continue
		}
		if !active {
			continue
		}
		if i+8 > len(c) {
			return nil, fmt.Errorf("short monster row")
		}
		var v [8]int32
		for j := range v {
			if c[i+j].Type != 0 {
				return nil, fmt.Errorf("unsupported monster row")
			}
			v[j] = c[i+j].Value
		}
		i += 8
		fixed, rankSeen, nonCombat := false, false, false
		var rank byte
		for i < len(c) && c[i].Type == 6 {
			switch c[i].Text {
			case "[fixed]":
				fixed = true
			case "[named]":
				// The 115 client map reader sub_1471E9060 only maps
				// [normal]/[champion]/[super champion]/[boss] to ranks.
				// [named] leaves its default rank 0 (sub_1471C18C0).
				// Dazzlement maps use it as a named spawn tag.
				if rankSeen {
					return nil, fmt.Errorf("duplicate source monster rank")
				}
				rankSeen = true
			case "[NPC]":
				// Map 91757 uses [fixed] [NPC] 1020 [boss]. The NPC
				// association has one numeric operand before the rank.
				i++
				if i >= len(c) || c[i].Type != 0 {
					return nil, fmt.Errorf("short monster NPC option")
				}
			case "[normal]", "[champion]", "[boss]":
				// Monster ranks: normal 0, champion 1, boss 3. Live capture
				// 20260912T025417 refused CMD45 (move to the next room) with
				// "unresolved monster spawn option [champion]" - a champion in
				// the target room stalled dungeon progression entirely because
				// the parser only knew [normal] and [boss].
				if rankSeen {
					return nil, fmt.Errorf("duplicate source monster rank")
				}
				rankSeen = true
				switch c[i].Text {
				case "[champion]":
					rank = 1
				case "[boss]":
					rank = 3
				}
			case "[cinematic]": // The native source index retains animation/event behavior.
			case "[dummy]", "[displayhuntdummy]":
				nonCombat = true
			default:
				return nil, fmt.Errorf("unresolved monster spawn option %q", c[i].Text)
			}
			i++
		}
		// A champion row carries one extra trailing field after its options
		// that normal and boss rows do not. The sole champion in the runtime
		// catalog (map 58570) shows a single 0 here; without consuming it the
		// next row is read from the wrong offset and reports a template-0
		// "invalid source row". Consume exactly one trailing zero for a
		// champion so the rest of the room aligns.
		if rank == 1 && i < len(c) && c[i].Type == 0 && c[i].Value == 0 {
			i++
		}
		i--
		// The native constructor sub_1471C18C0 zeros the rank at +0x24.
		// sub_1471E9060 leaves it unchanged when no rank option matches.
		// White Land map 100004527 row 3 is [fixed] without a rank tag;
		// requiring rankSeen rejects a valid ordinary monster and its room.
		// Keep rankSeen for duplicate-option validation, not as a requirement.
		if !fixed || v[0] <= 0 || v[6] < 0 || v[7] < 0 || len(out) >= 255 {
			return nil, fmt.Errorf("unsupported random monster placement or invalid source row")
		}
		level := int64(v[2])
		if v[1] == 1 {
			level += int64(basis)
		} else if v[1] != 0 {
			return nil, fmt.Errorf("unsupported monster level expression")
		}
		if level == 0 && basis > 0 {
			level = int64(basis)
		}
		if level < 0 || level > 255 {
			return nil, fmt.Errorf("invalid monster level")
		}
		// Quest 3352's elevator room has an off-map template-1 sentinel at
		// (424,-364). Its elevator summons eleven local monsters, but the
		// sentinel never dies in either captured run. Sending it as a live
		// monster leaves the room occupied after those eleven are defeated.
		// The source's ordinary elevator room (16408) has the same control
		// objects without this sentinel.
		if script.Path == "map/cataclysm/northmyre/05_town_of_doubt/3352_76384.map" &&
			v[0] == 1 && v[3] == 424 && v[4] == -364 {
			continue
		}
		team := uint32(100)
		// The archer and gunblader tutorial maps place this normal-rank actor
		// below the playable area. Keep its source row/entity index for map
		// scripts, but don't let the unreachable actor keep the exit closed.
		offMapTutorialActor :=
			(script.Path == "map/cataclysm/newtutorial/archer_f_tutorial/100008880.map" && v[0] == 70216 && v[3] == 893 && v[4] == -333) ||
				(script.Path == "map/cataclysm/newtutorial/gunblader_m/70577.map" && v[0] == 70216 && v[3] == 1014 && v[4] == -311)
		if offMapTutorialActor {
			team = 0
			nonCombat = true
		}
		out = append(out, protocol.DungeonMonster{Entity: uint16(4096 + len(out)), SourceIndex: uint32(len(out)), Level: byte(level), Template: uint32(v[0]), Rank: rank, Team: team, NonCombat: nonCombat, SourceTail: [2]int32{v[6], v[7]}, CreateTrigger: createTriggerAt(ordinals, len(out))})
	}
	// Source teams are parallel to monster rows. Team0 supplies friendly
	// cinematic actors in this route; team100 supplies enemies. Never remove
	// either from the spawn list: client scripts address their source index.
	var teams []int32
	active = false
	for _, cell := range c {
		if cell.Type == 3 {
			active = cell.Text == "[monster team]"
			continue
		}
		if !active {
			continue
		}
		if cell.Type != 0 || cell.Value < 0 {
			return nil, fmt.Errorf("unresolved monster team")
		}
		teams = append(teams, cell.Value)
	}
	if len(teams) > 0 {
		if len(teams) != len(out) {
			return nil, fmt.Errorf("source monster/team count mismatch")
		}
		for i, team := range teams {
			out[i].Team = uint32(team)
			out[i].NonCombat = out[i].NonCombat || team == 0
		}
	}
	return appendFixedAPCs(script, out)
}

// Fixed map APCs use the ordinary NOTI29 list with rank5..8, not the
// separate random-APC list. Native1471d34ca parses their own source indices;
// 145b25e79 dispatches them to145b20dc0, which loads level from the AIC.
func appendFixedAPCs(script catalog.ScriptRecord, out []protocol.DungeonMonster) ([]protocol.DungeonMonster, error) {
	c := script.Cells
	active, index := false, uint32(0)
	for i := 0; i < len(c); {
		if c[i].Type == 3 {
			active = c[i].Text == "[ai character]"
			i++
			continue
		}
		if !active {
			i++
			continue
		}
		if i+4 > len(c) {
			return nil, fmt.Errorf("short fixed APC row")
		}
		for j := 0; j < 4; j++ {
			if c[i+j].Type != 0 {
				return nil, fmt.Errorf("invalid fixed APC coordinate")
			}
		}
		id := c[i].Value
		i += 4
		if id <= 0 || index >= 64 || len(out) >= 255 || i >= len(c) || c[i].Type != 6 {
			return nil, fmt.Errorf("invalid fixed APC identity/team")
		}
		var team uint32
		switch c[i].Text {
		case "[character]":
			team = 0
		case "[monster]":
			team = 100
		case "[neutral]":
			team = 200
		default:
			return nil, fmt.Errorf("unresolved fixed APC team %q", c[i].Text)
		}
		i++
		if i < len(c) && c[i].Type == 6 && c[i].Text == "[NPC]" {
			i++
			if i >= len(c) || c[i].Type != 0 {
				return nil, fmt.Errorf("short APC NPC option")
			}
			i++
		}
		if i < len(c) && c[i].Type == 6 && c[i].Text == "[cinematic]" {
			i++
		}
		if i >= len(c) || c[i].Type != 6 {
			return nil, fmt.Errorf("missing APC rank")
		}
		var rank byte
		switch c[i].Text {
		case "[normal]":
			rank = 5
		case "[champion]":
			rank = 6
		case "[super champion]":
			rank = 7
		case "[boss]":
			rank = 8
		default:
			return nil, fmt.Errorf("unresolved APC rank %q", c[i].Text)
		}
		i++
		if rank == 6 || rank == 7 {
			if i >= len(c) || c[i].Type != 0 || c[i].Value != 0 {
				return nil, fmt.Errorf("unresolved APC champion abilities")
			}
			i++
		}
		if i+2 > len(c) || c[i].Type != 0 || c[i+1].Type != 0 || c[i].Value < 0 || c[i].Value > 255 || c[i+1].Value < 0 || c[i+1].Value > 255 {
			return nil, fmt.Errorf("invalid APC tail")
		}
		out = append(out, protocol.DungeonMonster{Entity: uint16(4096 + len(out)), SourceIndex: index, Template: uint32(id), Rank: rank, Team: team, APC: true, NonCombat: team != 100, SourceTail: [2]int32{c[i].Value, c[i+1].Value}})
		index++
		i += 2
	}
	return out, nil
}
