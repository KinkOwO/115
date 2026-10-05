package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// TownArea contains source facts only. The server's chosen spawn point is
// separate policy, not a claim about the official new-character tutorial.
type TownArea struct {
	Source       pvf.ArchiveSnapshot `json:"source"`
	TownID       uint32              `json:"town_id"`
	AreaID       uint32              `json:"area_id"`
	TownPath     string              `json:"town_path"`
	TownSHA256   string              `json:"town_sha256"`
	MapPath      string              `json:"map_path"`
	MapSHA256    string              `json:"map_sha256"`
	MinimumLevel uint32              `json:"minimum_level"`
	Walkable     [][4]int32          `json:"walkable_rectangles"`
}

func ImportTownArea(a *pvf.Archive, townID, areaID uint32) (TownArea, error) {
	c := TownArea{Source: a.Snapshot(), TownID: townID, AreaID: areaID}
	list, err := a.Tokens("list/town.lst")
	if err != nil {
		return c, err
	}
	if len(list)%2 != 0 {
		return c, fmt.Errorf("invalid town list")
	}
	for i := 0; i < len(list); i += 2 {
		if list[i].Type != 0 || list[i+1].Type != 6 {
			return c, fmt.Errorf("invalid town list pair")
		}
		if list[i].Value >= 0 && uint32(list[i].Value) == townID {
			c.TownPath = strings.ToLower(list[i+1].Text)
		}
	}
	if c.TownPath == "" {
		return c, fmt.Errorf("town %d absent from PVF", townID)
	}
	ts, err := a.Tokens(c.TownPath)
	if err != nil {
		return c, err
	}
	active := false
	for i, t := range ts {
		if t.Type != 3 {
			continue
		}
		if t.Text == "[area]" {
			if i+2 >= len(ts) || ts[i+1].Type != 0 || ts[i+2].Type != 6 {
				return c, fmt.Errorf("invalid town area")
			}
			active = ts[i+1].Value >= 0 && uint32(ts[i+1].Value) == areaID
			if active {
				// 源里 town 文件给的 map 引用有**两种形态**：普通城镇给的是
				// `cataclysm/town/…`（归档根在 map/ 下，要补 "map/"），而月湖城镇
				// 给的是 `contents/2025/moonlake/…`（归档根就是它自己）。逐一探测，
				// 取真正存在于归档里的那个 —— 不猜前缀，也不因源改写法而静默走偏。
				ref := strings.ToLower(strings.ReplaceAll(ts[i+2].Text, "\\", "/"))
				for _, cand := range []string{"map/" + ref, ref} {
					if _, ok := a.FindFile(cand); ok {
						c.MapPath = cand
						break
					}
				}
				if c.MapPath == "" {
					return c, fmt.Errorf("town %d area %d map %q absent from the archive", townID, areaID, ref)
				}
			}
		} else if t.Text == "[/area]" {
			active = false
		} else if active && t.Text == "[need level]" {
			if i+1 >= len(ts) || ts[i+1].Type != 0 || ts[i+1].Value < 0 {
				return c, fmt.Errorf("invalid area level")
			}
			c.MinimumLevel = uint32(ts[i+1].Value)
		}
	}
	if c.MapPath == "" {
		return c, fmt.Errorf("area %d absent from town %d", areaID, townID)
	}
	ts, err = a.Tokens(c.MapPath)
	if err != nil {
		return c, err
	}
	// 地板 = [virtual movable area]（4 值组 x y w h ×N）。[town movable area]
	// 是**门/传送区**（6 值组 = 矩形 + 门通向的城镇/区号），不是地板——把门区
	// 当地板会把出生点放到传送门上（2026-10-05 实机：维纳斯旅馆出生在门口）。
	var cells []int32
	active = false
	for _, t := range ts {
		if t.Type == 3 {
			active = t.Text == "[virtual movable area]"
			continue
		}
		if active {
			if t.Type != 0 {
				return c, fmt.Errorf("non-numeric walkable area")
			}
			cells = append(cells, t.Value)
		}
	}
	if len(cells) == 0 || len(cells)%4 != 0 {
		// 赛丽亚旅馆房间的地板定义在 [import script] 引用的
		// `Common/Gate_Seria.map` 里——该文件在本归档缺失（客户端资源缺口，
		// 与 Venus Orb.obj 同类；归档内 gate_seria 零命中）。旅馆房间布局
		// 全内容统一（同 Seria_Room 动画/NPC 布点），回退用归档内已知的
		// 同布局地板定义：monsterfighters seriagate/gate.map 的 virtual 段
		// （伊斯官服出生点 545,254 即其第一矩形中心——赛丽亚正前方）。
		shared, e := seriaRoomWalkable(a)
		if e != nil {
			return c, fmt.Errorf("map walkable rectangles not recovered")
		}
		cells = shared
	}
	for i := 0; i < len(cells); i += 4 {
		if cells[i+2] <= 0 || cells[i+3] <= 0 {
			return c, fmt.Errorf("invalid walkable rectangle")
		}
		c.Walkable = append(c.Walkable, [4]int32{cells[i], cells[i+1], cells[i+2], cells[i+3]})
	}
	for path, dst := range map[string]*string{c.TownPath: &c.TownSHA256, c.MapPath: &c.MapSHA256} {
		raw, e := a.ReadRaw(path)
		if e != nil {
			return c, e
		}
		*dst = fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	return c, nil
}

// Spawn 取本城镇的默认落点：**第一个可行走矩形的中心**。
//
// 源只说哪些矩形能走，不给"出生点"；取中心是本地策略，作用与旧的
// townPolicy.X/Y 相同，但不再需要人工挑坐标 —— 特殊频道的落点要从该频道
// **自己的城镇**直读（业主 2026-10-02）。
func (c TownArea) Spawn() (uint16, uint16) {
	if len(c.Walkable) == 0 {
		return 0, 0
	}
	r := c.Walkable[0] // [x, y, w, h]
	x, y := int64(r[0])+int64(r[2])/2, int64(r[1])+int64(r[3])/2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x > 0xffff {
		x = 0xffff
	}
	if y > 0xffff {
		y = 0xffff
	}
	return uint16(x), uint16(y)
}

func (c TownArea) Allows(level byte, x, y uint16) bool {
	if uint32(level) < c.MinimumLevel {
		return false
	}
	for _, r := range c.Walkable {
		px, py := int64(x), int64(y)
		if r[0] < 0 && x >= 0x8000 {
			px = int64(int16(x))
		}
		if r[1] < 0 && y >= 0x8000 {
			py = int64(int16(y))
		}
		if px >= int64(r[0]) && px < int64(r[0])+int64(r[2]) && py >= int64(r[1]) && py < int64(r[1])+int64(r[3]) {
			return true
		}
	}
	return false
}

// seriaRoomWalkable 返回官方赛丽亚旅馆房间的通用地板（virtual movable area）。
// Venus_Seria.map 这类旅馆地图把地板定义放在 [import script]
// `Common/Gate_Seria.map` 里，而该文件在本归档缺失（资源缺口）——房间布局
// 全内容统一（同 Seria_Room 动画/NPC 布点），用归档内已知的同布局定义
// （contents/2023/monsterfighters/map/seriagate/gate.map，伊斯官服出生点
// 545,254 即其第一矩形中心）。
func seriaRoomWalkable(a *pvf.Archive) ([]int32, error) {
	const shared = "contents/2023/monsterfighters/map/seriagate/gate.map"
	if _, ok := a.FindFile(shared); !ok {
		return nil, fmt.Errorf("shared seria-room floor %s absent", shared)
	}
	ts, err := a.Tokens(shared)
	if err != nil {
		return nil, err
	}
	var cells []int32
	mode := false
	for _, t := range ts {
		if t.Type == 3 {
			mode = t.Text == "[virtual movable area]"
			continue
		}
		if mode && t.Type == 0 {
			cells = append(cells, t.Value)
		}
	}
	if len(cells) == 0 || len(cells)%4 != 0 {
		return nil, fmt.Errorf("shared seria-room floor %s has no walkable rectangles", shared)
	}
	return cells, nil
}

func LoadTownArea(path string) (TownArea, error) {
	var c TownArea
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	if len(c.Source.Checksum) != 64 || len(c.TownSHA256) != 64 || len(c.MapSHA256) != 64 || len(c.Walkable) == 0 {
		return c, fmt.Errorf("incomplete town catalog")
	}
	return c, nil
}
