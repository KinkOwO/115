package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/quest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestPVFLotteryLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete lottery source and weighted boundary parity")
	}
	c, err := prepareCatalogsForRuntimeTest(t, "characters,lottery", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "missing-characters.json", "", "", "", gamedata.CatalogInputs{CharacterPolicyPath: "../../configs/pvf-character-policy.json", IndexPath: "missing-index.json", LotteryPolicyPath: "../../configs/pvf-lottery-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := loadRuntimeLotteryItems(c, "missing-items.json", c.Items.Items)
	if err != nil {
		t.Fatal(err)
	}
	count, err := loadRuntimeLotteryEquipment(c, "missing-equipment.json", c.Items.Items, direct)
	if err != nil || count != 2477 {
		t.Fatal(count, err)
	}
	// The historical lottery pool JSON (configs/lottery-item-pools.json /
	// lottery-equipment-pools.json, source 7ef2) is an audit baseline only and is
	// no longer compared here. Native preparation identity and mutation isolation
	// are still asserted below; weighted boundaries are covered by
	// TestNativeLotteryContentCanChange.
	before := direct.byTemplate[7772].Candidates[0]
	direct.byTemplate[7772].Candidates[0].Count++
	again, err := loadRuntimeLotteryItems(c, "missing-items.json", c.Items.Items)
	if err != nil || again.byTemplate[7772].Candidates[0] != before {
		t.Fatal("prepared source was mutated", err)
	}
	t.Logf("native lottery pools prepared: items=%d equipment=%d; prepared source isolated from mutation", len(direct.byTemplate), count)
}

func prepareCatalogsForRuntimeTest(t *testing.T, selection, archive, checksum, characters, quests, progression, world string, provided gamedata.CatalogInputs) (*gamedata.Catalogs, error) {
	provided.Selection, provided.ArchivePath, provided.ArchiveChecksum = selection, archive, checksum
	provided.CharacterPath, provided.QuestPath, provided.ProgressionPath, provided.WorldPath = characters, quests, progression, world
	return gamedata.PrepareCatalogs(provided, runtimeCatalogAdaptersForTest(t))
}
func TestNativeLotteryContentCanChange(t *testing.T) {
	before := lotterySourcePVFSHA256
	t.Cleanup(func() { lotterySourcePVFSHA256 = before })
	source := strings.Repeat("a", 64)
	SetLotterySource(source)
	index := map[uint32]ItemIndexInfo{
		10: {ID: 10, Kind: "stackable", StackableType: "[upgradable legacy]", Path: "stackable/new.stk"},
		11: {ID: 11, Kind: "stackable", StackableType: "[upgradable legacy]", Path: "stackable/gear.stk"},
		20: {ID: 20, Kind: "stackable"}, 21: {ID: 21, Kind: "equipment", Path: "equipment/weapon/new.equ"},
	}
	tables := catalog.LotteryTables{
		Items:     catalog.LotteryPoolCatalog{SourcePVFSHA256: source, Pools: []catalog.LotterySourcePool{{SourceItem: 10, SourceScript: "stackable/new.stk", SourceScriptSHA256: strings.Repeat("b", 64), Candidates: [][3]uint32{{20, 4, 3}, {0, 6, 1234}}}}},
		Equipment: catalog.LotteryPoolCatalog{SourcePVFSHA256: source, Pools: []catalog.LotterySourcePool{{SourceItem: 11, SourceScript: "stackable/gear.stk", SourceScriptSHA256: strings.Repeat("c", 64), Candidates: [][3]uint32{{21, 9, 1}, {20, 1, 2}}}}},
	}
	c := &gamedata.Catalogs{LotteryTables: &tables}
	bound, err := loadRuntimeLotteryItems(c, "missing.json", index)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := loadRuntimeLotteryEquipment(c, "missing.json", index, bound); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if bound.SourcePVFSHA256 != source || bound.byTemplate[10].total != 10 || bound.byTemplate[11].total != 10 {
		t.Fatal("source identity or weights changed")
	}
	for _, tc := range []struct {
		id       uint32
		draw     int64
		template uint32
		count    uint32
	}{{10, 3, 20, 3}, {10, 4, 0, 1234}, {11, 8, 21, 1}, {11, 9, 20, 2}} {
		row, err := bound.byTemplate[tc.id].pick(tc.draw)
		if err != nil || row.Template != tc.template || row.Count != tc.count {
			t.Fatal(tc, row, err)
		}
	}
	bound.byTemplate[10].Candidates[0].Count++
	again, err := loadRuntimeLotteryItems(c, "missing.json", index)
	if err != nil || again.byTemplate[10].Candidates[0].Count != 3 {
		t.Fatal("prepared source mutated", err)
	}
	tables.Items.SourcePVFSHA256 = strings.Repeat("d", 64)
	if _, err := loadRuntimeLotteryItems(c, "missing.json", index); err == nil {
		t.Fatal("foreign source accepted")
	}
}

func TestNativeLotteryDiscoveryCurrentArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_SCOPE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_SCOPE_TEST_ARCHIVE for full native lottery discovery")
	}
	before := lotterySourcePVFSHA256
	t.Cleanup(func() { lotterySourcePVFSHA256 = before })
	s, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path, ExpectedChecksum: os.Getenv("DFO_PVF_SCOPE_TEST_SHA256")})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	index, err := s.ItemIndex("")
	if err != nil {
		t.Fatal(err)
	}
	direct, scope, err := s.DiscoverLottery(index)
	if err != nil {
		t.Fatal(err)
	}
	if scope.Candidates != len(direct.Items.Pools)+len(direct.Equipment.Pools)+len(scope.Issues) {
		t.Fatal("incomplete discovery")
	}
	old, err := catalog.ReadLotteryPolicy("../../configs/pvf-lottery-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := s.Lottery(index, old)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(direct, legacy) {
		t.Fatal("supported native scope differs from historical pool set")
	}
	for i, issue := range scope.Issues {
		if issue.Path != index.Items[issue.Template].Path || len(issue.SHA256) != 64 || issue.Reason == "" || i > 0 && issue.Template <= scope.Issues[i-1].Template {
			t.Fatal("untraceable or unordered issue", issue)
		}
	}
	c, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{
		Selection: "characters,lottery", ArchivePath: path, ArchiveChecksum: os.Getenv("DFO_PVF_SCOPE_TEST_SHA256"),
		CharacterPath: "missing-characters.json", CharacterPolicyPath: "../../configs/pvf-character-policy.json", LotteryPolicyPath: "missing-policy.json", IndexPath: "missing-index.json",
	}, runtimeCatalogAdaptersForTest(t))
	if err != nil {
		t.Fatal(err)
	}
	bound, err := loadRuntimeLotteryItems(c, "missing-items.json", index.Items)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadRuntimeLotteryEquipment(c, "missing-equipment.json", index.Items, bound); err != nil {
		t.Fatal(err)
	}
	for _, issue := range scope.Issues {
		if bound.byTemplate[issue.Template] != nil {
			t.Fatal("unsupported pool enabled", issue.Template)
		}
	}
	boundaries := 0
	for _, part := range []catalog.LotteryPoolCatalog{legacy.Items, legacy.Equipment} {
		for _, pool := range part.Pools {
			var offset int64
			for _, row := range pool.Candidates {
				for _, draw := range []int64{offset, offset + int64(row[1]) - 1} {
					got, err := bound.byTemplate[pool.SourceItem].pick(draw)
					if err != nil || got.Template != row[0] || got.Weight != row[1] || got.Count != row[2] {
						t.Fatal("weighted boundary changed", pool.SourceItem, draw, err)
					}
					boundaries++
				}
				offset += int64(row[1])
			}
		}
	}
	t.Logf("native candidates=%d items=%d equipment=%d unavailable=%d weighted boundaries=%d", scope.Candidates, len(direct.Items.Pools), len(direct.Equipment.Pools), len(scope.Issues), boundaries)
}

func TestNativeLotteryStartupWithoutExportedCatalogs(t *testing.T) {
	path := os.Getenv("DFO_PVF_SCOPE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_SCOPE_TEST_ARCHIVE for source preparation without exported catalogs")
	}
	// Core preparation installs several source identities. Keep this optional
	// real-archive check isolated from the historical JSON tests in this process.
	if os.Getenv("DFO_PVF_LOTTERY_STARTUP_CHILD") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestNativeLotteryStartupWithoutExportedCatalogs$", "-test.v")
		cmd.Env = append(os.Environ(), "DFO_PVF_LOTTERY_STARTUP_CHILD=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated source preparation: %v\n%s", err, output)
		}
		t.Log(string(output))
		return
	}

	c, err := prepareCatalogsForRuntimeTest(t, "characters,lottery", path, os.Getenv("DFO_PVF_SCOPE_TEST_SHA256"), "missing-characters.json", "", "", "", gamedata.CatalogInputs{
		CharacterPolicyPath: "../../configs/pvf-character-policy.json",
		LotteryPolicyPath:   "missing-policy.json", IndexPath: "missing-index.json", VerifyBaselines: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Characters == nil || c.Items == nil || c.LotteryTables == nil || c.SourceChecksum != c.Characters.Source.Checksum || c.SourceChecksum != c.LotteryTables.Items.SourcePVFSHA256 {
		t.Fatal("source preparation lost identity or catalog")
	}
	bound, err := loadRuntimeLotteryItems(c, "missing-items.json", c.Items.Items)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadRuntimeLotteryEquipment(c, "missing-equipment.json", c.Items.Items, bound); err != nil {
		t.Fatal(err)
	}
	t.Logf("native preparation without exported catalogs: source=%s pools=%d", c.SourceChecksum, len(bound.byTemplate))
}

// Restore the source identities changed by startup so archive tests remain isolated.
func runtimeCatalogAdaptersForTest(t *testing.T) gamedata.CatalogAdapters {
	t.Helper()
	previousLottery, previousOdyssey := lotterySourcePVFSHA256, catalog.OdysseySource
	t.Cleanup(func() {
		lotterySourcePVFSHA256 = previousLottery
		catalog.SetOdysseySource(previousOdyssey)
		inventory.SetClearCubeSource(previousOdyssey)
		quest.SetImageCommunicationSource(previousOdyssey)
	})
	return runtimeCatalogAdapters()
}
