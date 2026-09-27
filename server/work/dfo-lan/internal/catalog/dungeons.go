package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type DungeonRoom struct {
	X, Y byte
	Map  uint32
	Boss bool
	// Alternates 保存同一节点在 [map specification] 里给出的其余地图 ID。
	// 实测 dungeon 86 的 boss 房给了 4 个 ID（20314..20317）：一张图可以挂多张
	// 备选地图。运行时只用 Map，其余仍然导入地图目录，源数据不丢。
	Alternates []uint32 `json:"alternates,omitempty"`
}
type DungeonMaze struct {
	Index byte   `json:"index"`
	Quest uint16 `json:"quest"`
	// QuestFlag 是 [quest connection] 第三格的原值：实测有 -1 和 2 两种取值，
	// 与任务 ID 无关，规则引擎不建模它，按原样保留备查。
	QuestFlag         int32 `json:"quest_flag,omitempty"`
	Size, Start, Boss [2]byte
	Rooms             []DungeonRoom  `json:"rooms"`
	Layers            []DungeonLayer `json:"layers,omitempty"`
	Pending           []string       `json:"pending,omitempty"`
}
type DungeonLayer struct {
	Position [2]byte  `json:"position"`
	Maps     []uint32 `json:"maps"`
}
type DungeonDefinition struct {
	ID                       uint32       `json:"id"`
	Script                   ScriptRecord `json:"script"`
	MinimumLevel, BasisLevel uint32
	Tutorial, NoFatigue      bool
	Odyssey                  bool
	DesignatedDifficulty     byte
	HuntBoss                 uint32 // Source Odyssey [hunt boss] single-target completion.
	// SourceBoss 是副本脚本自己用 [clear condition] [hunt boss] <模板> <数量> 声明的
	// 通关领主：杀掉它就算通关。这是**源对通关条件的声明**，对所有副本成立，
	// 不是某个玩法的特例。
	//
	// 只有「客户端不发 CMD117」的副本才走得到它，见 internal/dungeon/completion.go
	// 的 tryComplete —— 客户端会发 CMD117 的副本由那条路径负责，这里不会重复结算。
	SourceBoss uint32
	Mazes      []DungeonMaze `json:"mazes"`
}
type DungeonCatalog struct {
	Source      pvf.ArchiveSnapshot          `json:"source"`
	Dungeons    map[uint32]DungeonDefinition `json:"dungeons"`
	Maps        map[uint32]ScriptRecord      `json:"maps"`
	Skipped     []string                     `json:"skipped,omitempty"`
	SceneRoutes []DungeonSceneRoute          `json:"scene_routes,omitempty"`
}

// DungeonSceneRoute is generated from the maze order and original CMT/ACT landing area.
type DungeonSceneRoute struct {
	Source        string   `json:"source"`
	Dungeon       uint32   `json:"dungeon"`
	Maze          byte     `json:"maze"`
	Position      [2]byte  `json:"position"`
	From          uint32   `json:"from_map"`
	To            uint32   `json:"to_map"`
	Record        [18]byte `json:"record"`
	DungeonSHA256 string   `json:"dungeon_sha256"`
	MapSHA256     string   `json:"map_sha256"`
}

func dungeonPair(c []pvf.Token) (r [2]byte, e error) {
	if len(c) != 2 {
		return r, fmt.Errorf("coordinate pair required")
	}
	for i, v := range c {
		if v.Type != 0 || v.Value < 0 || v.Value > 255 {
			return r, fmt.Errorf("invalid coordinate")
		}
		r[i] = byte(v.Value)
	}
	return r, nil
}

// absentCoordinate 是源里表示「没有这个房间」的哨兵 -1 -1。
// 协议层用 255/255 表示 absent（见 protocol/dungeon.go 的 native sentinel 注释），
// 而房间坐标必然小于 [size] 且 size<=255，所以 255/255 永远不会匹配到房间。
var absentCoordinate = [2]byte{255, 255}

// sourceFirstPair 读一段坐标区间的第一组坐标。
// [size] 只有一组；[start map]/[boss map] 可以列多组候选房间
// （实测 dungeon 92 的 [start map] 是 (1,4) 和 (3,4) 两组，原来只接受恰好两个
// 数值，于是整张 maze 被标成 [start map]: coordinate pair required）。
// 运行时只需要一个房间，取第一组，其余候选忽略。
// absentOK 为真时接受 -1 -1 哨兵：实测 dungeon 20000 的 [boss map] 就是 -1 -1，
// 表示这张 maze 没有 boss 房，不是解析失败。
func sourceFirstPair(c []pvf.Token, absentOK bool) ([2]byte, error) {
	if len(c) == 0 || len(c)%2 != 0 {
		return [2]byte{}, fmt.Errorf("coordinate pair required")
	}
	if absentOK && c[0].Type == 0 && c[0].Value == -1 && c[1].Type == 0 && c[1].Value == -1 {
		return absentCoordinate, nil
	}
	return dungeonPair(c[:2])
}

// sourceBoss reads the script's own clear condition: [clear condition] holds one
// [hunt boss] <template> <count> pair per maze, and killing that template is what
// clears the run. Sources repeat it once per maze rather than once per dungeon, so
// every pair must agree on a single template with count 1, or the reading is left
// at zero rather than guessed - settling a run on the wrong monster's death is
// worse than not settling it.
//
// Scoped to [clear condition] on purpose: [hunt boss] also appears in other
// blocks, and only the clear condition makes a statement about completion.
func sourceBoss(cells []pvf.Token) uint32 {
	var pairs []int32
	inClear, inHunt := false, false
	for _, c := range cells {
		if c.Type == 3 {
			switch c.Text {
			case "[clear condition]":
				inClear = true
				inHunt = false
			case "[/clear condition]":
				inClear = false
				inHunt = false
			case "[hunt boss]":
				inHunt = inClear
			default:
				inHunt = false
			}
			continue
		}
		if !inHunt || c.Type != 0 {
			continue
		}
		pairs = append(pairs, c.Value)
	}
	if len(pairs) < 2 || len(pairs)%2 != 0 {
		return 0
	}
	boss := int32(0)
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i] <= 0 || pairs[i+1] != 1 {
			return 0
		}
		if boss == 0 {
			boss = pairs[i]
		} else if boss != pairs[i] {
			return 0
		}
	}
	return uint32(boss)
}

func ParseDungeon(id uint32, s ScriptRecord) (DungeonDefinition, error) {
	d := DungeonDefinition{ID: id, Script: s}
	mode := sectionCells(s.Cells, "[dungeon mode script]")
	d.Odyssey = len(mode) == 1 && mode[0].Type == 6 && mode[0].Text == "arad odyssey"
	if d.Odyssey {
		hunt := sectionCells(s.Cells, "[hunt boss]")
		if len(hunt) > 0 {
			if len(hunt) != 2 || hunt[0].Type != 0 || hunt[0].Value <= 0 || hunt[1].Type != 0 || hunt[1].Value != 1 {
				return d, fmt.Errorf("unsupported Odyssey hunt boss condition")
			}
			d.HuntBoss = uint32(hunt[0].Value)
		}
		v := sectionCells(s.Cells, "[designate dungeon difficulty]")
		if len(v) != 1 || v[0].Type != 0 || v[0].Value < 0 || v[0].Value > 4 {
			return d, fmt.Errorf("invalid Odyssey designated difficulty")
		}
		d.DesignatedDifficulty = byte(v[0].Value)
	}
	d.SourceBoss = sourceBoss(s.Cells)
	minimum := sectionCells(s.Cells, "[minimum required level]")
	if len(minimum) != 1 || minimum[0].Type != 0 || minimum[0].Value < 1 {
		return d, fmt.Errorf("invalid [minimum required level]")
	}
	d.MinimumLevel = uint32(minimum[0].Value)
	basis := sectionCells(s.Cells, "[basis level]")
	hasBasis := false
	for _, c := range s.Cells {
		hasBasis = hasBasis || c.Type == 3 && c.Text == "[basis level]"
	}
	if hasBasis {
		if len(basis) != 1 || basis[0].Type != 0 || basis[0].Value < 1 {
			return d, fmt.Errorf("invalid [basis level]")
		}
		d.BasisLevel = uint32(basis[0].Value)
	} else {
		d.BasisLevel = d.MinimumLevel
		recommended := sectionCells(s.Cells, "[recommended level]")
		if len(recommended) > 0 && recommended[0].Type == 0 && recommended[0].Value > 0 {
			d.BasisLevel = uint32(recommended[0].Value)
		}
	}
	for _, c := range s.Cells {
		if c.Type == 3 {
			d.Tutorial = d.Tutorial || c.Text == "[tutorial dungeon]"
			d.NoFatigue = d.NoFatigue || c.Text == "[no fatigue]"
		}
	}
	if strings.Contains(strings.ToLower(strings.ReplaceAll(s.Path, "\\", "/")), "/poongjintrainingroom/") {
		d.NoFatigue = true
	}
	for i := 0; i < len(s.Cells); i++ {
		if s.Cells[i].Type != 3 || s.Cells[i].Text != "[maze info]" {
			continue
		}
		start := i + 1
		i++
		for i < len(s.Cells) && !(s.Cells[i].Type == 3 && s.Cells[i].Text == "[maze info]") {
			i++
		}
		c := s.Cells[start:i]
		i--
		if len(d.Mazes) >= 256 {
			return d, fmt.Errorf("too many mazes")
		}
		m := DungeonMaze{Index: byte(len(d.Mazes))}
		q := sectionCells(c, "[quest connection]")
		if len(q) > 0 {
			// 第一格恒为 0，第二格是任务 ID，第三格是任务链接标记。
			// 原来要求第三格必须等于 -1，结果 dungeon 86 的 index5
			// （9 个房间的完整 maze，第三格是 2）被判成 unsupported，
			// Quest 留在 0，反过来又制造出第二张 quest==0 的 maze。
			if len(q) != 3 || q[0].Type != 0 || q[0].Value != 0 || q[1].Type != 0 || q[1].Value < 1 || q[1].Value > 65535 || q[2].Type != 0 {
				m.Pending = append(m.Pending, "unsupported quest connection")
			} else {
				m.Quest = uint16(q[1].Value)
				m.QuestFlag = int32(q[2].Value)
			}
		}
		for _, p := range []struct {
			name     string
			dst      *[2]byte
			absentOK bool
		}{{"[size]", &m.Size, false}, {"[start map]", &m.Start, false}, {"[boss map]", &m.Boss, true}} {
			cells := sectionCells(c, p.name)
			if p.name == "[size]" && len(cells) > 2 {
				// Dungeon 53, quest 3354 declares [size] 4 3 followed by
				// [size] 4 4. The final dimensions contain its start (3,3)
				// and all seven source rooms; the earlier dimensions do not.
				cells = cells[len(cells)-2:]
			}
			v, e := sourceFirstPair(cells, p.absentOK)
			if e != nil {
				m.Pending = append(m.Pending, p.name+": "+e.Error())
			} else {
				*p.dst = v
			}
		}
		nodes := sectionCells(c, "[map specification]")
		if len(nodes) == 0 {
			m.Pending = append(m.Pending, "unsupported room specification")
		} else {
			// 节点标签：map / boss 后面跟 1..N 个地图 ID；
			// boss_selection_probability 后面跟交替的 (地图 ID, 权重)
			// （实测 contents/2024/snk/.../farming_1.dgn 的随机 BOSS 房）。
			seen := map[[2]byte]bool{}
			layered := map[[2]byte]bool{}
			for j := 0; j < len(nodes); {
				label := nodes[j]
				weighted := label.Type == 6 && label.Text == "boss_selection_probability"
				if label.Type != 6 || (label.Text != "map" && label.Text != "boss" && label.Text != "layered" && !weighted) {
					m.Pending = append(m.Pending, "invalid room specification")
					break
				}
				// 一个节点的内容由下一个标签界定。原来按固定 4 格分组，遇到
				// 多地图节点就 len%4!=0，把整张 maze 判成
				// unsupported room specification、rooms 归零（实测 dungeon 86）。
				end := j + 1
				for end < len(nodes) && nodes[end].Type != 6 {
					end++
				}
				if end-(j+1) < 3 {
					m.Pending = append(m.Pending, "invalid room specification")
					break
				}
				xy, e := dungeonPair(nodes[j+1 : j+3])
				if e != nil || xy[0] >= m.Size[0] || xy[1] >= m.Size[1] {
					m.Pending = append(m.Pending, "invalid room specification")
					break
				}
				if label.Text == "layered" {
					if layered[xy] {
						m.Pending = append(m.Pending, "duplicate layered room")
						break
					}
					layered[xy] = true
					layer := DungeonLayer{Position: xy}
					bad := false
					for _, cell := range nodes[j+3 : end] {
						if cell.Type != 0 || cell.Value <= 0 {
							bad = true
							break
						}
						layer.Maps = append(layer.Maps, uint32(cell.Value))
					}
					if bad {
						m.Pending = append(m.Pending, "invalid layer map")
						break
					}
					m.Layers = append(m.Layers, layer)
					j = end
					continue
				}
				if d.Odyssey && seen[xy] {
					m.Pending = append(m.Pending, "duplicate base room")
					break
				}
				seen[xy] = true
				room := DungeonRoom{X: xy[0], Y: xy[1], Boss: label.Text != "map"}
				bad := false
				fields := nodes[j+3 : end]
				step := 1
				if weighted {
					step = 2
					if len(fields)%2 != 0 {
						bad = true
					}
				}
				for k := 0; !bad && k < len(fields); k += step {
					cell := fields[k]
					if cell.Type != 0 || cell.Value <= 0 {
						bad = true
						break
					}
					if weighted && fields[k+1].Type != 0 {
						bad = true
						break
					}
					if room.Map == 0 {
						room.Map = uint32(cell.Value)
						continue
					}
					room.Alternates = append(room.Alternates, uint32(cell.Value))
				}
				if bad {
					m.Pending = append(m.Pending, "invalid room specification")
					break
				}
				m.Rooms = append(m.Rooms, room)
				j = end
			}
			for xy := range layered {
				if !seen[xy] {
					m.Pending = append(m.Pending, "layered room has no base map")
				}
			}
		}
		d.Mazes = append(d.Mazes, m)
	}
	if len(d.Mazes) == 0 {
		return d, fmt.Errorf("no source maze")
	}
	return d, nil
}
func ImportDungeons(a *pvf.Archive, ids []uint32) (DungeonCatalog, error) {
	out := DungeonCatalog{Source: a.Snapshot(), Dungeons: map[uint32]DungeonDefinition{}, Maps: map[uint32]ScriptRecord{}}
	var skipped []string
	indices := make([]map[uint32]string, 2)
	for i, name := range []string{"list/dungeon.lst", "list/map.lst"} {
		s, e := ReadScript(a, name)
		if e != nil {
			return out, e
		}
		rows, e := ParseIndex(s.Cells)
		if e != nil {
			return out, e
		}
		indices[i] = map[uint32]string{}
		for _, r := range rows {
			indices[i][r.ID] = r.Path
		}
	}
	for _, id := range ids {
		name, ok := indices[0][id]
		if !ok {
			// 世界目录里有 3000 多个副本 ID，其中一部分在 list/dungeon.lst 里已不存在。
			// 批量导入时要跳过它们，而不是整批失败。
			skipped = append(skipped, fmt.Sprintf("dungeon %d not in list", id))
			continue
		}
		s, e := ResolveScript(a, name)
		if e != nil {
			skipped = append(skipped, fmt.Sprintf("dungeon %d: %v", id, e))
			continue
		}
		d, e := ParseDungeon(id, s)
		if e != nil {
			skipped = append(skipped, fmt.Sprintf("dungeon %d: %v", id, e))
			continue
		}
		out.Dungeons[id] = d
		importMap := func(mapID uint32) {
			if _, ok := out.Maps[mapID]; ok {
				return
			}
			name, ok := indices[1][mapID]
			if !ok {
				skipped = append(skipped, fmt.Sprintf("map %d (dungeon %d) not in list", mapID, id))
				return
			}
			s, e := ResolveScript(a, name)
			if e != nil {
				skipped = append(skipped, fmt.Sprintf("map %d: %v", mapID, e))
				return
			}
			out.Maps[mapID] = s
		}
		for _, m := range d.Mazes {
			for _, r := range m.Rooms {
				importMap(r.Map)
				// 同一节点列出的备选地图同样导入，否则运行时会引用到目录里
				// 不存在的地图（实测 dungeon 86 的 boss 房 20314..20317）。
				for _, alt := range r.Alternates {
					importMap(alt)
				}
			}
			for _, layer := range m.Layers {
				for _, id := range layer.Maps {
					importMap(id)
				}
			}
		}
	}
	out.Skipped = skipped
	return out, nil
}
func LoadDungeons(path string) (DungeonCatalog, error) {
	var c DungeonCatalog
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	if len(c.Source.Checksum) != 64 || len(c.Dungeons) == 0 {
		return c, fmt.Errorf("invalid dungeon catalog")
	}
	for id, d := range c.Dungeons {
		if id != d.ID {
			return c, fmt.Errorf("dungeon key mismatch")
		}
		parsed, e := ParseDungeon(id, d.Script)
		if e != nil {
			return c, e
		}
		for i := range parsed.Mazes {
			for _, layer := range parsed.Mazes[i].Layers {
				for _, mapID := range layer.Maps {
					if _, ok := c.Maps[mapID]; !ok {
						parsed.Mazes[i].Pending = append(parsed.Mazes[i].Pending, fmt.Sprintf("layer map %d not imported", mapID))
					}
				}
			}
		}
		c.Dungeons[id] = parsed
	}
	seenRoutes := map[string]bool{}
	for _, route := range c.SceneRoutes {
		key := fmt.Sprintf("%d/%d/%d/%x", route.Dungeon, route.Maze, route.From, route.Record)
		d, ok := c.Dungeons[route.Dungeon]
		if !ok || !d.Odyssey || seenRoutes[key] || route.Source != c.Source.Checksum || route.DungeonSHA256 != d.Script.SHA256 || route.MapSHA256 == "" || route.MapSHA256 != c.Maps[route.From].SHA256 {
			return c, fmt.Errorf("invalid or duplicate scene route %v", key)
		}
		seenRoutes[key] = true
		if _, ok := c.Maps[route.To]; !ok {
			return c, fmt.Errorf("scene target map missing %d", route.To)
		}
		if !SceneRouteInMaze(d, route) {
			return c, fmt.Errorf("scene route differs from source maze %v", key)
		}
	}
	// Catalogs without scene routes remain compatible with earlier releases.
	// Once enabled, refuse partial exports at startup instead of mid-dungeon.
	if len(c.SceneRoutes) > 0 {
		for _, d := range c.Dungeons {
			if !d.Odyssey {
				continue
			}
			for _, m := range d.Mazes {
				for _, layer := range m.Layers {
					var previous uint32
					for _, room := range m.Rooms {
						if [2]byte{room.X, room.Y} == layer.Position {
							previous = room.Map
						}
					}
					for _, next := range layer.Maps {
						found := false
						for _, r := range c.SceneRoutes {
							if r.Dungeon == d.ID && r.Maze == m.Index && r.Position == layer.Position && r.From == previous && r.To == next {
								found = true
								break
							}
						}
						if !found {
							return c, fmt.Errorf("missing scene route dungeon%d map%d->%d", d.ID, previous, next)
						}
						previous = next
					}
				}
			}
		}
	}
	return c, nil
}

// MergeDungeonCatalog adds a small source-matched import without rewriting the
// full catalog. Existing dungeon definitions are never replaced.
func MergeDungeonCatalog(dst *DungeonCatalog, overlay DungeonCatalog) error {
	if dst == nil || dst.Source.Checksum == "" || dst.Source.Checksum != overlay.Source.Checksum {
		return fmt.Errorf("dungeon overlay source checksum mismatch")
	}
	for id := range overlay.Dungeons {
		if _, exists := dst.Dungeons[id]; exists {
			return fmt.Errorf("dungeon overlay duplicates %d", id)
		}
	}
	for id, script := range overlay.Maps {
		if existing, exists := dst.Maps[id]; exists && existing.SHA256 != script.SHA256 {
			return fmt.Errorf("dungeon overlay changes map %d", id)
		}
	}
	for id, d := range overlay.Dungeons {
		dst.Dungeons[id] = d
	}
	for id, script := range overlay.Maps {
		dst.Maps[id] = script
	}
	return nil
}

func SceneRouteInMaze(d DungeonDefinition, r DungeonSceneRoute) bool {
	for _, maze := range d.Mazes {
		if maze.Index != r.Maze {
			continue
		}
		for _, layer := range maze.Layers {
			if layer.Position != r.Position {
				continue
			}
			var previous uint32
			for _, room := range maze.Rooms {
				if [2]byte{room.X, room.Y} == r.Position {
					previous = room.Map
				}
			}
			for _, id := range layer.Maps {
				if previous == r.From && id == r.To {
					return true
				}
				previous = id
			}
		}
	}
	return false
}
