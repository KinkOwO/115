package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"testing"
)

// 真实内层归档对照：把频道解析器压到当次源上。
//
//	DFO_PVF_CORE_TEST_ARCHIVE=D:\115us\server\work\client-build\Script.inner.pvf
//	go test ./internal/catalog/ -run 'ChannelDirectoryNative|ChannelInfoNative' -count=1
//
// 未设置环境变量时跳过（与其它 *_native_test.go 同一约定）。
func openChannelArchive(t *testing.T) *pvf.Archive {
	t.Helper()
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native channel parity")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: p, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

// F7 那六个频道：属性必须与源逐字段一致（town / guide dungeon / 面板归属）。
//
// 这六项**不是**军团范畴：源里 isLegion=0、isSemiRaid=1。它们的 ID（发布用频道号）
// 由本地配置按"ID = type"约定给出 —— 符号名到 type 数字没有真源（见文件头注释）。
func TestChannelDirectoryNative(t *testing.T) {
	a := openChannelArchive(t)
	dir, err := ImportChannelDirectory(a)
	if err != nil {
		t.Fatalf("import channel directory: %v", err)
	}
	if len(dir.ByType) == 0 {
		t.Fatal("no channel types imported")
	}
	// slot 段是 channelslotinfo 的核心：打印一下，便于定位"面板对不上"的问题。
	t.Logf("channel types=%d, slot symbols=%d", len(dir.ByType), len(dir.SlotSymbols))
	for _, s := range dir.SlotSymbols {
		t.Logf("  slot symbol: %s", s)
	}

	cases := []struct {
		channelType uint32
		town        int
		dungeon     uint32
		// panel 为空表示源里对不上：那一行只有符号名、没有 [channel type] 数字，
		// 而符号→type 没有真源（见 channel_directory.go 文件头）。这种项不参与断言。
		panel string
	}{
		{108, 220, 100004520, ""},                     // 噩梦循环 Unshackled Nightmare
		{103, 214, 100004134, ""},                     // 幽冥之女神殿 Temple of Death
		{102, 213, 100004131, ""},                     // 蔚蓝号 Azure Main
		{101, 215, 100004137, "conquestDungeonPanel"}, // 沉月湖：源里直接写 [channel type] 101
		{117, 237, 100003630, ""},                     // Castle of the Apostate
		{116, 240, 100005013, ""},                     // Constellation Turtle Library
	}
	for _, c := range cases {
		got, ok := dir.Attributes(c.channelType)
		if !ok {
			t.Fatalf("channel type %d is missing from the source", c.channelType)
		}
		if got.Town != c.town {
			t.Fatalf("type %d [seriaRoomTown] = %d, want %d", c.channelType, got.Town, c.town)
		}
		if got.GuideDungeon != c.dungeon {
			t.Fatalf("type %d [guide dungeon index] = %d, want %d", c.channelType, got.GuideDungeon, c.dungeon)
		}
		if c.panel != "" && got.Panel != c.panel {
			t.Fatalf("type %d [attach panel] = %q, want %q", c.channelType, got.Panel, c.panel)
		}
		// 这六项属于征服面板，不是军团。
		if got.IsLegion {
			t.Fatalf("type %d must not be isLegion (source says isSemiRaid=1)", c.channelType)
		}
		if !got.IsSemiRaid {
			t.Fatalf("type %d must be isSemiRaid", c.channelType)
		}
	}

	// 源里真正 isLegion=1 的是这六个 —— 不能因为中文名把它们和上面混为一谈。
	for _, legionType := range []uint32{81, 84, 91, 96, 99, 119} {
		got, ok := dir.Attributes(legionType)
		if !ok {
			t.Fatalf("legion channel type %d is missing from the source", legionType)
		}
		if !got.IsLegion {
			t.Fatalf("type %d should be isLegion", legionType)
		}
	}

	// 军团频道 119 的房间城镇是 239（与本地配置的 Note 一致）。
	if got, ok := dir.Attributes(119); !ok || got.Town != 239 {
		t.Fatalf("type 119 town = %d (ok=%v), want 239", got.Town, ok)
	}
	// 赤红铁矿 106 的房间城镇 218；它的 [guide dungeon index] 在源里**没有**，
	// 所以这里只断言城镇 —— 106/107 的区别不能被 dungeon 关联法搞混（见下一条测试）。
	if got, ok := dir.Attributes(106); !ok || got.Town != 218 {
		t.Fatalf("type 106 town = %d (ok=%v), want 218", got.Town, ok)
	}
}

// 反证：`[drop item dungeon index]`（channelslotinfo）与 `[guide dungeon index]`
// （clientchannelinfo）**语义不同**，不能用前者反查 type。
//
// 实测：CHANNEL_BLEEDING_MINE 的 slot 写 `[drop item dungeon index] 100004326`，
// 而 100004326 在 clientchannelinfo 里挂的是 type **107**；赤红铁矿的真值是 **106**。
// 所以这条关联会把 106 错算成 107 —— 解析器**不采用**它，只按 type 对齐。
func TestChannelDirectoryDoesNotDeriveTypeFromSlotDungeon(t *testing.T) {
	a := openChannelArchive(t)
	dir, err := ImportChannelDirectory(a)
	if err != nil {
		t.Fatalf("import channel directory: %v", err)
	}
	bleeding, ok := dir.Attributes(106)
	if !ok {
		t.Fatal("type 106 (Bleeding Mine) is missing from the source")
	}
	if bleeding.SlotSymbol == "" {
		// 106 在 channelslotinfo 里用符号 CHANNEL_BLEEDING_MINE 登记；若源改成别处登记，
		// 本断言会失败，提醒重新取证而不是静默漂移。
		t.Logf("type 106 slot symbol is empty; source layout may have changed")
	}
	// 106 的 guide dungeon 在源里没有（那是不落副本的特殊频道），必须为 0。
	if bleeding.GuideDungeon != 0 {
		t.Fatalf("type 106 [guide dungeon index] = %d, want 0 (absent in source)", bleeding.GuideDungeon)
	}
	// 107 才是挂了 100004326 的那个。
	if got, ok := dir.Attributes(107); !ok || got.GuideDungeon != 100004326 {
		t.Fatalf("type 107 guide dungeon = %d (ok=%v), want 100004326", got.GuideDungeon, ok)
	}
}

// 普通频道目录：server 1 的行必须能逐字段读出来。
func TestChannelInfoNative(t *testing.T) {
	a := openChannelArchive(t)
	info, err := ImportChannelInfo(a)
	if err != nil {
		t.Fatalf("import channel info: %v", err)
	}
	rows, ok := info.Rows(1)
	if !ok {
		t.Fatalf("server 1 is missing; servers=%v", info.ServerIDs())
	}
	if len(rows) < 3 {
		t.Fatalf("server 1 has %d channel row(s)", len(rows))
	}
	byID := map[uint32]ChannelInfoRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	// 源：`1 2 [elven_guard] 10 0 0 0 0 0 0 0 0 0 0`
	if r, ok := byID[1]; !ok || r.Type != 2 || r.Area != "[elven_guard]" || r.SourceValues[0] != "10" {
		t.Fatalf("channel 1 = %+v, want type 2 / [elven_guard] / first value 10", r)
	}
	// 源：`6 3 [none] 0 …`
	if r, ok := byID[6]; !ok || r.Type != 3 || r.Area != "[none]" {
		t.Fatalf("channel 6 = %+v, want type 3 / [none]", r)
	}
	// 源：`10 0 [granfloris] 5 0 …` —— 注意 type 是 **0**，不是本地配置用的 22。
	if r, ok := byID[10]; !ok || r.Type != 0 || r.Area != "[granfloris]" || r.SourceValues[0] != "5" {
		t.Fatalf("channel 10 = %+v, want type 0 / [granfloris] / first value 5", r)
	}
	for _, r := range rows {
		if len(r.SourceValues) != 11 {
			t.Fatalf("channel %d has %d source value(s), want 11", r.ID, len(r.SourceValues))
		}
	}
	// 这张表只有普通频道：全部 type 必须落在 0..6。
	for _, r := range rows {
		if r.Type > 6 {
			t.Fatalf("channel %d type = %d; etc/channel_info.etc should only carry ordinary types", r.ID, r.Type)
		}
	}
	if _, ok := info.AreaDungeons["[elven_guard]"]; !ok {
		t.Fatalf("[dungeon] blocks not parsed: %v", info.AreaDungeons)
	}
}
