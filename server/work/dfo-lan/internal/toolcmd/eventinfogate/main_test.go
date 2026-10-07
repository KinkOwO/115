package eventinfogate

import (
	"bytes"
	"os"
	"testing"
)

// 测试从本包目录出发：../../../ = server/work/dfo-lan。
const (
	captureRel   = "../../../internal/legion/event_info_official.plain"
	generatedRel = "../../../cmd/wireprobe/event_info_generated.go"
)

// TestDerivedGateTableMatchesGeneratedConstant 是本工具存在的理由：从真源
// （internal/legion/event_info_official.plain，77 条官服记录）推导出来的表，必须与
// cmd/wireprobe/event_info_generated.go 里**编入二进制**的那份逐字节相同。
//
// 这条断言把「再生成 → 生成文件 → 服务端下发的体」串成一条可验证的链，
// 让生成文件头部那句 "DO NOT EDIT by hand" 有机器守卫；也证明原来的 Python
// 脚本已经被这份 Go 实现等价替换。
func TestDerivedGateTableMatchesGeneratedConstant(t *testing.T) {
	data, err := os.ReadFile(captureRel)
	if err != nil {
		t.Fatalf("read official capture: %v", err)
	}
	body, err := BuildEventInfoGateTable(data)
	if err != nil {
		t.Fatalf("build gate table: %v", err)
	}
	source, err := os.ReadFile(generatedRel)
	if err != nil {
		t.Fatalf("read generated table: %v", err)
	}
	want, err := ExtractTableHex(source)
	if err != nil {
		t.Fatalf("extract eventInfoTableHex: %v", err)
	}
	if !bytes.Equal(body, want) {
		t.Fatalf("derived gate table (%d bytes) differs from the compiled eventInfoTable (%d bytes); "+
			"the capture or the gate id list drifted", len(body), len(want))
	}
	if len(body) != 1141 {
		t.Fatalf("compiled gate table is %d bytes, want 1141", len(body))
	}
}

// TestEventInfoGateRecordsAreUniqueInOfficialCapture 钉住「按 id 唯一定位」这条
// 前提：19 个门 id 在官服抓包里各自只能命中一条固定形状记录。命中 0 条或多条
// 都必须报错——宁可不生成，也不能发出一张猜出来的表。
func TestEventInfoGateRecordsAreUniqueInOfficialCapture(t *testing.T) {
	data, err := os.ReadFile(captureRel)
	if err != nil {
		t.Fatalf("read official capture: %v", err)
	}
	for _, id := range EventInfoGateIDs {
		record, err := FindEventInfoGateRecord(data, id)
		if err != nil {
			t.Fatalf("gate record 0x%04X: %v", id, err)
		}
		if len(record) < 6 {
			t.Fatalf("gate record 0x%04X is only %d bytes", id, len(record))
		}
	}
	// 一个不在官服表里的 id 必须明确报「没有」，不得静默给空记录。
	if _, err := FindEventInfoGateRecord(data, 0x7FFF); err == nil {
		t.Fatal("an unknown gate id must be refused")
	}
}

// TestEventInfoGateTableFromOfficialCapture 是 001 全频道方案的再生成契约：
// 从官方抓包推导出的体必须是 19 条记录、1141 字节、以 0x00 空表尾收口，
// 且含全部关键门名、不带 URL/横幅负载（选角界面渲染崩溃）、不是 zlib 流
// （私服客户端拒收 zlib 体，探针 V2/V5 判定）。
func TestEventInfoGateTableFromOfficialCapture(t *testing.T) {
	data, err := os.ReadFile(captureRel)
	if err != nil {
		t.Fatalf("read official capture: %v", err)
	}
	body, err := BuildEventInfoGateTable(data)
	if err != nil {
		t.Fatalf("build gate table: %v", err)
	}
	if len(body) != 1141 {
		t.Fatalf("gate table len=%d, want 1141 (2-byte count + 19 records + tail)", len(body))
	}
	if body[0] != 19 || body[1] != 0 {
		t.Fatalf("gate record count=%d, want 19", int(body[0])|int(body[1])<<8)
	}
	if body[len(body)-1] != 0x00 {
		t.Fatal("gate table must end with the empty 0x00 schedule tail")
	}
	for _, name := range []string{
		"Ispins Legion Open", "Apocalypse Channel",
		"DefaultEvent(ENTER_BAKAL_RAID)", "DefaultEvent(ENTER_ASRAHAN_RAID)",
		"DefaultEvent(ENTER_ARTIFICIAL_GOD_RAID)", "DefaultEvent(ENTER_DELEZIE_RAID)",
		"DefaultEvent(ENTER_INAE_DUSK_WAR)", "Venus Open", "Dusky Island Open",
	} {
		if !bytes.Contains(body, []byte(name)) {
			t.Fatalf("gate table missing the %q record", name)
		}
	}
	if bytes.Contains(body, []byte("http")) || bytes.Contains(body, []byte(".xui")) {
		t.Fatal("gate table must stay payload-free; banner records crash the select screen")
	}
	if bytes.HasPrefix(body, []byte{0x78, 0x9c}) {
		t.Fatal("gate table must stay raw; zlib bodies freeze the private client (V2 verdict)")
	}
}

// TestEventInfoGateTableRequiresEveryGateRecord 证明表是**由真源驱动**的：
// 抓包被截断、凑不齐某条门记录时必须报错，而不是静默发出短表
// （短表会让页签静默不解锁，现象与「没修」完全一样）。
func TestEventInfoGateTableRequiresEveryGateRecord(t *testing.T) {
	data, err := os.ReadFile(captureRel)
	if err != nil {
		t.Fatalf("read official capture: %v", err)
	}
	if _, err := BuildEventInfoGateTable(data[:1000]); err == nil {
		t.Fatal("a capture missing gate records must be refused, not silently shortened")
	}
}

// TestExtractTableHexRejectsAForeignFile 确认校验不会对着一个没有该常量的文件
// 静默“通过”（那样 -verify 就成了摆设）。
func TestExtractTableHexRejectsAForeignFile(t *testing.T) {
	if _, err := ExtractTableHex([]byte("package main\n\nconst other = 1\n")); err == nil {
		t.Fatal("a file without eventInfoTableHex must be refused")
	}
}
