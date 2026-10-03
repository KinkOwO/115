package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
)

const towerGriefTopLayer = 100

type TowerGriefLayer struct {
	Floor         uint32 `json:"floor"`
	Dungeon       uint32 `json:"dungeon"`
	DungeonSHA256 string `json:"dungeon_sha256"`
	Map           uint32 `json:"map"`
	RewardRule    string `json:"reward_rule"`
}

type TowerGriefOverlay struct {
	SourceChecksum string                  `json:"source_checksum"`
	DailyEntries   uint16                  `json:"daily_entries"`
	Layers         []TowerGriefLayer       `json:"layers"`
	Maps           map[uint32]ScriptRecord `json:"maps"`
}

// ImportTowerGriefOverlay follows the PVF's explicit floor-to-dungeon table and
// each MAP's [dungeon] owner. The 100 tower DGN mazes do not have ordinary
// [map specification] cells; their maps therefore cannot be imported by the
// ordinary room collector.
func ImportTowerGriefOverlay(a *pvf.Archive) (TowerGriefOverlay, error) {
	var out TowerGriefOverlay
	if a == nil {
		return out, fmt.Errorf("nil tower source archive")
	}
	out.SourceChecksum = a.Snapshot().Checksum
	rules, err := ReadScript(a, "etc/towerofgrief.etc")
	if err != nil {
		return out, err
	}
	top := sectionCells(rules.Cells, "[top layer]")
	if len(top) != 1 || top[0].Type != 0 || top[0].Value != towerGriefTopLayer {
		return out, fmt.Errorf("unexpected Tower of Grief top layer")
	}
	entries := sectionCells(rules.Cells, "[account enterable max count]")
	if len(entries) != 1 || entries[0].Type != 0 || entries[0].Value <= 0 || entries[0].Value > 100 {
		return out, fmt.Errorf("invalid Tower of Grief account entry limit")
	}
	out.DailyEntries = uint16(entries[0].Value)
	rewardRows := sectionCells(rules.Cells, "[each layer dungeon clear reward]")
	if len(rewardRows)%3 != 0 || len(rewardRows) == 0 {
		return out, fmt.Errorf("invalid Tower of Grief reward rules")
	}
	rewardRules := make([]string, towerGriefTopLayer+1)
	last := int32(0)
	for i := 0; i < len(rewardRows); i += 3 {
		start, end, name := rewardRows[i], rewardRows[i+1], rewardRows[i+2]
		if start.Type != 0 || end.Type != 0 || name.Type != 6 || start.Value != last+1 || end.Value < start.Value || end.Value > towerGriefTopLayer ||
			(name.Text != "tower_grief_reward_normal" && name.Text != "tower_grief_reward_special") {
			return out, fmt.Errorf("invalid Tower of Grief reward interval")
		}
		for floor := start.Value; floor <= end.Value; floor++ {
			rewardRules[floor] = name.Text
		}
		last = end.Value
	}
	if last != towerGriefTopLayer {
		return out, fmt.Errorf("incomplete Tower of Grief reward rules")
	}
	rows := sectionCells(rules.Cells, "[each layer matching dungeon]")
	if len(rows) != towerGriefTopLayer*3 {
		return out, fmt.Errorf("Tower of Grief layer table has %d cells", len(rows))
	}
	dungeonIndex, err := ReadScript(a, "list/dungeon.lst")
	if err != nil {
		return out, err
	}
	dungeons, err := ParseIndex(dungeonIndex.Cells)
	if err != nil {
		return out, err
	}
	dungeonPaths := make(map[uint32]string, len(dungeons))
	for _, row := range dungeons {
		dungeonPaths[row.ID] = row.Path
	}
	for i := 0; i < len(rows); i += 3 {
		floor := uint32(i/3 + 1)
		if rows[i].Type != 0 || rows[i].Value != int32(floor) || rows[i+1].Type != 0 || rows[i+1].Value <= 0 || rows[i+2].Type != 0 || rows[i+2].Value != -1 {
			return out, fmt.Errorf("unsupported Tower of Grief layer row %d", floor)
		}
		id := uint32(rows[i+1].Value)
		path := dungeonPaths[id]
		if !strings.HasPrefix(path, "dungeon/towers/grief/") {
			return out, fmt.Errorf("Tower of Grief floor %d has unexpected dungeon %d", floor, id)
		}
		script, err := ResolveScript(a, path)
		if err != nil {
			return out, err
		}
		out.Layers = append(out.Layers, TowerGriefLayer{Floor: floor, Dungeon: id, DungeonSHA256: script.SHA256, RewardRule: rewardRules[floor]})
	}
	mapIndex, err := ReadScript(a, "list/map.lst")
	if err != nil {
		return out, err
	}
	maps, err := ParseIndex(mapIndex.Cells)
	if err != nil {
		return out, err
	}
	out.Maps = make(map[uint32]ScriptRecord, towerGriefTopLayer)
	floorByDungeon := make(map[uint32]int, towerGriefTopLayer)
	for i, layer := range out.Layers {
		if _, exists := floorByDungeon[layer.Dungeon]; exists {
			return out, fmt.Errorf("duplicate Tower of Grief dungeon %d", layer.Dungeon)
		}
		floorByDungeon[layer.Dungeon] = i
	}
	for _, row := range maps {
		if !strings.HasPrefix(row.Path, "map/towerofgrief_down/") && !strings.HasPrefix(row.Path, "map/towerofgrief_up/") {
			continue
		}
		script, err := ResolveScript(a, row.Path)
		if err != nil {
			return out, err
		}
		owner := sectionCells(script.Cells, "[dungeon]")
		if len(owner) != 1 || owner[0].Type != 0 || owner[0].Value <= 0 {
			return out, fmt.Errorf("Tower of Grief map %d has invalid owner", row.ID)
		}
		index, ok := floorByDungeon[uint32(owner[0].Value)]
		if !ok || out.Layers[index].Map != 0 {
			return out, fmt.Errorf("Tower of Grief map %d has unmatched or duplicate owner", row.ID)
		}
		out.Layers[index].Map = row.ID
		out.Maps[row.ID] = script
	}
	if len(out.Maps) != towerGriefTopLayer {
		return out, fmt.Errorf("Tower of Grief maps: got %d, want %d", len(out.Maps), towerGriefTopLayer)
	}
	for _, layer := range out.Layers {
		if layer.Map == 0 {
			return out, fmt.Errorf("Tower of Grief floor %d has no map", layer.Floor)
		}
	}
	return out, nil
}

// ApplyTowerGriefMaps validates and attaches the prepared source table.
func ApplyTowerGriefMaps(c *DungeonCatalog, overlay TowerGriefOverlay) error {
	if c == nil {
		return fmt.Errorf("nil dungeon catalog")
	}
	if overlay.SourceChecksum != c.Source.Checksum || overlay.DailyEntries == 0 || len(overlay.Layers) != towerGriefTopLayer || len(overlay.Maps) != towerGriefTopLayer {
		return fmt.Errorf("Tower of Grief overlay source mismatch or incomplete")
	}
	resolved := make(map[uint32]DungeonDefinition, towerGriefTopLayer)
	seenMaps := make(map[uint32]bool, towerGriefTopLayer)
	for i, layer := range overlay.Layers {
		if layer.Floor != uint32(i+1) || layer.Map == 0 || len(layer.DungeonSHA256) != 64 ||
			(layer.RewardRule != "tower_grief_reward_normal" && layer.RewardRule != "tower_grief_reward_special") {
			return fmt.Errorf("Tower of Grief layer %d is incomplete", i+1)
		}
		d, ok := c.Dungeons[layer.Dungeon]
		if !ok || !strings.HasPrefix(d.Script.Path, "dungeon/towers/grief/") || d.Script.SHA256 != layer.DungeonSHA256 || len(d.Mazes) != 1 || seenMaps[layer.Map] {
			return fmt.Errorf("Tower of Grief layer %d dungeon or map identity mismatch", layer.Floor)
		}
		m := &d.Mazes[0]
		if m.Quest != 0 || m.Size != [2]byte{1, 1} || m.Start != [2]byte{} || m.Boss != [2]byte{} || len(m.Rooms) != 0 || len(m.Pending) != 1 || m.Pending[0] != "unsupported room specification" {
			return fmt.Errorf("Tower of Grief layer %d has unexpected maze", layer.Floor)
		}
		script, ok := overlay.Maps[layer.Map]
		owner := sectionCells(script.Cells, "[dungeon]")
		if !ok || len(script.SHA256) != 64 || (!strings.HasPrefix(script.Path, "map/towerofgrief_down/") && !strings.HasPrefix(script.Path, "map/towerofgrief_up/")) || len(owner) != 1 || owner[0].Type != 0 || owner[0].Value != int32(layer.Dungeon) {
			return fmt.Errorf("Tower of Grief layer %d map source mismatch", layer.Floor)
		}
		if existing, ok := c.Maps[layer.Map]; ok && existing.SHA256 != script.SHA256 {
			return fmt.Errorf("Tower of Grief map %d conflicts with base catalog", layer.Map)
		}
		m.Rooms = []DungeonRoom{{X: 0, Y: 0, Map: layer.Map, Boss: true}}
		m.Pending = nil
		d.TowerGriefFloor = uint16(layer.Floor)
		d.Tower = &TowerRuntime{Key: "grief", Floor: uint16(layer.Floor), TopFloor: towerGriefTopLayer, DailyEntries: overlay.DailyEntries, ResetHourUTC: 9, RewardRule: layer.RewardRule}
		resolved[layer.Dungeon] = d
		seenMaps[layer.Map] = true
	}
	if len(resolved) != towerGriefTopLayer {
		return fmt.Errorf("duplicate Tower of Grief dungeon in overlay")
	}
	for id, d := range resolved {
		c.Dungeons[id] = d
	}
	for id, script := range overlay.Maps {
		c.Maps[id] = script
	}
	return nil
}

// TowerGriefFloors returns only the source-verified runtime layer mapping.
// Index zero is unused; callers must not infer dungeon IDs from floor numbers.
func (c *DungeonCatalog) TowerGriefFloors() ([101]uint32, error) {
	var floors [101]uint32
	if c == nil {
		return floors, fmt.Errorf("nil dungeon catalog")
	}
	for id, d := range c.Dungeons {
		floor := d.TowerGriefFloor
		if floor == 0 {
			continue
		}
		if floor > towerGriefTopLayer || floors[floor] != 0 {
			return floors, fmt.Errorf("invalid or duplicate Tower of Grief floor %d", floor)
		}
		floors[floor] = id
	}
	for floor := 1; floor <= towerGriefTopLayer; floor++ {
		if floors[floor] == 0 {
			return floors, fmt.Errorf("missing Tower of Grief floor %d", floor)
		}
	}
	return floors, nil
}

// TowerFloors builds a verified in-memory lookup for any attached tower.
func (c *DungeonCatalog) TowerFloors(key string, top uint16) ([]uint32, error) {
	if c == nil || key == "" || top == 0 {
		return nil, fmt.Errorf("invalid tower floor request")
	}
	floors := make([]uint32, int(top)+1)
	for id, d := range c.Dungeons {
		if d.Tower == nil || d.Tower.Key != key {
			continue
		}
		if d.Tower.TopFloor != top || d.Tower.Floor == 0 || d.Tower.Floor > top || floors[d.Tower.Floor] != 0 {
			return nil, fmt.Errorf("invalid or duplicate %s tower floor", key)
		}
		floors[d.Tower.Floor] = id
	}
	for floor := 1; floor <= int(top); floor++ {
		if floors[floor] == 0 {
			return nil, fmt.Errorf("missing %s tower floor %d", key, floor)
		}
	}
	return floors, nil
}
