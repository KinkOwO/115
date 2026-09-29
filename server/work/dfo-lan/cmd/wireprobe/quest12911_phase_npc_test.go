package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/storage"
	"dfolan/internal/world"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestALullNativeTriggerUsesChestTownPhaseNPC(t *testing.T) {
	t.Setenv("DFO_QUEST_VISIBLE_NPC_RELAX", "0")
	qcat, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	wcat, err := catalog.LoadWorld("../../configs/world.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	d := qcat.Quests[12911]
	svc := &world.Service{Catalog: wcat}
	at := storage.WorldPosition{Town: 80, Area: 0, X: 492, Y: 185}
	const npc = 100000670
	p, err := hex.DecodeString("21006f32000000000000000000000000")
	if err != nil || len(p) != 16 || binary.LittleEndian.Uint16(p) != 33 || binary.LittleEndian.Uint16(p[2:]) != uint16(d.ID) {
		t.Fatal("live A Lull request no longer matches its source quest", err)
	}
	if d.Script.Path != "contents/2022/110levelscenario/sanctusbellum/quest/sanctusbellum_act_36.qst" ||
		d.Script.SHA256 != "2943dd54f5f6db3995f4c21b8974b4da13556698072e14325469719f19bf93e7" {
		t.Fatal("A Lull source identity changed")
	}
	if svc.HasNPC(at, npc) || allowsQuestVisibleNPCInteraction(12911, npc, at, d, qcat) ||
		allowsQuestPhaseNPCInteraction(svc, npc, at, d, qcat) {
		t.Fatal("the original phase NPC refusal is no longer reproduced")
	}
	if position, found := svc.PhaseNPCPosition(at, npc); !found || position != [2]uint16{495, 182} {
		t.Fatalf("source Hyria placement changed: %v, %v", position, found)
	}
	for _, row := range wcat.Areas["80/0"].PhaseNPCs {
		if row.ID == npc && (row.MapPath != "map/cataclysm/town/destroyed_chesttown/destroyed_chesttown_main.map" ||
			row.MapSHA256 != "f271d5b39ec0e757b1c311666e414767518cbbcd41c844d38519609b187da213") {
			t.Fatal("Hyria phase-map source changed")
		}
	}
	if !allowsALullPhaseNPCInteraction(svc, 12911, npc, at, d, qcat) {
		t.Fatal("observed native trigger should reach the accepted quest check")
	}
	for _, tc := range []struct {
		name string
		id   uint16
		npc  uint32
		at   storage.WorldPosition
	}{
		{"wrong quest", 12909, npc, at},
		{"wrong NPC", 12911, npc + 1, at},
		{"wrong town", 12911, npc, storage.WorldPosition{Town: 40, Area: 0}},
		{"wrong area", 12911, npc, storage.WorldPosition{Town: 80, Area: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if allowsALullPhaseNPCInteraction(svc, tc.id, tc.npc, tc.at, d, qcat) {
				t.Fatal("unobserved interaction was allowed")
			}
		})
	}
	for _, tc := range []struct {
		name   string
		mutate func(*catalog.QuestDefinition)
	}{
		{"wrong definition", func(q *catalog.QuestDefinition) { q.ID++ }},
		{"unresolved source", func(q *catalog.QuestDefinition) { q.Pending = []string{"unresolved"} }},
		{"wrong kind", func(q *catalog.QuestDefinition) { q.Kind = "[clear map]" }},
		{"wrong objective", func(q *catalog.QuestDefinition) { q.ObjectiveCells = []pvf.Token{{Type: 0, Value: npc + 1}} }},
		{"wrong completion NPC", func(q *catalog.QuestDefinition) {
			for i, c := range q.Script.Cells {
				if c.Text == "[complete npc index]" {
					q.Script.Cells[i+1].Value++
				}
			}
		}},
		{"wrong subtype", func(q *catalog.QuestDefinition) {
			for i, c := range q.Script.Cells {
				if c.Text == "[sub type]" {
					q.Script.Cells[i+1].Value = 1
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other := d
			other.Script.Cells = append([]pvf.Token(nil), d.Script.Cells...)
			tc.mutate(&other)
			if allowsALullPhaseNPCInteraction(svc, 12911, npc, at, other, qcat) {
				t.Fatal("unsupported source definition was allowed")
			}
		})
	}
	for _, tc := range []struct {
		name   string
		mutate func(*catalog.WorldArea)
	}{
		{"missing phase NPC", func(a *catalog.WorldArea) { a.PhaseNPCs = nil }},
		{"ambiguous phase position", func(a *catalog.WorldArea) { a.PhaseNPCs = append(a.PhaseNPCs, catalog.PhaseNPC{ID: npc, X: 1, Y: 1}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			area := wcat.Areas["80/0"]
			area.PhaseNPCs = append([]catalog.PhaseNPC(nil), area.PhaseNPCs...)
			tc.mutate(&area)
			other := wcat
			other.Areas = map[string]catalog.WorldArea{"80/0": area}
			if allowsALullPhaseNPCInteraction(&world.Service{Catalog: other}, 12911, npc, at, d, qcat) {
				t.Fatal("unproven phase placement was allowed")
			}
		})
	}
	other := wcat
	other.Source.Checksum = "different source"
	if allowsALullPhaseNPCInteraction(nil, 12911, npc, at, d, qcat) ||
		allowsALullPhaseNPCInteraction(&world.Service{Catalog: other}, 12911, npc, at, d, qcat) {
		t.Fatal("missing world service or mismatched source was allowed")
	}
}
