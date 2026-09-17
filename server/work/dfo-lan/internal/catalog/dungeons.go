package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
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
	Rooms             []DungeonRoom `json:"rooms"`
	Pending           []string      `json:"pending,omitempty"`
}
type DungeonDefinition struct {
	ID                       uint32       `json:"id"`
	Script                   ScriptRecord `json:"script"`
	MinimumLevel, BasisLevel uint32
	Tutorial, NoFatigue      bool
	Mazes                    []DungeonMaze `json:"mazes"`
}
type DungeonCatalog struct {
	Source   pvf.ArchiveSnapshot          `json:"source"`
	Dungeons map[uint32]DungeonDefinition `json:"dungeons"`
	Maps     map[uint32]ScriptRecord      `json:"maps"`
	// 批量导入时被跳过的条目（ID 已不在 list/*.lst 里，或脚本解析失败）。
	// 只做记录，不影响目录本身。
	Skipped []string `json:"skipped,omitempty"`
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
func ParseDungeon(id uint32, s ScriptRecord) (DungeonDefinition, error) {
	d := DungeonDefinition{ID: id, Script: s}
	for _, pair := range []struct {
		name string
		dst  *uint32
	}{{"[minimum required level]", &d.MinimumLevel}, {"[basis level]", &d.BasisLevel}} {
		v := sectionCells(s.Cells, pair.name)
		if len(v) != 1 || v[0].Type != 0 || v[0].Value < 1 {
			return d, fmt.Errorf("invalid %s", pair.name)
		}
		*pair.dst = uint32(v[0].Value)
	}
	for _, c := range s.Cells {
		if c.Type == 3 {
			d.Tutorial = d.Tutorial || c.Text == "[tutorial dungeon]"
			d.NoFatigue = d.NoFatigue || c.Text == "[no fatigue]"
		}
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
			v, e := sourceFirstPair(sectionCells(c, p.name), p.absentOK)
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
			for j := 0; j < len(nodes); {
				label := nodes[j]
				weighted := label.Type == 6 && label.Text == "boss_selection_probability"
				if label.Type != 6 || (label.Text != "map" && label.Text != "boss" && !weighted) {
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
		c.Dungeons[id] = parsed
	}
	return c, nil
}
