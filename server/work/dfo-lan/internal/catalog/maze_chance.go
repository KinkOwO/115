package catalog

import (
	"encoding/json"
	"fmt"
	"os"
)

// MazeChanceDungeon 是「按 [maze chance rate] 掷骰选图」白名单的一条。
//
// SourceRates 是源 DGN 自己声明的权重，原样保留只作对照；Rates 才是服务端
// 实际掷骰用的权重。两者分开，是为了让「我们对官方数值做了什么改动」永远只有
// 一个 diff 的距离 —— 而不是沉在一次性的生成脚本里。
type MazeChanceDungeon struct {
	ID            uint32   `json:"id"`
	DungeonSHA256 string   `json:"dungeon_sha256"`
	SourceRates   []uint32 `json:"source_rates"`
	Rates         []uint32 `json:"rates,omitempty"`
}

// MazeChanceOverlay 是 configs/dungeons.maze-chance-rates.json 的结构。
type MazeChanceOverlay struct {
	SourceChecksum string              `json:"source_checksum"`
	Dungeons       []MazeChanceDungeon `json:"dungeons"`
}

// ReadMazeChanceRates 按 [maze info] 段出现顺序读出每张 maze 的
// [maze chance rate] 第一个值。任何一张 maze 缺这一段就返回 ok=false ——
// 半张表不能拿来归一化，那样会静默改变其余部分的相对概率。
func ReadMazeChanceRates(script ScriptRecord) ([]uint32, bool) {
	var out []uint32
	for i := 0; i < len(script.Cells); i++ {
		if script.Cells[i].Type != 3 || script.Cells[i].Text != "[maze info]" {
			continue
		}
		start := i + 1
		i++
		for i < len(script.Cells) && !(script.Cells[i].Type == 3 && script.Cells[i].Text == "[maze info]") {
			i++
		}
		cells := sectionCells(script.Cells[start:i], "[maze chance rate]")
		i--
		if len(cells) != 1 || cells[0].Type != 0 || cells[0].Value < 0 {
			return nil, false
		}
		out = append(out, uint32(cells[0].Value))
	}
	return out, len(out) > 0
}

// AttachMazeChanceRates 装载白名单权重并装到对应副本上。
//
// 只有列在 overlay 里的副本会被启用。源里有 67 个副本声明了
// [maze chance rate]，但它们的量纲并不统一（实测同时存在合计 100、1000、
// 10000、200000、1000000 的写法，还夹杂 0 权重），所以服务端不替另外那 66 个
// 做决定 —— 加一张副本 = 往 overlay 加一行，而不是一次全局行为变更。
func AttachMazeChanceRates(c *DungeonCatalog, path string) error {
	if c == nil {
		return fmt.Errorf("nil dungeon catalog")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var overlay MazeChanceOverlay
	if err := json.Unmarshal(raw, &overlay); err != nil {
		return err
	}
	if overlay.SourceChecksum != c.Source.Checksum {
		return fmt.Errorf("maze chance overlay source mismatch")
	}
	if len(overlay.Dungeons) == 0 {
		return fmt.Errorf("maze chance overlay is empty")
	}
	for _, entry := range overlay.Dungeons {
		d, ok := c.Dungeons[entry.ID]
		if !ok {
			return fmt.Errorf("maze chance overlay names absent dungeon %d", entry.ID)
		}
		if d.Script.SHA256 != entry.DungeonSHA256 {
			return fmt.Errorf("dungeon %d changed since the overlay was generated (%s != %s)",
				entry.ID, d.Script.SHA256, entry.DungeonSHA256)
		}
		rates := entry.Rates
		if len(rates) == 0 {
			rates = entry.SourceRates
		}
		if len(rates) != len(d.Mazes) || len(d.Mazes) < 2 {
			return fmt.Errorf("dungeon %d has %d mazes but the overlay lists %d weights",
				entry.ID, len(d.Mazes), len(rates))
		}
		var total uint64
		selectable := 0
		for i, w := range rates {
			if d.Mazes[i].Index != byte(i) {
				return fmt.Errorf("dungeon %d maze %d is out of source order", entry.ID, i)
			}
			total += uint64(w)
			if w > 0 {
				selectable++
			}
		}
		// 少于两张可选 = 这个副本根本不需要掷骰，那就不该出现在白名单里；
		// 允许它会让「为什么这张图永远出不来」变成没人查得动的谜。
		if total == 0 || selectable < 2 {
			return fmt.Errorf("dungeon %d maze chance weights leave %d selectable maze(s)",
				entry.ID, selectable)
		}
		d.MazeChanceRates = append([]uint32(nil), rates...)
		c.Dungeons[entry.ID] = d
	}
	return nil
}
