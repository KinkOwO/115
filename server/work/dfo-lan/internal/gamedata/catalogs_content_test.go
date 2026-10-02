package gamedata

import (
	"dfolan/internal/adventure"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"os"
	"testing"
	"time"
)

func TestPVFAdventureLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete adventure source parity")
	}
	c, err := prepareCatalogsForTest(t, "adventure", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json"})
	if err != nil {
		t.Fatal(err)
	}
	restore, err := c.InstallAdventureRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	actual, err := adventure.Current()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPVFCatalog(c.AdventureRules, actual); err != nil {
		t.Fatal(err)
	}
	if actual.MaxLevel != 50 || actual.Experience[50] != 171361456285 || actual.Experience[60] != 369956531543 || len(actual.Shops) != 3 {
		t.Fatal("adventure source scope changed")
	}
	t.Log("native experience levels", len(actual.Experience), "shops", len(actual.Shops), "unique items", len(actual.Items))
}

func TestAdventureAuditProvenanceAllowanceIsNarrow(t *testing.T) {
	old, err := adventure.EmbeddedRules()
	if err != nil {
		t.Fatal(err)
	}
	direct := *old
	direct.SourceChecksum = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	if err := auditAdventureRules(old, &direct); err != nil {
		t.Fatal(err)
	}
	if old.SourceChecksum != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" {
		t.Fatal("audit rewrote embedded provenance")
	}
	unknown := *old
	unknown.SourceChecksum = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	if auditAdventureRules(&unknown, &direct) == nil {
		t.Fatal("unknown provenance accepted")
	}
	direct.Sources = map[string]string{"etc/adventurersystem/adventurersystem2018.etc": "wrong-source-hash"}
	if auditAdventureRules(old, &direct) == nil {
		t.Fatal("changed source hash suppressed")
	}
}

func TestPVFBlackPurgatoryLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for source reward scope parity")
	}
	c, err := prepareCatalogsForTest(t, "black-purgatory", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json", ContentPolicyPath: "../../configs/pvf-mine-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	boxes := testRewardBoxSource{definitions: c.Boosters, items: c.Items.Items}
	r, err := c.LoadBlackPurgatory("missing-rewards.json", boxes, func(id uint32) (catalog.LootItem, bool) { item, ok := c.Loot.Items[id]; return item, ok })
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Cards) != 5 || len(r.VIPSourceOnly) != 1 || len(r.Boss.Groups["epic"].Candidates) != 208 || len(r.Boss.Groups["mythic"].Candidates) != 35 || len(r.Boss.Groups["corrupt_product"].Candidates) != 135 || r.Boss.Rates["epic"] != 100000 || r.Boss.Rates["mythic"] != 1000 || r.Boss.Rates["corrupt_product"] != 10000 {
		t.Fatal("Black Purgatory scope/probability changed")
	}
	legacy := *r
	legacy.ClientSource = "unknown"
	if err := auditBlackPurgatory(legacy, *r); err == nil {
		t.Fatal("unknown provenance accepted")
	}
	legacy = *r
	legacy.Source = "wrong"
	if err := auditBlackPurgatory(legacy, *r); err == nil {
		t.Fatal("mixed character source accepted")
	}
	base := loot.BlackPurgatoryRewards{}
	if err := auditBlackPurgatory(base, *r); err == nil {
		t.Fatal("missing script identity accepted")
	}
}

func TestPVFClearCubeLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for source overlay parity")
	}
	c, err := prepareCatalogsForTest(t, "clear-cube", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json"})
	if err != nil {
		t.Fatal(err)
	}
	base := catalog.LootCatalog{Source: c.Items.Source, Items: map[uint32]catalog.LootItem{2: {ID: 2}}}
	next, err := c.WithClearCube(base, "missing-cube.json")
	if err != nil {
		t.Fatal(err)
	}
	if next.Items[3037].ID != 3037 || next.Items[3037].Weight != 0 || next.Items[3037].Grade != 0 || len(base.Items) != 1 {
		t.Fatal("storage projection or original catalog mutated")
	}
	base.Source.Checksum = "wrong"
	if _, err := c.WithClearCube(base, "missing-cube.json"); err == nil {
		t.Fatal("mixed source overlay accepted")
	}
}

func TestPVFMineLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for mine reward graph parity")
	}
	c, err := prepareCatalogsForTest(t, "bleeding-mine", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json", ContentPolicyPath: "../../configs/pvf-mine-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := c.LoadMine("missing-mine.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.StageBoxes) != 12 || len(r.BossBoxes) != 12 || len(r.GroupBoxes) != 3 || len(r.Boxes) != 117 || len(r.Items) != 1782 || r.Combine.Maximum != 5 || len(r.NoDrop) != 1 || r.NoDrop[0] != 10330673 {
		t.Fatal("mine source scope changed")
	}
	empty := 0
	for _, pools := range r.Boxes {
		for _, pool := range pools {
			for _, v := range pool.Candidates {
				if v.Template < 0 {
					empty++
				}
			}
		}
	}
	if empty == 0 {
		t.Fatal("signed empty faces disappeared")
	}
	t.Log("signed empty faces", empty, "combine chances", r.Combine.Chances)
}

func TestPVFOdysseyRoutesLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete journal route parity")
	}
	c, err := prepareCatalogsForTest(t, "odyssey-routes", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{})
	if err != nil {
		t.Fatal(err)
	}
	var service character.ProgressionService
	if err := c.BindOdysseyRoutes(&service); err != nil {
		t.Fatal(err)
	}
	if err := verifyPVFCatalog(c.OdysseyRoutes, service.JournalRoutes); err != nil {
		t.Fatal(err)
	}
	if len(service.JournalRoutes.Nodes) != 29 {
		t.Fatal("journal scope changed")
	}
	before := c.OdysseyRoutes.Nodes[0].Dungeons[0]
	service.JournalRoutes.Nodes[0].Dungeons[0]++
	if c.OdysseyRoutes.Nodes[0].Dungeons[0] != before {
		t.Fatal("runtime mutation reached prepared source")
	}
	t.Log("native nodes", len(c.OdysseyRoutes.Nodes), "source SHA256", c.OdysseyRoutes.SHA256)
}

func TestPVFOdysseyLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for Odyssey native parity")
	}
	c, err := prepareCatalogsForTest(t, "odyssey-growth,odyssey-chapters,odyssey-weapons,odyssey-drop,odyssey-currency", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json", ContentPolicyPath: "../../configs/pvf-mine-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	g, err := c.LoadOdysseyGrowth("missing-growth.json")
	if err != nil {
		t.Fatal(err)
	}
	ch, err := c.LoadOdysseyChapters("missing-chapters.json")
	if err != nil {
		t.Fatal(err)
	}
	drop, err := c.LoadOdysseyDrop("missing-drop.json")
	if err != nil {
		t.Fatal(err)
	}
	coins, err := c.LoadOdysseyCurrency("missing-coins.json")
	if err != nil {
		t.Fatal(err)
	}
	w, err := c.LoadOdysseyWeapons("missing-weapons.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.ClearLevels) != 50 || g.GraduateReward != 10420561 || ch.DungeonCount != 50 || len(w.Categories) != 85 {
		t.Fatal("Odyssey source scope changed")
	}
	awards, seed, err := drop.Roll(12345, 100004953)
	if err != nil || len(awards) != 0 || seed != 12345 {
		t.Fatal("chapter 2 policy changed", err)
	}
	if coins.Rates != [4]uint32{1000, 10000, 10000, 10000} || coins.Items[10418036].Weight != 0 {
		t.Fatal("coin policy/pool changed")
	}
	contains := func(id uint32) bool {
		for _, category := range w.Categories {
			for _, item := range category.Items {
				if item == id {
					return true
				}
			}
		}
		return false
	}
	if !contains(101001229) || !contains(10418036) {
		t.Fatal("weapon source rows changed")
	}
	t.Log(len(g.Items), ch.ChapterCount, len(drop.Drops), len(coins.Items), len(w.Categories))
}

func TestPVFRecommendedLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete recommended eligibility parity")
	}
	c, err := prepareCatalogsForTest(t, "adventure-recommended", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{})
	if err != nil {
		t.Fatal(err)
	}
	old, err := adventure.EmbeddedRecommendedRules()
	if err != nil {
		t.Fatal(err)
	}
	restore, err := c.InstallRecommendedRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	current, err := adventure.CurrentRecommendedRules()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPVFCatalog(c.RecommendedRules, current); err != nil {
		t.Fatal(err)
	}
	excluded := map[uint32]bool{}
	ids := map[uint32]bool{0: true, 1: true, 999999999: true}
	for id := range old.Ranges {
		ids[id] = true
	}
	for _, id := range old.Excluded {
		ids[id] = true
		excluded[id] = true
	}
	for _, id := range old.AmbiguousDungeons {
		ids[id] = true
	}
	checks := 0
	for id := range ids {
		for level := 0; level <= 255; level++ {
			bounds, found := old.Ranges[id]
			want := uint32(level) >= old.MinimumLevel && !excluded[id] && found && uint32(level) >= bounds[0] && uint32(level) <= bounds[1]
			got, err := adventure.RecommendedDungeonClear(id, byte(level))
			if err != nil || got != want {
				t.Fatalf("dungeon=%d level=%d got=%v want=%v error=%v", id, level, got, want, err)
			}
			checks++
		}
	}
	t.Log("ranges", len(current.Ranges), "excluded", len(current.Excluded), "ambiguous", len(current.AmbiguousDungeons), "unavailable worldmaps", len(current.UnavailableWorldmaps), "sources", len(current.Sources), "eligibility queries", checks)
}

func TestPVFRosterBackgroundsLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete background source parity")
	}
	c, err := prepareCatalogsForTest(t, "roster-backgrounds", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json"})
	if err != nil {
		t.Fatal(err)
	}
	old, err := character.EmbeddedRosterBackgroundTickets()
	if err != nil {
		t.Fatal(err)
	}
	restore, err := c.InstallRosterBackgrounds()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	current, err := character.CurrentRosterBackgroundTickets()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPVFCatalog(c.RosterBackgrounds, current); err != nil {
		t.Fatal(err)
	}
	if len(current.Items) != 95 || len(current.Backgrounds) != 63 {
		t.Fatal("background source scope changed")
	}
	checks := 0
	for id, ticket := range old.Items {
		actual, err := character.RosterBackgroundTicketFor(id)
		if err != nil {
			t.Fatal(err)
		}
		moments := []time.Time{time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)}
		if ticket.Until != "" {
			at, err := time.ParseInLocation("2006-01-02 15:04:05", ticket.Until, time.Local)
			if err != nil {
				t.Fatal(err)
			}
			moments = append(moments, at.Add(-time.Second), at, at.Add(time.Second))
		}
		for _, now := range moments {
			want, we := ticket.UnlockAt(now)
			got, ge := actual.UnlockAt(now)
			if got != want || (we == nil) != (ge == nil) {
				t.Fatal("background authorization boundary changed", id, now, got, want, ge, we)
			}
			checks++
		}
	}
	t.Log("native tickets", len(current.Items), "background resources", len(current.Backgrounds), "authorization checks", checks, "resource hash", current.BackgroundSHA256)
}

func TestPVFSeasonLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete season source parity")
	}
	c, err := prepareCatalogsForTest(t, "season", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json"})
	if err != nil {
		t.Fatal(err)
	}
	old, err := adventure.EmbeddedSeasonRules()
	if err != nil {
		t.Fatal(err)
	}
	restore, err := c.InstallSeasonRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	current, err := adventure.CurrentSeason()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPVFCatalog(c.SeasonRules, current); err != nil {
		t.Fatal(err)
	}
	if len(current.Levels) != 120 || len(current.Contents) != 59 || len(current.Capsules) != 40 || current.OathCost.Gold != 0 || current.OathCostKey != 16 {
		t.Fatal("season scope changed")
	}
	checks := 0
	for _, level := range old.Levels {
		for _, experience := range []uint32{level.Upper - 1, level.Upper, level.Upper + 1} {
			state := adventure.SeasonState{Experience: experience}
			if current.Level(state) != old.Level(state) || current.DisplayLevel(state) != old.DisplayLevel(state) {
				t.Fatal("season level threshold changed", experience)
			}
			checks++
		}
	}
	for _, s := range old.SpecialReward[:2] {
		at, err := time.Parse("2006-01-02 15:04:05", s)
		if err != nil {
			t.Fatal(err)
		}
		for _, when := range []time.Time{at.Add(-time.Nanosecond), at, at.Add(time.Nanosecond)} {
			state := adventure.SeasonState{Experience: old.Levels[29].Upper}
			id, ok := current.SpecialRewardAt(state, when)
			wantID, wantOK := old.SpecialRewardAt(state, when)
			if id != wantID || ok != wantOK {
				t.Fatal("season event boundary changed", when)
			}
		}
	}
	t.Log("native levels", len(current.Levels), "contents", len(current.Contents), "capsules", len(current.Capsules), "level boundaries", checks, "COS hash", current.SHA256, "CTP hash", current.CostSHA256)
}

func TestPVFSelectionBoxesLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for full bounded selection box parity")
	}
	c, err := prepareCatalogsForTest(t, "selection-boxes", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json", SelectionPolicyPath: "../../configs/pvf-selection-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	boxes, err := c.LoadSelectionBoxes("missing-selection-boxes.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(boxes.Boxes) != 2975 || len(boxes.Fixed) != 2 || len(boxes.Unparsed) != 1 || !boxes.IsFixed(10307659) || !boxes.IsFixed(490022952) || boxes.Unparsed[0] != 10358468 {
		t.Fatal("selection source range changed")
	}
	old, err := catalog.LoadSelectionBoxes("../../configs/selection-boxes-candidate.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, box := range boxes.Boxes {
		for _, cat := range box.Categories {
			if len(cat.Items) == 0 {
				continue
			}
			picks := []uint32{cat.Items[0].Template, 1}
			items, missing, checked := boxes.Resolve(box.Template, cat.Category, picks)
			a, b, ok := old.Resolve(box.Template, cat.Category, picks)
			if checked != ok {
				t.Fatal("selection category lookup changed")
			}
			if err := verifyPVFCatalog(items, a); err != nil {
				t.Fatal(err)
			}
			if err := verifyPVFCatalog(missing, b); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Log("complete category/count/recommendation/hash parity and derived Resolve lookup verified for all modeled categories")
}

func TestPVFAttunementLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native reward CTP parity")
	}
	c, err := prepareCatalogsForTest(t, "attunement", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json", ContentPolicyPath: "../../configs/pvf-content-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := c.LoadAttunement("missing.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.ApplyRebalance(loot.Rebalance{FixedTiltPercent: 25, OmenHalveIdle: true}); err != nil {
		t.Fatal(err)
	}
	b, err := c.LoadAttunement("missing.json")
	if err != nil {
		t.Fatal(err)
	}
	if diff := Compare(c.Attunement, b, 1); diff.Count != 0 {
		t.Fatal("rebalance mutated native source", diff)
	}
	if len(c.Attunement.Dungeons()) != 4 || c.Attunement.Coupons() != 5 {
		t.Fatal("reward scope changed")
	}
	t.Log(c.Attunement.Dungeons(), len(c.Attunement.Templates()), c.Attunement.Coupons())
}

func TestPVFApocalypseLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native CTP parity")
	}
	c, err := prepareCatalogsForTest(t, "apocalypse", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json"})
	if err != nil {
		t.Fatal(err)
	}
	x, err := c.LoadApocalypse("missing.json")
	if err != nil {
		t.Fatal(err)
	}
	clock, err := legion.NewApocalypseClock(x)
	if err != nil || clock.Len() != 6 || len(x.Operations) != 4 {
		t.Fatal("apocalypse clock/operations changed", err)
	}
	t.Log(x.RecordCount, len(x.Duties.Records), clock.TotalSeconds())
}
