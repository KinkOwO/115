package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

func integrationGrantCommonRobotGear(t *testing.T, f *townRobotFleet, id int, path string) {
	t.Helper()
	var design []struct {
		Template uint32 `json:"template"`
		Slot     uint16 `json:"slot"`
		Path     string `json:"path"`
		SHA      string `json:"sha256"`
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &design); err != nil {
		t.Fatal(err)
	}
	m := f.manager
	for _, row := range design {
		d, err := m.characters.Equipment.Definition(row.Template)
		if err != nil {
			t.Fatal(err)
		}
		if d.Path != row.Path || d.SHA256 != row.SHA {
			t.Fatal("selected gear design no longer matches source", row.Template)
		}
		if err = f.changeRobot(townRobotChange{ID: id, Revision: m.records[id].Revision, Action: "grant", Template: row.Template, Amount: 1}); err != nil {
			t.Fatal(id, row.Template, err)
		}
		bag, err := inventory.ReadBag(m.records[id].Role.State)
		if err != nil {
			t.Fatal(err)
		}
		var item inventory.BagEquipment
		var list byte
		found := false
		for _, v := range bag.Equipment {
			if v.Template == row.Template {
				item = v
				found = true
				break
			}
		}
		if !found {
			for k, pool := range bag.Special {
				for _, v := range pool {
					if v.Template == row.Template {
						item = v
						list = k
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}
		if !found {
			t.Fatal("granted equipment not in owned inventory", row.Template)
		}
		var replaced uint32
		for _, v := range bag.Worn {
			if v.Slot == row.Slot {
				replaced = v.Template
			}
		}
		move := protocol.ItemMoveRequest{SourceList: list, SourceSlot: item.Slot, SourceItem: item.Template, DestinationList: 3, DestinationSlot: row.Slot, DestinationItem: replaced, Count: 1, Selection: ^uint32(0)}
		if err = f.changeRobot(townRobotChange{ID: id, Revision: m.records[id].Revision, Action: "move", Move: move}); err != nil {
			t.Fatal(id, row.Template, row.Slot, err)
		}
	}
}

// Discover complete wardrobe groups directly from current source package rows.
// Only the existing shop115_cosmetics content family is selected, as in r28.
func TestIntegrationRobotWardrobePool(t *testing.T) {
	out := os.Getenv("INTEGRATION_ROBOT_WARDROBE_POOL")
	if out == "" {
		t.Skip("explicit offline source pool export")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: `G:/电脑问题/115US整合制作-20261010/resource-work/output/Script.inner.pvf`, MaxBytes: 1 << 30}, "11eb18737c3f2d2687d76669fa2f4ddefc2c943eff826520709ca13569c9147d")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	index, err := catalog.ResolveScript(a, "list/stackable.lst")
	if err != nil {
		t.Fatal(err)
	}
	refs, err := catalog.ParseIndex(index.Cells)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Path < refs[j].Path })
	sets := []map[string]any{}
	seen := map[string]bool{}
	for _, ref := range refs {
		if !strings.HasPrefix(ref.Path, "stackable/shop115_cosmetics/") {
			continue
		}
		s, err := catalog.ResolveScript(a, ref.Path)
		if err != nil {
			t.Fatal(err)
		}
		category := []int32{}
		row := 0
		for i, token := range s.Cells {
			if token.Text == "[booster select category]" {
				category = nil
				row++
				for j := i + 1; j < len(s.Cells) && j < i+3; j++ {
					if s.Cells[j].Type == 0 {
						category = append(category, s.Cells[j].Value)
					}
				}
			}
			if token.Text != "[avatar]" || len(category) != 2 {
				continue
			}
			nums := []int32{}
			valid := true
			for j := i + 1; j < len(s.Cells) && s.Cells[j].Type != 3; j++ {
				if s.Cells[j].Type != 0 {
					valid = false
				}
				nums = append(nums, s.Cells[j].Value)
			}
			if !valid || (len(nums) != 32 && len(nums) != 36) {
				continue
			}
			ids := []uint32{}
			for j := 0; j < len(nums); j += 4 {
				if nums[j] <= 0 || nums[j+1] != 1 {
					valid = false
				}
				ids = append(ids, uint32(nums[j]))
			}
			if !valid {
				continue
			}
			keyIds := append([]uint32(nil), ids...)
			sort.Slice(keyIds, func(i, j int) bool { return keyIds[i] < keyIds[j] })
			key := fmt.Sprint(keyIds)
			if seen[key] {
				continue
			}
			seen[key] = true
			sets = append(sets, map[string]any{"package": s.Path, "sha256": s.SHA256, "category": append([]int32(nil), category...), "row": row, "templates": ids})
		}
	}
	if len(sets) < 1000 {
		t.Fatal("unexpected wardrobe source coverage", len(sets))
	}
	body, err := json.MarshalIndent(map[string]any{"source": a.Snapshot(), "sets": sets}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(out, body, 0600); err != nil {
		t.Fatal(err)
	}
	t.Log("source complete wardrobe rows", len(sets))
}
