package protocol

import (
	"encoding/hex"
	"testing"
)

// 官服 2026-10-03 蔚蓝号抓包（E:/迅雷下载/20261003-214424/decoded/）的原始载荷。
// 这些是 L0′ 实机真值，不是我们的实现快照 —— 本文件的断言全部指回它们。
const (
	// F16-c2s.txt #304：CMD12 建队（攻坚队），队名 `111`。
	officialAzurePartyCreate115 = "0000030000003131310400000000000000001f00000101020407070707ffffffff000000000000000000000000000000"

	// F16-s2c.txt #369：N9 子命令 0 全量名册，192B，未进本（Info=0）。
	officialAzureRosterFull115 = "01000f270a01014c250001000100030000003131310100040000000000000000" +
		"1f00000000000000000000040000000000010102040707070700ffffffffff00" +
		"000000003c000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000001009a1000010000000001000100a7e1000000010000000000001f00" +
		"a7e1000000010000000000b00025010000000102a6e9c4d53400000000000000"

	// F16-s2c.txt #427：N9 子命令 1（进本后），160B，Info=100004131。
	officialAzureRosterUpdate115 = "01000f270a01014c2501010002000300000031313101000423f1f50500000000" +
		"1f00000000000000000002000000000000010102040707070700ffffffffff00" +
		"000000003c000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000010000001e00b000250100000000023689e2d03600000000000000"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// CMD12：逐字段对官服帧。这条同时是我们对「蔚蓝号 = 攻坚队」的判别依据：
// Options.Mode = 0x1f(31)，而沉月湖那条路径要求 27。
func TestDecodePartyCreateAzureOfficial(t *testing.T) {
	p := mustHex(t, officialAzurePartyCreate115)
	if len(p) != 48 {
		t.Fatalf("official C12 body = %d bytes, want 48", len(p))
	}
	got, err := DecodePartyCreate115(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != 0 || got.Reserved != 0 {
		t.Fatalf("action/reserved = %d/%d, want 0/0", got.Action, got.Reserved)
	}
	if got.Name != "111" {
		t.Fatalf("name = %q, want %q", got.Name, "111")
	}
	o := got.Options
	if o.Capacity != 4 {
		t.Fatalf("capacity = %d, want 4", o.Capacity)
	}
	if o.Info != 0 {
		t.Fatalf("info = %d, want 0（客户端请求里未填）", o.Info)
	}
	if o.Mode != 31 {
		t.Fatalf("mode = %d, want 31 (Azure Main 征服队)", o.Mode)
	}
	if o.SlotFilters != [8]byte{1, 1, 2, 4, 7, 7, 7, 7} {
		t.Fatalf("slot filters = %v, want [1 1 2 4 7 7 7 7]", o.SlotFilters)
	}
	if o.Field20 != 0xffffffff {
		t.Fatalf("field20 = %#x, want 0xffffffff（征服模式要求）", o.Field20)
	}
	if o.Selection != 0 || o.Variant != 0 || o.Extra != 0 || o.ModeValue != 0 {
		t.Fatalf("selection/variant/extra/modevalue = %d/%d/%d/%d, want all 0",
			o.Selection, o.Variant, o.Extra, o.ModeValue)
	}
	if o.LogicalBytes != 30 {
		t.Fatalf("logical bytes = %d, want 30", o.LogicalBytes)
	}
}

// 频道 → (Mode, Route)：102 这一行有官服帧独立佐证（Mode 31 / route 37）。
func TestConquestPartyModeForChannelAzure(t *testing.T) {
	m, ok := ConquestPartyModeForChannel115(102)
	if !ok {
		t.Fatal("channel 102 (Azure Main) missing from conquest table")
	}
	if m.Mode != 31 || m.Route != 37 || !m.Counters {
		t.Fatalf("channel 102 = %+v, want mode 31 route 37 counters true", m)
	}
	if _, ok := ConquestPartyModeForChannel115(9999); ok {
		t.Fatal("unknown channel must not resolve a mode")
	}
}

// LENBLOB 的不变量：数据为空时逐字节恒等（沉月湖/契约能直接吃基础布局就是因为这条）。
func TestInsertLenBlobEmptyIsIdentity(t *testing.T) {
	base, err := PartyRosterSeats115(266, [2]byte{1, 76}, [4]uint16{4250}, 4250, 4)
	if err != nil {
		t.Fatal(err)
	}
	out, err := insertLenBlob115(base, partyRosterTitleBlob115, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(base) {
		t.Fatalf("empty blob changed length: %d -> %d", len(base), len(out))
	}
	for i := range base {
		if out[i] != base[i] {
			t.Fatalf("empty blob changed byte %d: %#x -> %#x", i, base[i], out[i])
		}
	}
	// 非空：长度字段 + 数据，且其后内容原样保留。
	with, err := insertLenBlob115(base, partyRosterTitleBlob115, []byte("111"))
	if err != nil {
		t.Fatal(err)
	}
	if len(with) != len(base)+3 {
		t.Fatalf("titled length = %d, want %d", len(with), len(base)+3)
	}
	if hex.EncodeToString(with[14:21]) != "03000000313131" {
		t.Fatalf("title blob = %s, want 03000000313131（官服 #369 同形）",
			hex.EncodeToString(with[14:21]))
	}
	if hex.EncodeToString(with[21:]) != hex.EncodeToString(base[18:]) {
		t.Fatal("blob 之后的内容没有原样保留")
	}
}

// 位移版专用选项写入在两段 LENBLOB 都为空时必须与 legion_party_base115.go 的原版逐字节一致。
// 防的是"两套实现漂移"——原版是 101/军团路径上实机验证过的那一份。
func TestShiftedApplierMatchesUnshifted(t *testing.T) {
	opts := PartyCreateOptions115{
		Capacity: 4, Info: 100004131, Byte5: 3, Word6: 0x1234, Byte8: 9,
		Mode: 31, ModeValue: 7, SlotFilters: [8]byte{1, 1, 2, 4, 7, 7, 7, 7},
		Field20: 0xffffffff, Selection: 5, Variant: 2,
	}
	base := make([]byte, 90+17*1)
	for i := range base {
		base[i] = byte(i)
	}

	want := append([]byte{}, base...)
	want = applySpecialPartyOptions115(want, opts, 37)

	got := append([]byte{}, base...)
	if err := applySpecialPartyOptionsShifted115(got, opts, opts.Mode, 0, 0); err != nil {
		t.Fatal(err)
	}
	got[len(got)-1] = 37 // 原版把 route 写在 base-1；位移版故意不写，见 Encode 的说明

	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("两段都空时与原版不一致 @%d: want %#x got %#x", i, want[i], got[i])
		}
	}
}

// ★ golden：用官服输入重编 N9 全量名册，对 #369 逐字段核对。
//
// 输入全部来自官服帧本身：title="111"、second=60 个 0、context={1,76}、partyID=266、
// capacity=4、Mode=31、route=37、slot0 actor=4250、N=1。
func TestConquestRosterGoldenAzureOfficial(t *testing.T) {
	official := mustHex(t, officialAzureRosterFull115)
	if len(official) != 192 {
		t.Fatalf("official full roster = %d bytes, want 192", len(official))
	}
	c12, err := DecodePartyCreate115(mustHex(t, officialAzurePartyCreate115))
	if err != nil {
		t.Fatal(err)
	}
	roster, err := ConquestRoster115{
		PartyID: 266,
		Context: [2]byte{1, 76},
		Seats:   [4]uint16{4250},
		Leader:  4250,
		Title:   c12.Name,
		Options: c12.Options,
		Mode:    31,
		Route:   37,
	}.Encode()
	if err != nil {
		t.Fatal(err)
	}

	// 基础 90+17 = 107，队名块 +3，第二段 LENBLOB +60 ⇒ 170。
	// 官服是 192 ⇒ 另有 22 字节尾部**未建模**（见下）。
	if len(roster) != 170 {
		t.Fatalf("roster length = %d, want 170", len(roster))
	}

	fields := []struct {
		name    string
		off, ln int
		want    string // 官服该位置的 hex
	}{
		{"常量 1", 0, 2, "0100"},
		{"PartyInfoType 0x270f", 2, 2, "0f27"},
		{"partyID 266", 4, 2, "0a01"},
		{"context {1,76}", 6, 2, "014c"},
		{"route 37", 8, 1, "25"},
		{"子命令 0", 9, 1, "00"},
		{"队名长度 3", 14, 4, "03000000"},
		{"队名 111", 18, 3, "313131"},
		{"capacity 4", 23, 1, "04"},
		{"Info 0（未进本）", 24, 4, "00000000"},
		{"Byte5", 28, 1, "00"},
		{"Mode 31", 32, 1, "1f"},
		{"SlotFilters", 49, 8, "0101020407070707"},
		{"Selection", 63, 4, "00000000"},
		{"Variant", 67, 1, "00"},
		{"第二段 LENBLOB 长度 60", 68, 4, "3c000000"},
		{"N=1", 132, 1, "01"},
		{"row0 slot 0", 133, 1, "00"},
		{"row0 actor 4250", 134, 2, "9a10"},
		{"第二处 Mode 31", 158, 1, "1f"},
	}
	for _, f := range fields {
		got := hex.EncodeToString(roster[f.off : f.off+f.ln])
		if got != f.want {
			t.Errorf("%s @[%d:%d] = %s, want %s（官服）", f.name, f.off, f.off+f.ln, got, f.want)
		}
	}

	// 尚未建模的位置：打印出来而不是假装对上。这张集合若变了，说明我们对布局的理解变了。
	diffs := []int{}
	for i := 0; i < len(roster); i++ {
		if roster[i] != official[i] {
			diffs = append(diffs, i)
		}
	}
	t.Logf("与官服 #369 相比，[0:%d) 内未建模的差异偏移：%v", len(roster), diffs)
	for _, i := range diffs {
		t.Logf("  [%3d] 我们=%02x 官服=%02x", i, roster[i], official[i])
	}
	t.Logf("官服另有 %d 字节尾部（我们未建模）：%s", len(official)-len(roster),
		hex.EncodeToString(official[len(roster):]))

	// 基线：未建模偏移的集合。它不是「允许差异的清单」，而是**当前理解程度**的记录：
	// 集合一变就说明我们对布局的理解变了，必须重新取证，而不是默默放行。
	// 这些位置在契约那套里都写 0（且 101 已实机验证可用），官服 102 帧则填了值 ——
	// 意义未知，不照抄。见 NOTI9-conquest-evidence.md §7.7。
	baseline := []int{10, 12, 21, 43, 58, 59, 60, 61, 62, 137, 142, 144, 146, 147, 151, 160, 161, 165}
	if len(diffs) != len(baseline) {
		t.Fatalf("未建模偏移集合变了：%v（基线 %v）—— 布局理解发生变化，需重新取证", diffs, baseline)
	}
	for i := range baseline {
		if diffs[i] != baseline[i] {
			t.Fatalf("未建模偏移集合变了：%v（基线 %v）", diffs, baseline)
		}
	}
}

// 跨帧校验：把 Info 换成副本 ID，断言 [24:28] 与 #427（进本后那帧）一致。
// 这条防的是"Info 映射只是碰巧对上 0"。
func TestConquestRosterInfoCarriesDungeonID(t *testing.T) {
	official := mustHex(t, officialAzureRosterUpdate115)
	want := hex.EncodeToString(official[24:28])
	if want != "23f1f505" {
		t.Fatalf("官方 #427 [24:28] = %s，期望 100004131 的 LE 编码", want)
	}
	c12, err := DecodePartyCreate115(mustHex(t, officialAzurePartyCreate115))
	if err != nil {
		t.Fatal(err)
	}
	opts := c12.Options
	opts.Info = 100004131
	roster, err := ConquestRoster115{
		PartyID: 266, Context: [2]byte{1, 76}, Seats: [4]uint16{4250}, Leader: 4250,
		Title: c12.Name, Options: opts, Mode: 31, Route: 37,
	}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got := hex.EncodeToString(roster[24:28])
	if got != want {
		t.Fatalf("Info 段 = %s, want %s（与官服 #427 对齐）", got, want)
	}
}
