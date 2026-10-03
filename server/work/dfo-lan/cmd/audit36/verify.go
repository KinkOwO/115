package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/managementdata"
	"dfolan/internal/quest"
	"fmt"
)

// verifyStartup loads every catalog and policy the 36 launch profile passes to
// wireprobe, through the very same loaders the server uses. A mismatch here is
// a startup failure the player would otherwise meet as a dead launcher.
func verifyStartup(native *gamedata.Source, quests catalog.QuestCatalog) int {
	failures := 0
	step := func(name string, e error) {
		if e != nil {
			failures++
			fmt.Printf("  FAIL %-34s %v\n", name, e)
			return
		}
		fmt.Printf("  ok   %s\n", name)
	}
	fmt.Printf("\n== 36 startup profile\n")
	chars, e := managementdata.Characters(native, "configs/pvf-character-policy.json")
	step("native PVF characters + policy", e)
	if e != nil {
		return failures
	}
	sourceChecksum := chars.Source.Checksum

	routes, e := catalog.LoadTutorialRoutes("configs/tutorial-routes.current35.json", sourceChecksum)
	step("tutorial-routes.current35.json", e)
	if e == nil {
		normal := 0
		for _, f := range routes.Flows {
			if !f.EventOnly {
				normal++
			}
		}
		fmt.Printf("       ordinary job routes=%d\n", normal)
	}
	td, e := catalog.LoadDungeons("configs/tutorial-dungeons.current36.json")
	step("tutorial-dungeons.current36.json", e)
	if e == nil && td.Source.Checksum != sourceChecksum {
		step("tutorial dungeon source match", fmt.Errorf("checksum differs"))
	}
	rules, e := loot.LoadRules("configs/drop.current36.json")
	step("drop.current36.json", e)
	if e == nil {
		fmt.Printf("       kinds=%v\n", rules.SupportedKinds)
	}
	index, e := native.ItemIndex("")
	if e != nil {
		step("native equipment item index", e)
	}
	policy, pe := inventory.ReadDropPolicy("configs/pvf-drop-policy.json")
	if pe != nil {
		step("PVF drop policy", pe)
	}
	var gear *inventory.EquipmentCatalog
	if e == nil && pe == nil {
		gear, e = native.EquipmentSelection(index, catalog.QuestCatalog{Source: native.Snapshot(), Quests: map[uint32]catalog.QuestDefinition{}}, policy)
	}
	step("native equipment selection + policy", e)
	if e == nil {
		fmt.Printf("       bag-usable drop pool=%d\n", len(gear.DropPool()))
	}

	step("native PVF quests", nil)
	{
		x := quest.BuildIndex(quests)
		settleable, positional := 0, 0
		for _, en := range x.Entries {
			if en.Implemented && en.RewardUsable && en.GrowUsable {
				settleable++
			}
		}
		positional = len(x.Positional)
		fmt.Printf("       settleable quests=%d positional objectives=%d clear-map maps=%d\n",
			settleable, positional, len(x.ByClearMap))
		// The low-level main chain must be settleable end to end.
		chain := []uint32{3145, 4873, 3146, 3147, 3148, 3149, 3150, 3151, 3154, 3155, 3156, 3160, 3168}
		var broken []uint32
		for _, id := range chain {
			en := x.Entries[id]
			if en == nil || !en.Implemented || !en.RewardUsable || !en.GrowUsable {
				broken = append(broken, id)
			}
		}
		if len(broken) > 0 {
			failures++
			fmt.Printf("  FAIL main chain still broken at %v\n", broken)
		} else {
			fmt.Printf("  ok   main chain 3145..3168 settleable end to end\n")
		}
	}
	fmt.Printf("  failures=%d\n", failures)
	return failures
}
