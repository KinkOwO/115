package gamedata

import (
	"os"
	"testing"
)

// 蔚蓝号（Azure Main）频道接入的原生对照：证明 type 102 的专属城镇能从当前 PVF 直读出来。
//
//	DFO_PVF_CORE_TEST_ARCHIVE=D:\115us\server\work\client-build\Script.inner.pvf
//	go test ./internal/gamedata/ -run 'AzureMainChannel' -count=1
//
// 未设置环境变量时跳过（与其它 *_native_test.go 同一约定）。
//
// 这份断言只覆盖「频道 → 专属城镇」这一段，不声明副本玩法已实现：
// configs/channel.local34.json 里 102 只声明 {ID, Name}，Type/Town/Dungeon 全部由
// clientchannelinfo.etc 直读补全（main.go 的 channelrefresh.Config.Resolve）。
func TestAzureMainChannelTownNative(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for Azure Main channel/town parity")
	}
	s, err := Open(Options{
		Mode:             PVF,
		ArchivePath:      p,
		ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"),
		DerivedCacheDir:  t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// 只跑频道投影：preparePVFChannels 只读 etc/clientchannelinfo.etc + list/town.lst，
	// 不装载任何玩法表，所以这一步很轻。
	var c Catalogs
	if err := preparePVFChannels(&c, s); err != nil {
		t.Fatalf("preparePVFChannels: %v", err)
	}

	a, ok := c.ChannelDirectory.Attributes(102)
	if !ok {
		t.Fatal("channel type 102 (Azure Main) is missing from clientchannelinfo.etc")
	}
	if a.Town != 213 {
		t.Fatalf("type 102 [seriaRoomTown] = %d, want 213", a.Town)
	}
	if a.GuideDungeon != 100004131 {
		t.Fatalf("type 102 [guide dungeon index] = %d, want 100004131", a.GuideDungeon)
	}
	if !a.IsSemiRaid {
		t.Fatalf("type 102 must be isSemiRaid (source row), got false")
	}
	if a.IsLegion {
		t.Fatalf("type 102 must not be isLegion (source says isSemiRaid)")
	}

	town, ok := c.ChannelTowns[102]
	if !ok {
		t.Fatal("type 102 has no usable channel town (walkable area missing)")
	}
	if town.TownID != 213 {
		t.Fatalf("channel 102 town = %d, want 213", town.TownID)
	}
	if len(town.Walkable) == 0 {
		t.Fatal("channel 102 town has no walkable area")
	}
	x, y := town.Spawn()
	t.Logf("channel 102 -> town %d/%d %s spawn=(%d,%d) walkable=%d",
		town.TownID, town.AreaID, town.MapPath, x, y, len(town.Walkable))

	// 反证：频道城镇表是按 type 逐项构建的，加 102 不能把沉月湖(101) 顶掉。
	moonTown, ok := c.ChannelTowns[101]
	if !ok {
		t.Fatal("channel 101 (Moon Lake) town disappeared")
	}
	if moonTown.TownID != 215 {
		t.Fatalf("channel 101 town = %d, want 215", moonTown.TownID)
	}
}
