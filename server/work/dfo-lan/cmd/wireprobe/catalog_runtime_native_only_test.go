package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"strings"
	"testing"
)

func TestPreparedLotteryIgnoresRetiredBaseline(t *testing.T) {
	before := lotterySourcePVFSHA256
	t.Cleanup(func() { lotterySourcePVFSHA256 = before })
	source := strings.Repeat("a", 64)
	SetLotterySource(source)
	tables := catalog.LotteryTables{
		Items:     catalog.LotteryPoolCatalog{SourcePVFSHA256: source, Pools: []catalog.LotterySourcePool{{SourceItem: 10, SourceScript: "stackable/new.stk", SourceScriptSHA256: strings.Repeat("b", 64), Candidates: [][3]uint32{{20, 4, 3}, {0, 6, 1234}}}}},
		Equipment: catalog.LotteryPoolCatalog{SourcePVFSHA256: source},
	}
	index := catalog.ItemIndex{Items: map[uint32]ItemIndexInfo{10: {ID: 10, Kind: "stackable", StackableType: "[upgradable legacy]", Path: "stackable/new.stk"}, 20: {ID: 20, Kind: "stackable"}}}
	if err := validatePreparedLottery(tables, index, t.TempDir(), true); err != nil {
		t.Fatal(err)
	}
	delete(index.Items, 20)
	if err := validatePreparedLottery(tables, index, t.TempDir(), true); err == nil {
		t.Fatal("unknown payout accepted")
	}
}

func TestRuntimeLotteryRequiresPreparedSource(t *testing.T) {
	c := &gamedata.Catalogs{}
	if _, err := loadRuntimeLotteryItems(c, "missing.json", nil); err == nil {
		t.Fatal("unprepared lottery accepted")
	}
	if _, err := loadRuntimeLotteryEquipment(c, "missing.json", nil, nil); err == nil {
		t.Fatal("nil base accepted")
	}
}
