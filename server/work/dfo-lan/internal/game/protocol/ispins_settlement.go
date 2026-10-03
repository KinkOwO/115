package protocol

// Historical newer-official template, retained only as investigation evidence.
// Deprecated: local 2.38.2 equipment reader rejects this grammar (next79 §30
// live stack 14563990b/146d77f77). Production settlement must use the persisted
// character.Service EntryBasicProbe/EntryAddition native encoders instead.

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// [ISPINS-SETTLEMENT-N2] next79 §27（2026-10-03 八测）：官服阶段结算链尾段
// （s4 帧 488-496）以一帧 N2（op=2，480B）开头，客户端收到该段后 10ms 才
// 发 CMD1654 进结算面板；私服链缺这一帧时客户端改发官服不存在的 op=2060，
// 1.2s 后 op=682 闪退（八测实证）。官服帧两处内嵌会话角色名 yan55 与官服
// actor 0x00ed（§21.5 verbatim 回放=本地 actor 表查无此人→闪退先例），必须
// 本地构造。op=2 行读取器为游标式顺序解析（docs/protocol/entry-userinfo.md
// “307 + UTF-8 name bytes”，行宽随名字长度变化），故按本地名字长度重排名
// 后字段；模板取自官服 s4 帧 488，两处名字块与第二块的 actor 槽（模板偏移
// 165）替换为本地值，其余字节 verbatim。
//
// 模板内第二名字块后紧跟迷你角色行：+0 职业、+1 转职、+2 等级（0x73=115）、
// +3 PvP、+4 状态（结算=01，回待机版帧 507 为 00，模板自带 01 无需再改）。
// 四阶段帧（488/583/666/772）除模板偏移 416..419 外逐字节一致：2 字节
// per-stage nonce + u16 阶段进度（3/7/11/15 = 4*stage+3），按阶段回放。
const ispinsSettlementTemplateHex = "000100035600000004000000ffffffffffffffff00000000000000000500000079616e3535000000000000000000000000204e000050c300000000010101010100010100010101000001aca90e0218000000000000fc00000000000000000000000000000000000000000000000000000000000000ffffffff00000000ff0000000000000075e500000000000060ea000000000000e8010000000000000000000000000000ed000500000079616e353510357300010400dcab0107040000000000000000000000000000000000000000000000000000000000000000000915aae51d040000000000000000000000000000000000000000000000000000000000000000000c05e4f906040000000000000000000000000000000000000000000000000000000000000000000d4671d21d04000000000000000000000000000000000000000000000000000000005bdc9a0000000000000000000000b0040000000000000000000000008384dc1d0000000001000000000000000000000000000100000000000000000001000000000000000000000000000000006400020000000f800300000000000300000000641000010000000000ff0101000000000000000000000000000000000000000000000001000000000000000000000000000000"

var ispinsSettlementTemplate = mustSettlementTemplate(ispinsSettlementTemplateHex)

var ispinsSettlementStagePatch = [4][4]byte{
	{0x0f, 0x80, 0x03, 0x00}, // stage0 (s4 frame 488)
	{0xbe, 0x9e, 0x07, 0x00}, // stage1 (s4 frame 583)
	{0x6d, 0xbd, 0x0b, 0x00}, // stage2 (s4 frame 666)
	{0x1c, 0xdc, 0x0f, 0x00}, // stage3 (s4 frame 772)
}

func mustSettlementTemplate(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic("ispins settlement template: " + err.Error())
	}
	if len(b) != 480 {
		panic(fmt.Sprintf("ispins settlement template: %d bytes, want 480", len(b)))
	}
	return b
}

// IspinsSettlementCharacterInfo 构造结算链尾段的 N2（官服 s4 帧 488 族）：
// 官服 480B 模板 + 本地角色名/actor 重排 + 每阶段 4 字节补丁。名字长度与
// 官服（5 字节 yan55）不同会平移名字后字段，游标式读取器按顺序消费即可。
func IspinsSettlementCharacterInfo(name string, actor uint16, stage int) ([]byte, error) {
	if len(name) < 1 || len(name) > 63 {
		return nil, fmt.Errorf("结算角色名无效")
	}
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("结算角色 actor 无效")
	}
	if stage < 0 || stage > 3 {
		return nil, fmt.Errorf("伊斯阶段号 %d 越界", stage)
	}
	tmpl := ispinsSettlementTemplate
	p := make([]byte, 0, len(tmpl)+2*len(name))
	p = append(p, tmpl[:28]...)
	p = binary.LittleEndian.AppendUint32(p, uint32(len(name)))
	p = append(p, name...)
	p = append(p, tmpl[37:165]...)
	p = binary.LittleEndian.AppendUint16(p, actor)
	p = binary.LittleEndian.AppendUint32(p, uint32(len(name)))
	p = append(p, name...)
	// 模板偏移 176 起 = 第二名字块后的迷你角色行（职业/转职/等级/状态 01）。
	p = append(p, tmpl[176:]...)
	// 官服模板偏移 416..419 是四阶段间唯一差异（nonce + 阶段进度），本地
	// 名字长度与官服（5）的差量把该区平移 2*(5-len(name))。
	patchAt := 416 - 2*(5-len(name))
	if patchAt < 0 || patchAt+4 > len(p) {
		return nil, fmt.Errorf("结算 N2 阶段补丁越界")
	}
	copy(p[patchAt:patchAt+4], ispinsSettlementStagePatch[stage][:])
	return p, nil
}
