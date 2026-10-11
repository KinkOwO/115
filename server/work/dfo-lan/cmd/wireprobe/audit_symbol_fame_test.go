package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestAuditNativeSymbolFameProgress(t *testing.T) {
	if os.Getenv("BUG_AUDIT_PVF") == "" {
		t.Skip("explicit audit PVF required")
	}
	src, e := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: os.Getenv("BUG_AUDIT_PVF")})
	if e != nil {
		t.Fatal(e)
	}
	defer src.Close()
	layout, e := src.SymbolSocketLayout()
	if e != nil {
		t.Fatal(e)
	}
	if layout == nil || layout.Rules == nil {
		t.Fatal("symbol source absent")
	}
	index, e := src.ItemIndex("")
	if e != nil {
		t.Fatal(e)
	}
	full, e := src.Equipment(index)
	if e != nil {
		t.Fatal(e)
	}
	eq := &inventory.EquipmentCatalog{Source: src.Snapshot(), Full: full}
	svc := &character.Service{Equipment: eq, SymbolSockets: layout}
	fameRules, e := src.Fame(index)
	if e != nil {
		t.Fatal(e)
	}
	restore, e := character.InstallFameRules(fameRules)
	if e != nil {
		t.Fatal(e)
	}
	defer restore()
	raw := json.RawMessage(`{"level":115,"inventory":{"version":"ordinary-bag-v1","gold":0}}`)
	initial, e := inventory.ReadSymbolSockets(raw, layout)
	if e != nil {
		t.Fatal(e)
	}
	w := &worldSession{role: database.Character{ID: 1, State: raw}, items: &inventory.ItemService{Catalog: catalog.LootCatalog{Source: src.Snapshot()}, SymbolSockets: layout, EmblemInlay: &inventory.EmblemInlayRules{Source: layout.Source}}}
	before, e := w.symbolView(time.Now())
	if e != nil {
		t.Fatal(e)
	}
	fameBefore, e := svc.EquipmentFameBreakdown(raw)
	if e != nil {
		t.Fatal(e)
	}
	var best uint32
	var progress inventory.SymbolProgress
	for key, tier := range layout.Rules.Tiers {
		if key.Type != 0 {
			continue
		}
		sum := tier.StartFame
		var max uint32
		for level, row := range tier.Levels {
			sum += row.FameCell
			if level > max {
				max = level
			}
		}
		if sum > best {
			best = sum
			progress = inventory.SymbolProgress{Grade: key.Grade, Level: max}
		}
	}
	if best == 0 {
		t.Fatal("source fame cells absent")
	}
	initial.Progress["red_symbol"] = progress
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	fields["symbol_emblem_sockets"], _ = json.Marshal(initial)
	raw, _ = json.Marshal(fields)
	w.role.State = raw
	after, e := w.symbolView(time.Now())
	if e != nil {
		t.Fatal(e)
	}
	fameAfter, e := svc.EquipmentFameBreakdown(raw)
	if e != nil {
		t.Fatal(e)
	}
	var uiBefore, uiAfter uint32
	for _, g := range before.Groups {
		if g.Name == "red_symbol" {
			uiBefore = g.Star.Fame
		}
	}
	for _, g := range after.Groups {
		if g.Name == "red_symbol" {
			uiAfter = g.Star.Fame
		}
	}
	evidence := map[string]any{"source": src.Snapshot().Checksum, "progress": progress, "ui_fame_before": uiBefore, "ui_fame_after": uiAfter, "character_fame_before": fameBefore.Total, "character_fame_after": fameAfter.Total, "unit_conversion": "not presumed; checks contribution presence only"}
	out, _ := json.MarshalIndent(evidence, "", "  ")
	if dest := os.Getenv("BUG_AUDIT_OUTPUT"); dest != "" {
		os.WriteFile(dest, out, 0600)
	}
	t.Logf("native symbol fame UI %d -> %d; character fame %d -> %d", uiBefore, uiAfter, fameBefore.Total, fameAfter.Total)
	if uiAfter <= uiBefore {
		t.Fatal("probe did not increase source fame")
	}
	if fameAfter.Total-fameBefore.Total != uiAfter-uiBefore {
		t.Error("UNRESOLVED: independently saved symbol progress increases displayed fame but contributes nothing to character fame")
	}
}
