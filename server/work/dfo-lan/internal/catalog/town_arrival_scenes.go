package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
)

// TownArrivalScene is a quest's explicit [move to dungeon] trigger on arrival
// in a town area. The trigger area stays a town; DungeonID names a separate
// scenario resource and must never be used as the area's classification.
type TownArrivalScene struct {
	QuestID   uint32
	Town      uint32
	Area      uint32
	DungeonID uint32
}

// TownArrivalSceneWhitelist includes only complete quest rules whose source
// town area and destination dungeon both exist in the same PVF export.
// Unresolved rules are returned for audit rather than silently whitelisted.
func TownArrivalSceneWhitelist(quests QuestCatalog, world WorldCatalog) (map[uint32]TownArrivalScene, []string) {
	listed := make(map[uint32]bool, len(world.Dungeons))
	for _, d := range world.Dungeons {
		listed[d.ID] = true
	}
	whitelist := map[uint32]TownArrivalScene{}
	var issues []string
	if quests.Source.Checksum != world.Source.Checksum {
		return whitelist, []string{"quest and world PVF checksums differ"}
	}
	for _, q := range quests.Quests {
		cells := q.Script.Cells
		for i := 0; i < len(cells); i++ {
			if cells[i].Type != 3 || cells[i].Text != "[move to dungeon]" {
				continue
			}
			end := i + 1
			for end < len(cells) && !(cells[end].Type == 3 && cells[end].Text == "[/move to dungeon]") {
				end++
			}
			block := cells[i+1 : end]
			i = end
			if !hasArrivalInTown(block) {
				continue
			}
			scene, err := parseTownArrivalScene(q.ID, block)
			if err != nil {
				issues = append(issues, err.Error())
				continue
			}
			if _, ok := world.Areas[AreaKey(scene.Town, scene.Area)]; !ok {
				issues = append(issues, fmt.Sprintf("quest %d: town area %d/%d absent from world catalog", q.ID, scene.Town, scene.Area))
				continue
			}
			if !listed[scene.DungeonID] {
				issues = append(issues, fmt.Sprintf("quest %d: dungeon %d absent from world catalog", q.ID, scene.DungeonID))
				continue
			}
			if old, exists := whitelist[scene.DungeonID]; exists {
				issues = append(issues, fmt.Sprintf("dungeon %d has multiple town arrival quests %d and %d", scene.DungeonID, old.QuestID, q.ID))
				delete(whitelist, scene.DungeonID)
				continue
			}
			whitelist[scene.DungeonID] = scene
		}
	}
	sort.Strings(issues)
	return whitelist, issues
}

func hasArrivalInTown(cells []pvf.Token) bool {
	for _, c := range cells {
		if c.Type == 6 && c.Text == "arrive in town" {
			return true
		}
	}
	return false
}

func parseTownArrivalScene(questID uint32, cells []pvf.Token) (TownArrivalScene, error) {
	scene := TownArrivalScene{QuestID: questID}
	var townFound, dungeonFound bool
	for i := 0; i < len(cells); i++ {
		if cells[i].Type != 3 {
			continue
		}
		switch cells[i].Text {
		case "[int datas]":
			if townFound || i+2 >= len(cells) || cells[i+1].Type != 0 || cells[i+2].Type != 0 || cells[i+1].Value < 0 || cells[i+2].Value < 0 {
				return scene, fmt.Errorf("quest %d: malformed town arrival coordinates", questID)
			}
			scene.Town, scene.Area = uint32(cells[i+1].Value), uint32(cells[i+2].Value)
			townFound = true
		case "[dungeon]":
			if dungeonFound || i+1 >= len(cells) || cells[i+1].Type != 0 || cells[i+1].Value <= 0 {
				return scene, fmt.Errorf("quest %d: malformed town arrival dungeon", questID)
			}
			scene.DungeonID = uint32(cells[i+1].Value)
			dungeonFound = true
		}
	}
	if !townFound || !dungeonFound {
		return scene, fmt.Errorf("quest %d: incomplete town arrival scene", questID)
	}
	return scene, nil
}
