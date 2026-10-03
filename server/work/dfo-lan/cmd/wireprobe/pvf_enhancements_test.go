package main

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"os"
	"testing"
)

func TestEnhancementAuditAllowsOnlyMissingOrdinaryTicketExpirationHeader(t *testing.T) {
	legacy, err := inventory.ReadEnhancementBaseline("../../configs")
	if err != nil {
		t.Fatal(err)
	}
	direct, err := inventory.ReadEnhancementBaseline("../../configs")
	if err != nil {
		t.Fatal(err)
	}
	const id = 10000160
	row := direct.ReinforcementTickets[id]
	row.Fields["[expiration date]"] = []pvf.Token{{Type: 6, Text: "2025-01-01 09:00:00"}}
	if err := auditPVFEnhancements(legacy, direct); err != nil {
		t.Fatal(err)
	}
	row.Fields["[expiration date]"][0].Text = "2026-01-01 09:00:00"
	baselineRow := legacy.ReinforcementTickets[id]
	if baselineRow.Fields["[expiration date]"][0].Text != "2025-01-01 09:00:00" {
		t.Fatal("audit supplementation aliased the direct source")
	}
	if err := auditPVFEnhancements(legacy, direct); err == nil {
		t.Fatal("changed existing expiration allowed")
	}
	baselineRow.Fields["[expiration date]"] = row.Fields["[expiration date]"]
	row.Fields["[unknown requirement]"] = []pvf.Token{{Type: 0, Value: 1}}
	if err := auditPVFEnhancements(legacy, direct); err == nil {
		t.Fatal("unknown source requirement allowed")
	}
}

func TestPVFEnhancementsLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for all six enhancement families")
	}
	c, err := preparePVFCoreCatalogs("enhancements", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", enhancementPolicyPath: "../../configs/pvf-enhancement-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	if err = c.loadEnhancements("missing-selected-JSON-directory"); err != nil {
		t.Fatal(err)
	}
	if card, ok := inventory.EnchantCardForBead(2600294); !ok || card != 3600 {
		t.Fatal("bead runtime index", card)
	}
	if !inventory.IsPureGrimoire(1286) {
		t.Fatal("server pure grimoire policy lost")
	}
	if count, ok := inventory.GoldMaterialCount(0); !ok || count != 10 {
		t.Fatal("reinforcement material", count)
	}
	if count, ok := inventory.AmplifyMaterialCount(9); !ok || count != 10 {
		t.Fatal("amplify material", count)
	}
	rate, ok := inventory.GoldSuccessPercent(10)
	if !ok || rate != 25 || inventory.AmplifySuccessPercent(4) != 70 {
		t.Fatal("server success policy changed")
	}
	t.Logf("six enhancement families passed complete effective parity and runtime activation")
}

func TestPVFEnhancementSourceOnlyLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for combined source-only startup")
	}
	verify := false
	c, err := preparePVFCoreCatalogs(pvfNextDomains+",enhancements", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "missing-quests.json", "missing-progression.json", "missing-world.json", pvfItemInputs{
		verifyBaselines: &verify, indexPath: "missing/items.index.json", fullPrefix: "missing/equipment-full", journalPath: "missing-journal.json", createCostPath: "missing-create-cost.json", learningPath: "missing-skills.json", pricesPath: "missing-prices.json", materialsPath: "missing-materials.json", boosterPath: "missing-boosters.json", tutorialPath: "missing-tutorial.json", enhancementPolicyPath: "../../configs/pvf-enhancement-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.equipment.Close()
	collectPVFImportMemory(c)
	if err := c.loadEnhancements("missing-selected-JSON-directory"); err != nil {
		t.Fatal(err)
	}
	if len(c.enhancements.ReinforcementTickets) != 1196 || len(c.enhancements.AmplifyTickets) != 1629 || len(c.enhancements.Grimoires.Grimoires) != 433 || len(c.enhancements.Enchant.Beads) != 4846 || len(c.enhancements.Gold.Levels) != 255 || len(c.enhancements.Amplify.Levels) != 255 {
		t.Fatal("incomplete enhancements")
	}
	for _, test := range []struct {
		level  int
		weapon bool
		want   uint32
	}{{0, false, 147750}, {0, true, 177300}, {15, true, 8155800}} {
		got, err := inventory.GoldCost(115, 4, test.level, test.weapon)
		if err != nil || got != test.want {
			t.Fatal("reinforcement live cost anchor", got, err)
		}
	}
	if gold, ok := inventory.AmplifyGold(9); !ok || gold != 50000 {
		t.Fatal("amplify cost", gold)
	}
	if _, err := c.equipment.Definition(101001153); err != nil {
		t.Fatal(err)
	}
	t.Log("15 selectors / 20 source families prepared with nonexistent selected JSON paths; character anchor and enhancement policy retained")
}
