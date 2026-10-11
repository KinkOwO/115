package main

import (
	"context"
	"dfolan/internal/adventure"
	"dfolan/internal/cashshop"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestAuditAllToolsAndContracts(t *testing.T) {
	if os.Getenv("BUG_AUDIT_PVF") == "" {
		t.Skip("explicit audit PVF required")
	}
	src, e := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: os.Getenv("BUG_AUDIT_PVF")})
	if e != nil {
		t.Fatal(e)
	}
	defer src.Close()
	archive, e := pvf.OpenReadOnly(pvf.Options{Path: os.Getenv("BUG_AUDIT_PVF"), MaxBytes: 1 << 30}, src.Snapshot().Checksum)
	if e != nil {
		t.Fatal(e)
	}
	defer archive.Close()
	joint, e := src.ItemCatalogs(catalog.ItemBasicOptions{Boosters: true}, true, "../../configs/pvf-enhancement-policy.json", false)
	if e != nil {
		t.Fatal(e)
	}
	index := joint.Basics.Index
	contractRules, e := cashshop.ImportContractRules(archive, &index)
	if e != nil {
		t.Fatal(e)
	}
	restoreContracts, e := cashshop.InstallContractRules(contractRules)
	if e != nil {
		t.Fatal(e)
	}
	defer restoreContracts()
	full, e := src.Equipment(index)
	if e != nil {
		t.Fatal(e)
	}
	eq := &inventory.EquipmentCatalog{Source: src.Snapshot(), Full: full}
	professions, e := src.Characters("")
	if e != nil {
		t.Fatal(e)
	}
	wear := &inventory.WearService{Catalog: eq, Professions: professions}
	e = joint.Enhancements.Activate()
	if e != nil {
		t.Fatal(e)
	}
	names := map[uint32]catalog.ItemDisplay{}
	subset := index
	subset.Items = map[uint32]catalog.ItemIndexEntry{}
	for id, it := range index.Items {
		if it.Kind == "stackable" {
			subset.Items[id] = it
		}
	}
	e = src.VisitItemDisplay(subset, func(d catalog.ItemDisplay) error { names[d.ID] = d; return nil })
	if e != nil {
		t.Fatal(e)
	}
	ids := []uint32{}
	for id := range index.Items {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	gears := map[int32]uint32{}
	for _, id := range ids {
		it := index.Items[id]
		if it.Kind != "equipment" || !strings.Contains(it.Path, "weapon") {
			continue
		}
		d, e := full.Definition(id)
		if e != nil {
			continue
		}
		lev := d.Fields["[minimum level]"]
		rar := d.Fields["[rarity]"]
		if len(lev) != 1 || len(rar) != 1 || lev[0].Value > 115 || lev[0].Value < 1 {
			continue
		}
		if _, e = eq.Basic(id); e != nil {
			continue
		}
		if _, ok := gears[lev[0].Value]; !ok {
			gears[lev[0].Value] = id
		}
	}
	results := map[string]any{"source": src.Snapshot().Checksum, "expiry_fixture": "fresh owned instance; historical source expiry is recorded separately", "scope": "all native portable templates and all stackables named contract plus every mapped contract"}
	tools := []map[string]any{}
	toolCounts := map[string]int{}
	for _, id := range ids {
		rule, ok := joint.Enhancements.Portable[id]
		if !ok {
			continue
		}
		dsp := names[id]
		action := rule.Fields["[action type]"]
		mode := byte(0)
		if action[0].Text == "[portable amplify]" {
			mode = 1
		}
		minimum, maximum := int32(1), int32(115)
		if v := rule.Fields["[check usable itemlevel]"]; len(v) == 2 {
			minimum, maximum = v[0].Value, v[1].Value
		}
		target := uint32(0)
		if list := rule.Fields["[reinforcement usable item list]"]; len(list) > 0 {
			for _, it := range list {
				if it.Type == 0 && full.HasDefinition(uint32(it.Value)) {
					target = uint32(it.Value)
					break
				}
			}
		} else {
			for level := maximum; level >= minimum; level-- {
				if gears[level] != 0 {
					target = gears[level]
					break
				}
			}
		}
		row := map[string]any{"id": id, "name": dsp.Name, "path": rule.Path, "mode": mode, "target": target, "fields": rule.Fields}
		status := "no_matching_native_equipment"
		if target != 0 {
			d, e := full.Definition(target)
			if e != nil {
				t.Fatal(e)
			}
			prof := byte(0)
			for _, job := range rule.Fields["[usable job]"] {
				if job.Type == 6 && job.Text != "[all]" {
					for p, c := range professions.Professions {
						if c.Job == job.Text {
							prof = p
							break
						}
					}
					break
				}
			}
			gear := inventory.BagEquipment{Slot: 12, Template: target}
			record := inventory.EquipmentRow(gear)
			record[10] = byte(action[1].Value)
			if mode == 1 {
				record[19] = 1
				record[20] = 12
			}
			gear.Record = record[:]
			feeItems := []inventory.BagItem{{Slot: 78, Template: id, Amount: 2}}
			feeCells := rule.Fields["[need material]"]
			for n := 0; n+1 < len(feeCells); n += 2 {
				feeItems = append(feeItems, inventory.BagItem{Slot: uint16(90 + n/2), Template: uint32(feeCells[n].Value), Amount: uint32(feeCells[n+1].Value)})
			}
			raw, e := inventory.SaveBag(json.RawMessage(`{"level":115,"audit_preserve":42}`), inventory.Bag{Version: "ordinary-bag-v1", Gold: 0, Equipment: []inventory.BagEquipment{gear}, Items: feeItems})
			if e != nil {
				t.Fatal(e)
			}
			role := inventory.Role{Profession: prof, ConfigVersion: src.Snapshot().SaveIdentity(), State: raw}
			req := protocol.ReinforcementRequest{Mode: mode, EquipmentSlot: 12, EquipmentTemplate: target, TicketSlot: 78, MaterialSlot: 0xffff, ProtectionSlot: 0xffff}
			var next json.RawMessage
			var spent, gold uint32
			if mode == 0 {
				var r inventory.GoldReinforcementReceipt
				next, _, r, e = wear.ApplyGoldReinforcement(role, nil, fmt.Sprintf("audit-tool:%d", id), req)
				spent, gold = r.MaterialSpent, r.GoldSpent
				row["receipt"] = r
			} else {
				var r inventory.AmplifyUpgradeReceipt
				next, r, e = wear.ApplyAmplifyUpgrade(role, req)
				spent, gold = r.MaterialSpent, r.GoldSpent
				row["receipt"] = r
			}
			if e != nil {
				status = "refused"
				row["reason"] = e.Error()
				if next != nil {
					t.Errorf("refused tool %d returned mutated state", id)
				}
			} else {
				status = "executed"
				b, e := inventory.ReadBag(next)
				if e != nil {
					t.Fatal(e)
				}
				if spent != 1 || gold != 0 || b.Gold != 0 || len(b.Items) != 1 || b.Items[0].Amount != 1 || !strings.Contains(string(next), `"audit_preserve":42`) {
					status = "incorrect_mutation"
					t.Errorf("tool %d incorrect consume/spend", id)
				}
			}
			row["equipment_definition"] = d.Path
		}
		row["status"] = status
		tools = append(tools, row)
		toolCounts[status]++
	}
	results["tools"] = tools
	results["tool_counts"] = toolCounts
	t.Logf("ALL portable tools=%d counts=%v", len(tools), toolCounts)
	season, e := src.Season(index)
	if e != nil {
		t.Fatal(e)
	}
	restore, e := adventure.InstallSeasonRules(season)
	if e != nil {
		t.Fatal(e)
	}
	defer restore()
	_, db, fixture := awakeningSQLiteFixture(t, 4)
	ctx := context.Background()
	if e = db.MigratePremiums(ctx); e != nil {
		t.Fatal(e)
	}
	nativeRole, e := db.CreateCharacter(ctx, database.Character{AccountID: fixture.AccountID, Name: "NativeContracts", ConfigVersion: src.Snapshot().SaveIdentity(), State: json.RawMessage(`{"level":115}`), Request: []byte{0}}, 24)
	if e != nil {
		t.Fatal(e)
	}
	rules, e := inventory.LoadBagRules("../../configs/inventory.compat90.json", src.Snapshot().Checksum)
	if e != nil {
		t.Fatal(e)
	}
	lootItems := map[uint32]catalog.LootItem{}
	for id, it := range index.Items {
		lootItems[id] = catalog.LootItem{ID: id, Kind: it.Kind, StackableType: it.StackableType, StackLimit: it.StackLimit}
	}
	lc := catalog.LootCatalog{Source: src.Snapshot(), Items: lootItems}
	itemSvc := &inventory.ItemService{Model: "audit-consume", Catalog: lc, BagRules: rules}
	lootSvc := &loot.Service{Catalog: lc, BagRules: rules}
	boost := &catalog.BoosterCatalog{Definitions: joint.Basics.Boosters, Items: index.Items}
	selectionPolicy, e := catalog.ReadSelectionBoxPolicy("../../configs/pvf-selection-policy.json")
	if e != nil {
		t.Fatal(e)
	}
	selections, e := src.SelectionBoxes(index, selectionPolicy)
	if e != nil {
		t.Fatal(e)
	}
	contracts := []map[string]any{}
	contractCounts := map[string]int{}
	for _, id := range ids {
		d, ok := names[id]
		if !ok {
			continue
		}
		mapped, mappedOK := cashshop.ResolveContractItem(id)
		if !mappedOK && d.StackableType != "[contract]" && !strings.Contains(d.Name, "契约") && !strings.Contains(strings.ToLower(d.Name), "contract") && id != 2660239 {
			continue
		}
		raw, e := inventory.SaveBag(json.RawMessage(`{"level":115,"audit_preserve":42}`), inventory.Bag{Version: "ordinary-bag-v1", Gold: 777, Items: []inventory.BagItem{{Slot: 65, Template: id, Amount: 1}}})
		if e != nil {
			t.Fatal(e)
		}
		role, _, e := db.CommitCharacterEvent(ctx, nativeRole.AccountID, nativeRole.ID, nativeRole.ConfigVersion, fmt.Sprintf("fixture:%d", id), "audit-fixture", func(database.Character) (json.RawMessage, json.RawMessage, error) {
			return raw, json.RawMessage(`{}`), nil
		})
		if e != nil {
			t.Fatal(e)
		}
		isBox := strings.Contains(d.StackableType, "booster") || strings.Contains(d.StackableType, "package")
		if isBox {
			// Each box owns a distinct character so the identical fixture transport
			// cannot replay a prior box's request receipt from another test case.
			account, err := db.DevelopmentAccount(ctx, fmt.Sprintf("audit-box-%d", id))
			if err != nil {
				t.Fatal(err)
			}
			role, e = db.CreateCharacter(ctx, database.Character{AccountID: account, Name: fmt.Sprintf("B%d", id), ConfigVersion: src.Snapshot().SaveIdentity(), State: raw, Request: []byte{0}}, 24)
			if e != nil {
				t.Fatal(e)
			}
		}
		w := &worldSession{store: db, role: role, items: itemSvc, loot: lootSvc, selectionBoxes: selections, level: 115}
		row := map[string]any{"id": id, "name": d.Name, "path": d.Path, "kind": d.StackableType, "mapped": mappedOK, "mapping": mapped}
		status := ""
		script, err := catalog.ResolveScript(archive, d.Path)
		if err != nil {
			t.Fatal(err)
		}
		row["source_cells"] = script.Cells
		beforePremiums, e := db.ActivePremiums(ctx, role.AccountID, time.Now())
		if e != nil {
			t.Fatal(e)
		}
		oldExpiry := map[uint8]int64{}
		for _, p := range beforePremiums {
			oldExpiry[p.Type] = p.EndTime
		}
		if isBox {
			req := make([]byte, 8)
			binary.LittleEndian.PutUint16(req, 65)
			binary.LittleEndian.PutUint32(req[2:], 1)
			packets, err := w.openBoosterItem(ctx, db, nil, lootSvc, boost, odysseyWeaponChoices{}, req, req)
			if err != nil {
				status = "box_refused"
				row["reason"] = err.Error()
			} else {
				status = "box_opened_rewards_only"
				row["packets"] = len(packets)
				b, err := inventory.ReadBag(w.role.State)
				if err != nil {
					t.Fatal(err)
				}
				row["remaining_items"] = b.Items
				if string(w.role.State) == string(role.State) {
					status = "box_no_change"
				}
				for _, packet := range packets {
					if packet.Name == "selection_box_awaiting_pick" {
						status = "requires_player_selection"
					}
				}
				row["complete_contract_effect"] = "inspect remaining mapped child items; selection/random paths recorded individually"
			}
		} else {
			req := make([]byte, 16)
			binary.LittleEndian.PutUint16(req, 65)
			binary.LittleEndian.PutUint32(req[3:], id)
			binary.LittleEndian.PutUint32(req[7:], id)
			events := []map[string]any{}
			packets, err := w.useStackable(req, func(v map[string]any) { events = append(events, v) })
			row["events"] = events
			if err != nil {
				status = "refused"
				row["reason"] = err.Error()
			} else {
				status = "consumed_without_contract_activation"
				row["packets"] = len(packets)
				for _, event := range events {
					if event["kind"] == "item_contracts_activated" {
						status = "activated"
					}
				}
				if mappedOK && status != "activated" {
					t.Errorf("mapped contract %d did not activate", id)
				}
				before := string(w.role.State)
				_, err = w.useStackable(req, func(map[string]any) {})
				row["replay_same_state"] = err == nil && string(w.role.State) == before
				if mappedOK && (err != nil || string(w.role.State) != before) {
					t.Errorf("contract %d replay changed state", id)
				}
			}
		}
		afterPremiums, e := db.ActivePremiums(ctx, role.AccountID, time.Now())
		if e != nil {
			t.Fatal(e)
		}
		changes := map[uint8]int64{}
		for _, p := range afterPremiums {
			if p.EndTime != oldExpiry[p.Type] {
				if oldExpiry[p.Type] != 0 {
					changes[p.Type] = p.EndTime - oldExpiry[p.Type]
				} else {
					changes[p.Type] = p.RemainingSecond
				}
			}
		}
		row["persisted_contract_seconds_added"] = changes
		if isBox && len(changes) > 0 {
			status = "box_activated"
		}
		if mappedOK && status == "activated" && changes[mapped.Type] != mapped.DurationSecond {
			t.Errorf("contract %d incorrect durable duration %d expected %d", id, changes[mapped.Type], mapped.DurationSecond)
		}
		if status == "consumed_without_contract_activation" && d.StackableType != "[contract]" && d.StackableType != "[etc]" && d.StackableType != "[waste]" {
			status = "name_collision_not_a_contract"
		}
		if strings.Contains(d.Name, "OOPArts") && status == "consumed_without_contract_activation" {
			status = "name_collision_not_a_contract"
		}
		if id == 2660239 {
			status = "character_slot_ticket_no_server_capacity_effect"
		}
		row["status"] = status
		contracts = append(contracts, row)
		contractCounts[status]++
	}

	picks := []map[string]any{}
	if box, ok := selections.ByTemplate(50006394); ok {
		for _, cat := range box.Categories {
			nativePicks := cat.Items
			if len(nativePicks) == 0 {
				script, err := catalog.ResolveScript(archive, box.Path)
				if err != nil {
					t.Fatal(err)
				}
				active := false
				for i := 0; i < len(script.Cells); i++ {
					c := script.Cells[i]
					if c.Type == 3 {
						active = c.Text == "[stackable]"
						continue
					}
					if active && c.Type == 0 && i+1 < len(script.Cells) && script.Cells[i+1].Type == 0 {
						nativePicks = append(nativePicks, catalog.SelectionItem{Template: uint32(c.Value), Count: uint32(script.Cells[i+1].Value)})
						i++
					}
				}
			}
			for _, pick := range nativePicks {
				account, err := db.DevelopmentAccount(ctx, fmt.Sprintf("audit-selection-%d-%d-%d", cat.Category[0], cat.Category[1], pick.Template))
				if err != nil {
					t.Fatal(err)
				}
				state, err := inventory.SaveBag(json.RawMessage(`{"level":115}`), inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 65, Template: 50006394, Amount: 1}}})
				if err != nil {
					t.Fatal(err)
				}
				role, err := db.CreateCharacter(ctx, database.Character{AccountID: account, Name: fmt.Sprintf("Pick%d", pick.Template), ConfigVersion: src.Snapshot().SaveIdentity(), State: state, Request: []byte{0}}, 24)
				if err != nil {
					t.Fatal(err)
				}
				w := &worldSession{store: db, role: role, items: itemSvc, loot: lootSvc, selectionBoxes: selections, level: 115}
				req := selectionBoxRequest(65, uint16(cat.Category[0])|uint16(cat.Category[1])<<8, pick.Template)
				plan, err := w.openBoosterItem(ctx, db, nil, lootSvc, boost, odysseyWeaponChoices{}, req, req)
				row := map[string]any{"box": uint32(50006394), "picked": pick.Template, "category": cat.Category, "count": pick.Count, "packets": len(plan)}
				if err != nil {
					row["status"] = "refused"
					row["reason"] = err.Error()
				} else {
					b, err := inventory.ReadBag(w.role.State)
					if err != nil {
						t.Fatal(err)
					}
					row["remaining_items"] = b.Items
					premiums, err := db.ActivePremiums(ctx, account, time.Now())
					if err != nil {
						t.Fatal(err)
					}
					added := map[uint8]int64{}
					for _, v := range premiums {
						added[v.Type] = v.RemainingSecond
					}
					row["persisted_contract_seconds_added"] = added
					row["status"] = "opened_rewards_only"
					if len(added) > 0 {
						row["status"] = "activated"
					}
				}
				picks = append(picks, row)
			}
		}
	}
	results["selection_contract_options"] = picks
	results["contracts_and_character_ticket"] = contracts
	results["contract_counts"] = contractCounts
	t.Logf("ALL contract candidates=%d counts=%v", len(contracts), contractCounts)
	raw, _ := json.MarshalIndent(results, "", "  ")
	if e = os.WriteFile(os.Getenv("BUG_AUDIT_OUTPUT"), raw, 0600); e != nil {
		t.Fatal(e)
	}
}
