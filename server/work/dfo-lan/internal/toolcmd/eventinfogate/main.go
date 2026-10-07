// eventinfogate: 从官方 NOTI108 抓包切片重新生成 / 校验
// cmd/wireprobe/event_info_generated.go 里的 eventInfoTableHex。
//
// 这是原 Python 脚本 reference/analysis-tools/gen_event_info_gate_table.py 的
// Go 复现（业主 2026-10-05 要求：包引入/依赖的 Python 一律移植为 Go，
// 仓库维持「启动链无 Python」）。原脚本随修复包交付后被删除，本工具以同一份
// 真源（internal/legion/event_info_official.plain，77 条官服记录）为准，
// 并有测试证明它逐字节复现了已编入服务端的那张表。
//
// 用法（在 server/work/dfo-lan 下运行）:
//
//	dfo-tool eventinfogate                    # 按默认路径生成并把表格 hex 打到 stdout
//	dfo-tool eventinfogate -verify <file.go>  # 校验生成结果与生成文件里的常量逐字节一致
package eventinfogate

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// EventInfoGateIDs 是「军团 / 攻坚战」页签门记录在官服 NOTI108 事件表里的 19 个
// 固定形状 id，顺序与官服表一致。页签解锁由事件表位驱动（IDA：bit-1007 的消费者
// 持有攻坚战页签锁；探针 V3 证明记录 776 能打开伊斯页签），只有把这些门记录一起
// 编入 NOTI108 体，各团本页签才不再显示「无法入场」。
//
// 清单与顺序是 2026-10-04 实机取证结论，逐条对应官服表里的固定形状记录。
var EventInfoGateIDs = []uint16{
	0x0991, // FiendWarEnterDungeonEvent (45)
	0x019E, // PreyRaidEnterDungeonEvent (50)
	0x01CE, // DefaultEvent(ENTER_SIROCO_RAID) (67)
	0x01E2, // DefaultEvent(ENTER_OZMA_RAID) (76)
	0x0308, // Ispins Legion Open (81/86/87)
	0x0280, // DefaultEvent(ENTER_PRE_BAKAL_RAID) (83)
	0x0281, // DefaultEvent(ENTER_BAKAL_RAID) (82)
	0x026D, // (84, official record name empty)
	0x0282, // Dusky Island Open (91)
	0x02A8, // Forest of Awakening Normal (96)
	0x02A9, // Forest of Awakening Extreme (96)
	0x024E, // EventPreAsrahanEnter(ENTER_PRE_ASRAHAN_RAID) (92)
	0x0272, // DefaultEvent(ENTER_ASRAHAN_RAID) (93)
	0x0273, // Venus Open (99)
	0x0274, // Semi Raid Bidding Open (85/86)
	0x03D1, // DefaultEvent(ENTER_ARTIFICIAL_GOD_RAID) (98/107)
	0x032C, // DefaultEvent(ENTER_INAE_DUSK_WAR) (111)
	0x0383, // DefaultEvent(ENTER_DELEZIE_RAID) (112/120)
	0x03EF, // Apocalypse Channel (119/30/31/32)
}

// 官方事件表窗口的合理区间（Unix 秒）。门记录的时间窗都是官服表原值，落在
// 2023..2034 之间；用它对「在字符串体内偶然撞上同一个 id」的误命中做过滤。
const (
	eventInfoWindowMin = 1_600_000_000 // 2020-09
	eventInfoWindowMax = 2_200_000_000 // 2039-09
)

// genHexLiteral 匹配生成文件里 eventInfoTableHex 的多行 "" + "hex..." 拼接字面量。
var genHexLiteral = regexp.MustCompile(`(?m)^\t"([0-9a-fA-F]*)"`)

func Run() {
	plain := flag.String("plain", "internal/legion/event_info_official.plain",
		"official NOTI108 capture (.plain) used as the regeneration source")
	verify := flag.String("verify", "",
		"generated Go file whose eventInfoTableHex must match; empty prints the hex instead")
	flag.Parse()

	data, err := os.ReadFile(*plain)
	if err != nil {
		panic(err)
	}
	body, err := BuildEventInfoGateTable(data)
	if err != nil {
		panic(err)
	}
	if *verify == "" {
		fmt.Printf("event-info gate table: %d gate records, %d bytes\n", len(EventInfoGateIDs), len(body))
		fmt.Println(hex.EncodeToString(body))
		return
	}
	source, err := os.ReadFile(*verify)
	if err != nil {
		panic(err)
	}
	want, err := ExtractTableHex(source)
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(body, want) {
		fmt.Fprintf(os.Stderr,
			"event-info gate table drifted: derived %d bytes from %s, but %s carries %d bytes\n",
			len(body), *plain, *verify, len(want))
		os.Exit(1)
	}
	fmt.Printf("event-info gate table: %d gate records, %d bytes; identical to %s\n",
		len(EventInfoGateIDs), len(body), *verify)
}

// ExtractTableHex 取出生成文件里 eventInfoTableHex 常量的十六进制字面量，
// 解码成服务端实际下发的字节。用于「再生成结果 == 编入二进制的表」这条契约。
func ExtractTableHex(source []byte) ([]byte, error) {
	text := string(source)
	start := strings.Index(text, "const eventInfoTableHex")
	if start < 0 {
		return nil, fmt.Errorf("generated file carries no eventInfoTableHex constant")
	}
	var joined strings.Builder
	for _, match := range genHexLiteral.FindAllStringSubmatch(text[start:], -1) {
		joined.WriteString(match[1])
	}
	if joined.Len() == 0 {
		return nil, fmt.Errorf("eventInfoTableHex has no hex literals to read")
	}
	body, err := hex.DecodeString(joined.String())
	if err != nil {
		return nil, fmt.Errorf("eventInfoTableHex is not valid hex: %w", err)
	}
	return body, nil
}

// parseFixedEventInfoRecord 从 off 起按 v2.38.2.34 的固定记录布局解析一条记录：
//
//	u16 id, u8 flag1, u8 f1, u8 f2, str x3, u32 start, u32 end, str x2, u8 flag2
//
// 其中 str 是 u32 长度 + 原始字节。解析自洽（不越界）且时间窗落在官服区间内时
// 返回该记录的原始字节。
//
// **注意**：官方表里并非每条记录都是这个固定形状——逐条顺序走会在第 5 条失步
// （那条记录尾部带别的字段）。但 19 条门记录本身都是固定形状、且各自唯一，
// 所以这里不顺序遍历，而是按 id 唯一定位。
func parseFixedEventInfoRecord(data []byte, off int) ([]byte, bool) {
	start := off
	if off+5 > len(data) {
		return nil, false
	}
	off += 5 // id(2) + flag1 + f1 + f2
	ok := true
	for s := 0; s < 3 && ok; s++ {
		off, ok = skipEventInfoString(data, off)
	}
	if !ok || off+8 > len(data) {
		return nil, false
	}
	windowStart := binary.LittleEndian.Uint32(data[off:])
	windowEnd := binary.LittleEndian.Uint32(data[off+4:])
	off += 8
	for s := 0; s < 2 && ok; s++ {
		off, ok = skipEventInfoString(data, off)
	}
	if !ok || off+1 > len(data) {
		return nil, false
	}
	off++ // flag2
	if windowStart < eventInfoWindowMin || windowStart > eventInfoWindowMax ||
		windowEnd < eventInfoWindowMin || windowEnd > eventInfoWindowMax ||
		windowEnd < windowStart {
		return nil, false
	}
	return data[start:off], true
}

// skipEventInfoString 跳过一条 u32 长度前缀字符串，返回下一条记录的偏移。
func skipEventInfoString(data []byte, off int) (int, bool) {
	if off+4 > len(data) {
		return off, false
	}
	n := int(binary.LittleEndian.Uint32(data[off:]))
	off += 4
	if n > len(data)-off {
		return off, false
	}
	return off + n, true
}

// FindEventInfoGateRecord 在官方抓包里定位 id 对应的门记录。要求**唯一命中**：
// 官服表里每个门 id 只出现一次，命中多于一条说明抓包或校验条件出了问题，
// 此时宁可报错也不猜。
func FindEventInfoGateRecord(data []byte, id uint16) ([]byte, error) {
	var found []byte
	for off := 0; off+2 <= len(data); off++ {
		if binary.LittleEndian.Uint16(data[off:]) != id {
			continue
		}
		record, ok := parseFixedEventInfoRecord(data, off)
		if !ok {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("gate record 0x%04X is ambiguous in the official capture", id)
		}
		found = record
	}
	if found == nil {
		return nil, fmt.Errorf("official capture carries no fixed-shape gate record 0x%04X", id)
	}
	return found, nil
}

// BuildEventInfoGateTable 复现 cmd/wireprobe 下发的 NOTI108 体：
// count(2) + 19 条官服门记录（逐字节 verbatim）+ 0x00 空表尾。这 19 条都是
// 固定形状、无 URL/横幅负载的记录，所以不会触发选角界面渲染崩溃；体必须是
// RAW 裸格式（zlib 体会被私服客户端拒收并冻结，探针 V2/V5 判定）。
func BuildEventInfoGateTable(data []byte) ([]byte, error) {
	body := make([]byte, 2, 2+19*60)
	binary.LittleEndian.PutUint16(body, uint16(len(EventInfoGateIDs)))
	for _, id := range EventInfoGateIDs {
		record, err := FindEventInfoGateRecord(data, id)
		if err != nil {
			return nil, err
		}
		body = append(body, record...)
	}
	return append(body, 0x00), nil
}
