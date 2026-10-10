package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestAuditNativeReports(t *testing.T) {
	path := os.Getenv("BUG_AUDIT_PVF")
	if path == "" {
		t.Skip("explicit audit resource required")
	}
	src, e := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path})
	if e != nil {
		t.Fatal(e)
	}
	defer src.Close()
	joint, e := src.ItemCatalogs(catalog.ItemBasicOptions{}, false, "", true)
	if e != nil {
		t.Fatal(e)
	}
	index := joint.Basics.Index
	full, e := src.Equipment(index)
	if e != nil {
		t.Fatal(e)
	}
	eq := &inventory.EquipmentCatalog{Full: full, Source: src.Snapshot()}
	svc := &character.Service{Equipment: eq}
	restore, e := character.InstallFameRules(joint.Fame)
	if e != nil {
		t.Fatal(e)
	}
	defer restore()
	points, e := src.PointRules()
	if e != nil {
		t.Fatal(e)
	}
	awake, e := src.EquipmentAwakening()
	if e != nil {
		t.Fatal(e)
	}
	inventory.SetEquipmentAwakeningRules(awake)
	wear := &inventory.WearService{Catalog: eq}
	selected := []catalog.ItemDisplay{}
	oathTemplates := map[uint32]bool{}
	for _, r := range points.Oath.Rules {
		oathTemplates[r.Template] = true
	}
	subset := index
	subset.Items = map[uint32]catalog.ItemIndexEntry{}
	for id, it := range index.Items {
		if oathTemplates[id] {
			subset.Items[id] = it
		}
		if it.Kind == "equipment" && (strings.Contains(it.Path, "glove") || strings.Contains(it.Path, "wrist") || strings.Contains(it.Path, "crystal") || strings.Contains(it.Path, "earring")) {
			subset.Items[id] = it
		}
		if it.Kind == "stackable" && (strings.Contains(it.Path, "character") || strings.Contains(it.Path, "aura") || strings.Contains(it.Path, "expand")) {
			subset.Items[id] = it
		}
	}
	e = src.VisitItemDisplay(subset, func(d catalog.ItemDisplay) error {
		if oathTemplates[d.ID] || strings.Contains(d.Name, "黄金乡") || strings.Contains(d.Name, "星蕴") || strings.Contains(d.Name, "角色") || strings.Contains(d.Name, "光环") || (d.MinimumLevel == 115 && strings.Contains(d.Path, "glove")) {
			selected = append(selected, d)
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}

	t.Run("GoldenBraceletPointPacket", func(t *testing.T) {
		def, e := full.Definition(100313529)
		if e != nil {
			t.Fatal(e)
		}
		cells := def.Fields["[part set index]"]
		if len(cells) != 1 {
			t.Fatal("native set identity missing")
		}
		items := []catalog.PointItem{{Template: 100313529, PartSetIndex: cells[0].Value}}
		value, hits := points.SetPoints(items)
		if value != 265 || hits != 1 {
			t.Fatalf("native golden bracelet=%d hits=%d", value, hits)
		}
		raw, e := inventory.SaveBag(json.RawMessage(`{"level":115}`), inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{{Slot: 20, Template: 100313529}, {Slot: 44, Template: 100401599}}})
		if e != nil {
			t.Fatal(e)
		}
		w := &worldSession{items: &inventory.ItemService{Equipment: eq, Points: &points}}
		w.role.State = raw
		packets := w.oathPointPackets()
		if len(packets) != 1 || len(packets[0].Payload) != 10 {
			t.Fatal("point notification absent")
		}
		if a, b := binary.LittleEndian.Uint32(packets[0].Payload[2:]), binary.LittleEndian.Uint32(packets[0].Payload[6:]); a != 205 || b != 265 {
			t.Fatalf("native point order oath=%d set=%d", a, b)
		}
	})
	results := map[string]any{"archive": path, "sha256": src.Snapshot().Checksum, "selected": selected}
	t.Logf("source=%s indexed=%d inspected=%d selected=%d set_rows=%d oath_rows=%d", src.Snapshot().Checksum, len(index.Items), len(subset.Items), len(selected), len(points.Set.Rules), len(points.Oath.Rules))
	t.Run("SetPointAggregationGap", func(t *testing.T) {
		items := []catalog.PointItem{}
		for _, d := range selected {
			if d.Kind == "equipment" {
				def, err := full.Definition(d.ID)
				if err != nil {
					t.Fatal(err)
				}
				set := int32(-1)
				if cells := def.Fields["[part set index]"]; len(cells) == 1 {
					set = cells[0].Value
				}
				items = append(items, catalog.PointItem{Template: d.ID, PartSetIndex: set})
			}
		}
		total, hits := points.SetPoints(items)
		results["set_aggregation"] = map[string]any{"input_items": len(items), "total": total, "hits": hits, "source_rows": len(points.Set.Rules)}
		if len(points.Set.Rules) > 0 && len(items) > 0 && total == 0 && hits == 0 {
			t.Errorf("UNRESOLVED: real source has %d set rows but set aggregation returns zero for all %d inspected equipment", len(points.Set.Rules), len(items))
		}
	})
	t.Run("GoldenBraceletAndCrystalFamePoints", func(t *testing.T) {
		rows := []map[string]any{}
		for _, d := range selected {
			if d.Kind != "equipment" || !(strings.Contains(d.Name, "黄金乡") && strings.Contains(d.Path, "wrist") || oathTemplates[d.ID]) {
				continue
			}
			slot := uint16(20)
			if oathTemplates[d.ID] {
				slot = 36
			}
			raw, e := inventory.SaveBag(json.RawMessage(`{"level":115}`), inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{{Slot: slot, Template: d.ID}}})
			if e != nil {
				t.Fatal(e)
			}
			fame, e := svc.EquipmentFameBreakdown(raw)
			if e != nil {
				t.Fatal(e)
			}
			oath, ohits := points.OathPoints([]catalog.PointItem{{Template: d.ID, PartSetIndex: -1}})
			rows = append(rows, map[string]any{"id": d.ID, "name": d.Name, "rarity": d.Rarity, "slot": slot, "fame": fame, "oath": oath, "oath_hits": ohits})
		}
		results["golden_and_crystal"] = rows
		t.Logf("real equipment point probes=%d; detailed before/equipped values in native JSON", len(rows))
	})
	t.Run("EpicGloveAscension", func(t *testing.T) {
		count := 0
		failed := []string{}
		for _, d := range selected {
			if d.Kind != "equipment" || d.MinimumLevel != 115 || !strings.Contains(d.Path, "glove") {
				continue
			}
			rarity, ok := catalog.EquipmentAwakeningRarityName(int(d.Rarity))
			if !ok || rarity != "epic" {
				continue
			}
			up, ok := awake.UpgradeSource(d.ID)
			if !ok || len(up.Targets) == 0 {
				continue
			}
			for _, target := range up.Targets {
				plan, e := inventory.PlanAwakening(awake, d.ID, 115, int(d.Rarity), awake.MaxLevel, 1, target)
				if e != nil {
					failed = append(failed, fmt.Sprintf("%d -> %d: %v", d.ID, target, e))
					continue
				}
				gear := inventory.BagEquipment{Slot: 12, Template: d.ID, Record: make([]byte, protocol.CurrentItemRecordSize)}
				binary.LittleEndian.PutUint32(gear.Record[2:], d.ID)
				gear.Record[170] = byte(awake.MaxLevel)
				gear.Record[10] = 15
				gear.Record[19] = 3
				gear.Record[20] = 12
				binary.LittleEndian.PutUint32(gear.Record[14:], 510000)
				bag := inventory.Bag{Version: "ordinary-bag-v1", Gold: 100000000, Equipment: []inventory.BagEquipment{gear}}
				for i, cost := range plan.Cost.Items {
					if !cost.Gold() {
						bag.Items = append(bag.Items, inventory.BagItem{Slot: uint16(65 + i), Template: cost.Template, Amount: cost.Amount})
					}
				}
				raw, e := inventory.SaveBag(json.RawMessage(`{"level":115}`), bag)
				if e != nil {
					t.Fatal(e)
				}
				next, _, receipt, e := wear.PrepareEquipmentAwakening(inventory.Role{ConfigVersion: src.Snapshot().SaveIdentity(), State: raw}, json.RawMessage(`{}`), fmt.Sprintf("audit:%d:%d", d.ID, target), protocol.EquipmentAwakeningRequest{MaterialGroup: 1, Slot: 12, Target: target})
				if e != nil {
					failed = append(failed, fmt.Sprintf("%d -> %d: %v", d.ID, target, e))
					continue
				}
				b, e := inventory.ReadBag(next)
				if e != nil {
					t.Fatal(e)
				}
				g := b.Equipment[0]
				if !receipt.Upgraded || g.Template != target || g.Record[170] != 0 || g.Record[10] != 15 || g.Record[19] != 3 || g.Record[20] != 12 || binary.LittleEndian.Uint32(g.Record[14:]) != 510000 || binary.LittleEndian.Uint32(g.Record[2:]) != target {
					t.Errorf("ascension lost instance %d -> %d", d.ID, target)
				}
				count++
			}
		}
		results["glove_ascension"] = map[string]any{"success": count, "failures": failed}
		if count == 0 {
			t.Error("no epic glove native case exercised")
		}
		if len(failed) > 0 {
			t.Error(failed)
		}
		t.Logf("native epic glove transitions=%d failures=%d, exact source costs provided, +15/amplification/enchant preserved", count, len(failed))
	})
	t.Run("SoleEarringInheritanceFromWorn", func(t *testing.T) {
		var source, target uint32
		for id, it := range index.Items {
			if it.Kind == "equipment" && strings.Contains(it.Path, "earring") {
				if id >= 100391000 && id <= 100391999 {
					target = id
				} else {
					source = id
				}
			}
			if source != 0 && target != 0 {
				break
			}
		}
		if source == 0 || target == 0 {
			t.Fatal("native earring samples missing")
		}
		mat := inventory.BagEquipment{Slot: 25, Template: source, Record: make([]byte, protocol.CurrentItemRecordSize)}
		binary.LittleEndian.PutUint32(mat.Record[2:], source)
		mat.Record[10] = 15
		mat.Record[19] = 3
		mat.Record[20] = 12
		binary.LittleEndian.PutUint32(mat.Record[14:], 510000)
		base := inventory.BagEquipment{Slot: 9, Template: target}
		raw, e := inventory.SaveBag(json.RawMessage(`{"level":115}`), inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{mat}, Equipment: []inventory.BagEquipment{base}})
		if e != nil {
			t.Fatal(e)
		}
		next, receipt, e := wear.ApplyInherit(inventory.Role{State: raw}, []protocol.InheritEntry{{SlotA: 25, TemplateA: source, SpaceA: 3, SlotB: 9, TemplateB: target, SpaceB: 0, Const: 257}})
		if e != nil {
			t.Fatal(e)
		}
		b, e := inventory.ReadBag(next)
		if e != nil {
			t.Fatal(e)
		}
		if b.Equipment[0].Record[10] != 15 || b.Equipment[0].Record[19] != 3 || b.Equipment[0].Record[20] != 12 || binary.LittleEndian.Uint32(b.Equipment[0].Record[14:]) != 510000 || b.Worn[0].Record[10]&31 != 0 {
			t.Error("native sole target inheritance lost values")
		}
		results["sole_inherit"] = map[string]any{"source": source, "target": target, "receipts": receipt}
		t.Logf("native worn earring %d -> sole earring %d: +15 amplification/enchant retained; automatic wear popup not exercised", source, target)
	})
	raw, _ := json.MarshalIndent(results, "", "  ")
	if dest := os.Getenv("BUG_AUDIT_OUTPUT"); dest != "" {
		if e := os.WriteFile(dest, raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
