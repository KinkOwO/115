package gamedata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"os"
	"testing"
)

// Validate the current source preparation without treating a historical
// archive's dungeon/item cardinality as the current archive's content table.
// This does not open storage, listen on a port or start a client.
func TestCurrentPVFStartupAndAsrahanBindings(t *testing.T) {
	archive := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("current native archive required")
	}
	inputs := CatalogInputs{
		VerifyBaselines:        false,
		DerivedCacheDir:        "-",
		CharacterPolicyPath:    "../../configs/pvf-character-policy.json",
		BoxPolicyPath:          "../../configs/pvf-box-policy.json",
		ItemShopPolicyPath:     "../../configs/pvf-item-shop-policy.json",
		EnhancementPolicyPath:  "../../configs/pvf-enhancement-policy.json",
		VaultPolicyPath:        "../../configs/pvf-vault-policy.json",
		DropPolicyPath:         "../../configs/pvf-drop-policy.json",
		ScenePolicyPath:        "../../configs/pvf-scene-policy.json",
		ContentPolicyPath:      "../../configs/pvf-mine-policy.json",
		ScriptWarpPolicyPath:   "../../configs/pvf-script-warp-policy.json",
		LayerRevisitPolicyPath: "../../configs/pvf-layer-revisit-policy.json",
		WearRulesPath:          "../../configs/equipment-wear.current35.json",
		CashshopRelease:        true,
	}
	c, err := prepareCatalogsForTest(t, SupportedDomains, archive, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "missing-characters.json", "missing-quests.json", "missing-progression.json", "missing-world.json", inputs)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Equipment.Close()
	defer c.Learning.Close()
	defer c.Quests.Close()
	defer c.Loot.CloseDetails()
	defer c.Dungeons.CloseMapSource()
	if c.SourceChecksum == "" || len(c.Dungeons.Dungeons) == 0 || len(c.World.Towns) == 0 {
		t.Fatal("current source did not prepare dungeon/world ownership")
	}
	for _, entry := range c.RaidEntrances {
		if entry.Asrahan == nil {
			continue
		}
		for mode, phase := range []catalog.AsrahanRaidPhase{entry.Asrahan.Normal, entry.Asrahan.Hard} {
			program, err := catalog.CompileAsrahanPhase(entry.Asrahan, phase)
			if err != nil || len(program) != len(phase.Events) {
				t.Fatal("Asrahan program/source mismatch", err)
			}
			groups, err := catalog.CompileAsrahanCombatGroups(entry.Asrahan, phase)
			if err != nil || len(groups) != len(entry.Asrahan.Groups) {
				t.Fatal("Asrahan combat/source mismatch", err)
			}
			for _, g := range groups {
				d, ok := c.Dungeons.Dungeons[g.Dungeon]
				if !ok {
					t.Fatalf("source combat dungeon unavailable: %d", g.Dungeon)
				}
				if d.MinimumLevel > 255 {
					t.Fatal("source level cannot fit native actor", d.MinimumLevel)
				}
				s, err := dungeon.Select(*c.Dungeons, protocol.DungeonSelection{ID: g.Dungeon, Difficulty: 1, Party: 65535}, byte(d.MinimumLevel), nil)
				if err != nil {
					t.Fatal("source boss arena unavailable", g.Dungeon, err)
				}
				s.Loaded = true // Simulate the already-verified dungeon loading callback only.
				row, added, err := s.AddAsrahanBoss(entry.Asrahan, mode == 1, g.Index)
				if err != nil {
					t.Fatal("source boss ownership unavailable", g.Index, err)
				}
				if added {
					if _, err := protocol.UnassignedMonsterAdd115([]protocol.UnassignedMonster115{row}); err != nil {
						t.Fatal(err)
					}
				}
				var boss uint16
				for _, actor := range s.Monsters {
					if actor.Template == g.Monster && actor.Rank == 3 && !actor.NonCombat && actor.Team != 0 {
						if boss != 0 {
							t.Fatal("duplicate source boss", g.Index)
						}
						boss = actor.Entity
					}
				}
				if boss == 0 {
					t.Fatal("source boss absent from actual room roster", g.Index)
				}
				before := len(s.Monsters)
				if _, again, err := s.AddAsrahanBoss(entry.Asrahan, mode == 1, g.Index); err != nil || again || len(s.Monsters) != before {
					t.Fatal("loading replay changed source roster", err)
				}
				if _, err := s.ConfirmDeath(uint32(boss), 7, 7); err != nil || !s.Dead[boss] {
					t.Fatal("source boss death ownership missing", err)
				}
				t.Logf("mode=%d group=%d dungeon=%d monster=%d dynamic=%t roster_owned=true", mode, g.Index, g.Dungeon, g.Monster, added)
			}
		}
		t.Logf("source=%s dungeon_count=%d normal_events=%d hard_events=%d combat_groups=%d client_started=false storage_opened=false", c.SourceChecksum, len(c.Dungeons.Dungeons), len(entry.Asrahan.Normal.Events), len(entry.Asrahan.Hard.Events), len(entry.Asrahan.Groups))
		return
	}
	t.Fatal("Asrahan entrance missing from current source preparation")
}
