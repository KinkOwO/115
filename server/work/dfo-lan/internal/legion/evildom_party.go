package legion

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// 次元回廊待机区建队链（与伊斯/森林**同构**，只换队伍类型字节）。
//
// 官服抓包（official_20261009-223033_live / session_s30）：
//
//	22:47:28.029 c2s CMD12  建队（队名 "11"，容量 4，队伍类型 0x0d，模式 1）
//	22:47:31.839 c2s CMD36  ← 客户端建队后**自己**往下走了（没等服务端应答）
//	22:47:59.033 c2s CMD2043 开始作战
//
// CMD12 请求逐字节（48B，与 ispins_party.go 记录的布局逐字段对齐）：
//
//	00 00 | 02 00 00 00 | 31 31 | 04 00 00 00 | 00 00 00 00 00 | 0d | 01 00 | 01 01 02 04 07 07 07 07 | ff ff ff ff | 00 …
//	       @2 名长=2       @6 名字  @8 容量=4        @13.. 零        @17 类型  @18 模式
//
// ⚠ 建队应答**不在这个包里构造**：官服抓包那 176 字节 N9 是官服新版客户端布局，
// 本机 2.38.2 读取器解析会读错字段（2026-10-10 两轮实机：服务端发了帧、客户端界面
// 毫无反应）。应答走 `protocol.EvildomPartyReply` —— 也就是伊斯/森林在用的
// 2.38.2 原生 NOTI9 模板，只换队伍类型 0x0d。
const (
	// EvildomPartyType 是次元回廊待机区的队伍类型字节（请求正文 @(n+11) 处）：
	// 军团家族里只有它用 0x0d（末世录 0x26 / 伊斯 0x0b / 维纳斯 0x22 / 森林 0x18,0x19）。
	EvildomPartyType byte = 0x0d
	// EvildomPartyMode 是这一族建队请求里的模式 u16（官方与实机都是 1），仅用于诊断。
	EvildomPartyMode uint16 = 1
	// EvildomPartyCapacity 是抓包实证的请求容量（4 人军团队）；解码时按原值记录。
	EvildomPartyCapacity uint32 = 4
)

// evildomPartyNameOffset 是 CMD12 请求里队名区的起始偏移（与其它待机区同值 @6）。
const evildomPartyNameOffset = 6

// EvildomPartyRequest 是次元回廊待机区 CMD12 的解码结果。
type EvildomPartyRequest struct {
	// Name 是解码后的队名（日志与记录用）。
	Name string
	// NameReplyBytes 是交给 protocol.EvildomPartyReply 的队名字节。
	NameReplyBytes []byte
	Capacity       uint32
	PartyType      byte
	Mode           uint16
	BodyLength     int
}

// DecodeEvildomParty 解码次元回廊待机区的建队请求。
//
// 布局与伊斯/森林**完全同构**（「组队应该都差不多」的出处），只有两处不同：
// 容量按原值记录（1..4，不强制 4 人），队伍类型字节是 0x0d。
//
//	@0..1 u16 0 | @2 名长 u32 = n | @6..@(6+n) 队名 | @6+n 容量 u32(1..4)
//	| @(n+15) 队伍类型 u8 | @(n+16) 模式 u16
//
// 偏移口径与 `DecodeIspinsStandbyParty`/`DecodeForestStandbyParty` 完全一致
// （at = 6+n；容量 = u32@at、类型 = p[at+9]、模式 = u16@at+10）—— 两次实机已
// 证明自造偏移会读错字段，所以这里只照抄，不另发明。
func DecodeEvildomParty(p []byte) (EvildomPartyRequest, error) {
	if len(p) < 36 {
		return EvildomPartyRequest{}, fmt.Errorf("次元回廊建队载荷 %d 字节，至少需要 36", len(p))
	}
	n := int(binary.LittleEndian.Uint32(p[2:]))
	if n < 1 || n > 63 || n > len(p)-36 {
		return EvildomPartyRequest{}, fmt.Errorf("次元回廊建队：队名字节数 %d 无效", n)
	}
	at := evildomPartyNameOffset + n
	if at+12 > len(p) {
		return EvildomPartyRequest{}, fmt.Errorf("次元回廊建队：载荷被截断")
	}
	capacity := binary.LittleEndian.Uint32(p[at:])
	if capacity < 1 || capacity > 4 {
		return EvildomPartyRequest{}, fmt.Errorf("次元回廊建队：队伍人数必须为 1 至 4 人，收到 %d", capacity)
	}
	raw := append([]byte(nil), p[evildomPartyNameOffset:at]...)
	name, reply := decodeEvildomName(raw)
	if name == "" {
		return EvildomPartyRequest{}, fmt.Errorf("次元回廊建队：队名为空")
	}
	req := EvildomPartyRequest{
		Name:           name,
		NameReplyBytes: reply,
		Capacity:       capacity,
		PartyType:      p[at+9],
		Mode:           binary.LittleEndian.Uint16(p[at+10:]),
		BodyLength:     len(p),
	}
	if req.PartyType != EvildomPartyType {
		return EvildomPartyRequest{}, fmt.Errorf(
			"次元回廊建队：队伍类型 %#x 不是 %#x（军团家族共用 CMD12）",
			req.PartyType, EvildomPartyType)
	}
	return req, nil
}

// decodeEvildomName 解析请求里的队名区，并给出交给应答模板的字节。
//
// 纯 ASCII 名字（业主实机 "55555"）按原字节回写；含 UTF-16 零字节的按
// UTF-16LE 解出字符串后再编回 —— 两种客户端语义都能对上，模板侧只认字节序列。
func decodeEvildomName(raw []byte) (name string, reply []byte) {
	hasNUL := false
	for _, b := range raw {
		if b == 0 {
			hasNUL = true
			break
		}
	}
	if !hasNUL {
		return string(raw), append([]byte(nil), raw...)
	}
	even := raw
	if len(even)%2 == 1 {
		even = even[:len(even)-1]
	}
	decoded := utf16leToString(even)
	return decoded, EvildomPartyNameBytes(decoded)
}

// EvildomPartyNameBytes 把队名编成 UTF-16LE 字节（含零字节的请求走这条路）。
func EvildomPartyNameBytes(name string) []byte {
	units := utf16.Encode([]rune(name))
	out := make([]byte, 0, len(units)*2)
	for _, u := range units {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

// utf16leToString 把 UTF-16LE 字节解成字符串。
func utf16leToString(b []byte) string {
	units := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		units = append(units, uint16(b[i])|uint16(b[i+1])<<8)
	}
	return string(utf16.Decode(units))
}
