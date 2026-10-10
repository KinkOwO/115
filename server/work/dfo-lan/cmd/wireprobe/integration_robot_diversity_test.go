package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestIntegrationRobotDiversitySource(t *testing.T) {
	out := os.Getenv("INTEGRATION_ROBOT_DIVERSITY_AUDIT")
	if out == "" {
		t.Skip("explicit offline source audit")
	}
	a, e := pvf.OpenReadOnly(pvf.Options{Path: `G:/电脑问题/115US整合制作-20261010/resource-work/output/Script.inner.pvf`, MaxBytes: 1 << 30}, "11eb18737c3f2d2687d76669fa2f4ddefc2c943eff826520709ca13569c9147d")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	index, e := catalog.ResolveScript(a, "list/equipment.lst")
	if e != nil {
		t.Fatal(e)
	}
	refs, e := catalog.ParseIndex(index.Cells)
	if e != nil {
		t.Fatal(e)
	}
	selected := []any{}
	names := []any{}
	counts := map[string]int{}
	for _, r := range refs {
		if strings.Contains(r.Path, "/weapon/") && !strings.Contains(r.Path, "/avatar/") && !strings.Contains(r.Path, "/at_avatar/") {
			v, e := catalog.ResolveScript(a, r.Path)
			if e != nil {
				t.Fatal(e)
			}
			level, rarity := int32(-1), int32(-1)
			for i := 0; i+1 < len(v.Cells); i++ {
				if v.Cells[i].Text == "[minimum level]" {
					level = v.Cells[i+1].Value
				}
				if v.Cells[i].Text == "[rarity]" {
					rarity = v.Cells[i+1].Value
				}
			}
			if level == 115 && rarity >= 4 {
				names = append(names, r)
				selected = append(selected, map[string]any{"id": r.ID, "script": v})
			}
		}
		category := ""
		if strings.Contains(r.Path, "/creature/") {
			category = "pet:" + r.Path[:strings.LastIndex(r.Path, "/")]
		}
		if strings.Contains(r.Path, "/title/") {
			category = "title"
		}
		if strings.Contains(r.Path, "/title/") && strings.Contains(r.Path, "cn115") {
			category = "modern-title:" + r.Path[:strings.LastIndex(r.Path, "/")]
		}
		if strings.Contains(r.Path, "/aurora/") || strings.Contains(r.Path, "/aura/") {
			category = "aura"
		}
		if strings.Contains(r.Path, "dragonrobe4") {
			category = "mall-costume"
		}
		if (strings.Contains(r.Path, "/avatar/") || strings.Contains(r.Path, "/at_avatar/")) && strings.HasPrefix(r.Path, "equipment/character/") && !strings.Contains(r.Path, "dragonrobe4") && (strings.Contains(r.Path, "/swordman/") || strings.Contains(r.Path, "/fighter/") || strings.Contains(r.Path, "/gunner/") || strings.Contains(r.Path, "/mage/") || strings.Contains(r.Path, "/priest/") || strings.Contains(r.Path, "/thief/")) {
			category = "costume:" + r.Path[:strings.LastIndex(r.Path, "/")]
		}
		cap := 300
		if strings.HasPrefix(category, "pet:") {
			cap = 24
		}
		if strings.HasPrefix(category, "costume:") {
			cap = 24
		}
		if category == "aura" {
			category = "aura:" + r.Path[:strings.LastIndex(r.Path, "/")]
			cap = 24
		}
		if category == "" || counts[category] >= cap {
			continue
		}
		v, e := catalog.ResolveScript(a, r.Path)
		if e != nil {
			t.Fatal(e)
		}
		selected = append(selected, map[string]any{"id": r.ID, "script": v})
		counts[category]++
	}
	chars, e := catalog.ImportCharacters(a)
	if e != nil {
		t.Fatal(e)
	}
	packages := []catalog.ScriptRecord{}
	for _, path := range []string{"stackable/dragonrobe4/930440001.stk"} {
		v, e := catalog.ResolveScript(a, path)
		if e != nil {
			t.Fatal(e)
		}
		packages = append(packages, v)
	}
	raw, _ := json.MarshalIndent(map[string]any{"characters": chars, "packages": packages, "items": selected, "weapon_refs": names, "counts": counts}, "", "  ")
	if e = os.WriteFile(out, raw, 0600); e != nil {
		t.Fatal(e)
	}
	t.Log(counts, "weapon refs", len(names))
}
