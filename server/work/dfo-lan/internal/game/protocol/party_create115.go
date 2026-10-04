package protocol

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf8"
)

// CMD12 建队请求（C→S，type=1，id=12）。
//
// 布局（对官服 2026-10-03 蔚蓝号抓包 F16-c2s.txt #304 逐字段核过）：
//
//	[0]      Action   0=创建 / 1=更新配置
//	[1]      Reserved
//	[2:6]    u32 LE   名字长度 n
//	[6:6+n]  UTF-8    名字
//	[6+n:]   选项尾（30..45 字节，逻辑只取前 30）
//
// 名字字段在**普通队伍**路径下是队长角色名（必须与在线角色名逐字节相等），
// 在**征服/攻坚队**路径下是队名 —— 两种路径下面这份解码完全一样，差别的只是它的语义与
// 选项尾里的 Mode。见 ConquestPartyRoster115。
type PartyCreate115 struct {
	Action   byte
	Reserved byte
	Name     string
	Options  PartyCreateOptions115
}

func DecodePartyCreate115(p []byte) (PartyCreate115, error) {
	var out PartyCreate115
	if len(p) < 10 {
		return out, fmt.Errorf("short party create request")
	}
	out.Action, out.Reserved = p[0], p[1]
	n := int(binary.LittleEndian.Uint32(p[2:]))
	if n < 1 || n > 31 || 6+n > len(p) {
		return out, fmt.Errorf("invalid party create name length %d", n)
	}
	name := string(p[6 : 6+n])
	if !utf8.ValidString(name) || strings.TrimSpace(name) != name {
		return out, fmt.Errorf("invalid party create name encoding")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return out, fmt.Errorf("control character in party create name")
		}
	}
	out.Name = name
	opts, err := DecodePartyCreateOptions115(p[6+n:])
	if err != nil {
		return out, err
	}
	out.Options = opts
	return out, nil
}

// DecodePartyCreateOptions115 解 CMD12 的选项尾。
//
// 逻辑长度固定 30 字节；客户端会把尾部补齐到 30..45（当前实测样本是 39 = 30 + 9 个 0）。
// 逐字段含义见 legion_party_base115.go 的 PartyCreateOptions115 注释。
func DecodePartyCreateOptions115(tail []byte) (PartyCreateOptions115, error) {
	var o PartyCreateOptions115
	if len(tail) < 30 || len(tail) > 45 {
		return o, fmt.Errorf("invalid party create options length %d", len(tail))
	}
	o.Capacity = tail[0]
	if o.Capacity < 1 || o.Capacity > 8 {
		return o, fmt.Errorf("invalid party capacity %d", o.Capacity)
	}
	o.Info = binary.LittleEndian.Uint32(tail[1:5])
	o.Byte5 = tail[5]
	o.Word6 = binary.LittleEndian.Uint16(tail[6:8])
	o.Byte8 = tail[8]
	o.Mode = tail[9]
	o.ModeValue = binary.LittleEndian.Uint16(tail[10:12])
	copy(o.SlotFilters[:], tail[12:20])
	o.Field20 = binary.LittleEndian.Uint32(tail[20:24])
	o.Selection = binary.LittleEndian.Uint32(tail[24:28])
	o.Variant = tail[28]
	o.Extra = tail[29]
	o.LogicalBytes = 30
	return o, nil
}
