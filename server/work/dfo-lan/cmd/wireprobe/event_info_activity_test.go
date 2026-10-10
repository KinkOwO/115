package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"testing"
)

// officialEventCapture 是官服 NOTI108 抓包的真源（77 条记录，与 eventinfogate 工具同源）。
// 相对本包目录：cmd/wireprobe → ../.. = server/work/dfo-lan。
const officialEventCapture = "../../internal/legion/event_info_official.plain"

// readOfficialEventRecords 按 EVENT_INFO 的记录布局走一遍真源，返回 id → 原始记录字节。
//
// 布局（与 event_info_generated.go 的注释、eventinfogate 工具、以及 IDA 复核一致）：
//
//	表 = u16 count + count × 记录 + 0x00 尾
//	记录 = u16 id, u8 present
//	     present == 0 ⇒ 记录到此结束
//	     否则再跟 u8 a, u8 b, str name, str s2, str url, u32 start, u32 end,
//	                 str calendar, str extra, u8 flag
//	str = u32 字节长 + 窄字节串（无结尾 0）
func readOfficialEventRecords(t *testing.T) map[uint16][]byte {
	t.Helper()
	raw, err := os.ReadFile(officialEventCapture)
	if err != nil {
		t.Fatalf("读不到官服抓包 %s: %v", officialEventCapture, err)
	}
	skipStr := func(o int) int {
		n := int(binary.LittleEndian.Uint32(raw[o:]))
		return o + 4 + n
	}
	count := int(binary.LittleEndian.Uint16(raw))
	o := 2
	out := make(map[uint16][]byte, count)
	for i := 0; i < count; i++ {
		if o+3 > len(raw) {
			t.Fatalf("第 %d 条记录越界（偏移 %d / 共 %d 字节）", i, o, len(raw))
		}
		start := o
		id := binary.LittleEndian.Uint16(raw[o:])
		o += 2
		switch raw[o] {
		case 0:
			o++
		default:
			o += 3 // present + a + b
			for k := 0; k < 3; k++ {
				o = skipStr(o)
			}
			o += 8 // start + end
			for k := 0; k < 2; k++ {
				o = skipStr(o)
			}
			o++ // flag
		}
		if o > len(raw) {
			t.Fatalf("第 %d 条记录（id=%d）越界", i, id)
		}
		out[id] = raw[start:o]
	}
	return out
}

// TestActivityRecordsMatchOfficialCapture 钉住 eventInfoActivityHex 与官服抓包**逐字节相同**。
//
// 这条用例的存在理由：我们的活动行不许"手敲"，只能是官服记录的切片。它同时守住
// 「eventInfoActivityIDs 的条数 == hex 里的记录条数」——两者不一致时表头 count 会写得比
// 正文少，客户端按 count 读就丢掉最后一行（同 662/665 那次踩过的坑）。
func TestActivityRecordsMatchOfficialCapture(t *testing.T) {
	records := readOfficialEventRecords(t)
	// 先钉住真源本身的形状：条数变了说明 event_info_official.plain 被换过，
	// 那时所有切片结论都要重新取证，不能让测试静默通过。
	if len(records) != 77 {
		t.Fatalf("官服抓包记录数 %d, want 77 —— 真源被换过，切片结论需重新取证", len(records))
	}

	var want []byte
	for _, id := range eventInfoActivityIDs {
		rec, ok := records[id]
		if !ok {
			t.Fatalf("官服抓包里没有 id=%d 的记录", id)
		}
		want = append(want, rec...)
	}
	if got := hex.EncodeToString(eventInfoActivityTable); got != hex.EncodeToString(want) {
		t.Fatalf("活动行与官服记录不一致:\n got=%s\nwant=%s", got, hex.EncodeToString(want))
	}

	// hex 里的记录条数必须等于 id 列表长度。
	walked := 0
	for o := 0; o < len(eventInfoActivityTable); {
		recordLen := len(records[binary.LittleEndian.Uint16(eventInfoActivityTable[o:])])
		if recordLen == 0 {
			t.Fatalf("活动行 #%d 不是任何一个官服 id 的记录", walked)
		}
		o += recordLen
		walked++
	}
	if walked != len(eventInfoActivityIDs) {
		t.Fatalf("活动行条数 %d, want %d", walked, len(eventInfoActivityIDs))
	}
}

// TestActivityRowIsAttendanceDaily 把 id 331 的关键字段钉死：标题、期窗、日历串。
// 任何一条与官服不符都会改变客户端对"活动开着 / 在期"的判断（§0.2：取值必须与官服一致）。
func TestActivityRowIsAttendanceDaily(t *testing.T) {
	records := readOfficialEventRecords(t)
	rec, ok := records[331]
	if !ok {
		t.Fatal("官服抓包里没有 id=331")
	}
	// 字段级断言（比整串十六进制更耐读；整串的逐字节真值由上面那条用例守）：
	if got := binary.LittleEndian.Uint16(rec); got != 331 {
		t.Fatalf("id=%d", got)
	}
	if len(rec) != 132 {
		t.Fatalf("记录长 %d, want 132", len(rec))
	}
	o := 5 // id(2) + present(1) + a(1) + b(1)
	str := func() string {
		n := int(binary.LittleEndian.Uint32(rec[o:]))
		o += 4
		s := string(rec[o : o+n])
		o += n
		return s
	}
	if name := str(); name != "7-Day Journey for Sky of a Thousand Seas" {
		t.Fatalf("标题 %q 与官服不符", name)
	}
	if s2 := str(); s2 != "" {
		t.Fatalf("第二串 %q, want 空", s2)
	}
	if url := str(); url != "" {
		t.Fatalf("url %q, want 空（带 url 的记录是选角屏崩溃风险面）", url)
	}
	if start, end := binary.LittleEndian.Uint32(rec[o:]), binary.LittleEndian.Uint32(rec[o+4:]); start != 1785801600 || end != 1791277198 {
		t.Fatalf("期窗 %d~%d, want 1785801600~1791277198", start, end)
	}
	o += 8
	if cal := str(); cal != `Live/Event/Kor/\2026\0326_AttendanceDailyEvent/cal.xui/0/0` {
		t.Fatalf("日历串 %q 与官服不符", cal)
	}
	if extra := str(); extra != "" {
		t.Fatalf("extra %q, want 空", extra)
	}
	if flag := rec[o]; flag != 1 {
		t.Fatalf("flag=%d, want 1", flag)
	}
}

// TestTownEventInfoTableCarriesActivityRows 钉住合并语义：活动行确实进了进城表，
// 且 count 跟着走；开关关掉时回到只含 Boost Up 行的旧行为。
func TestTownEventInfoTableCarriesActivityRows(t *testing.T) {
	base := binary.LittleEndian.Uint16(eventInfoTable[:2])
	records := readOfficialEventRecords(t)
	attendance := records[331]

	with, ok := buildTownEventInfoTable(true)
	if !ok {
		t.Fatal("合并失败")
	}
	if !bytes.Contains(with, attendance) {
		t.Fatal("进城表里没有 331 的原始记录")
	}
	if got, want := binary.LittleEndian.Uint16(with[:2]), base+uint16(len(eventInfoActivityIDs))+4; got != want {
		t.Fatalf("count=%d, want %d", got, want)
	}

	t.Setenv("DFO_EVENT_INFO_ACTIVITY", "off")
	off, ok := buildTownEventInfoTable(true)
	if !ok {
		t.Fatal("开关关闭后合并失败")
	}
	if bytes.Contains(off, attendance) {
		t.Fatal("开关关闭后仍带 331 行")
	}
	if got, want := binary.LittleEndian.Uint16(off[:2]), base+4; got != want {
		t.Fatalf("开关关闭后 count=%d, want %d", got, want)
	}
}

// TestActivityRowsSwitchAcceptsOffSpellings 钉住开关的取值口径（profile 里写哪个都能关）。
func TestActivityRowsSwitchAcceptsOffSpellings(t *testing.T) {
	for _, v := range []string{"off", "OFF", " off ", "0", "false"} {
		t.Setenv("DFO_EVENT_INFO_ACTIVITY", v)
		if activityEventInfoRows() != nil {
			t.Fatalf("%q 未关掉活动行", v)
		}
	}
	for _, v := range []string{"", "boost", "on", "1", "true"} {
		t.Setenv("DFO_EVENT_INFO_ACTIVITY", v)
		if activityEventInfoRows() == nil {
			t.Fatalf("%q 意外关掉了活动行", v)
		}
	}
}
