package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// 军团待机区「创建队伍」CMD12 的共用编解码（next79 §21）。
//
// 各内容（伊斯 0x0b、维纳斯 0x22……）的待机区对话框发出同构请求：
// [u16 0][u32 名长][队名][u32 容量][5 零][u8 队伍类型][u16 模式]
// [槽过滤 8B][ff×4][零尾]，仅队伍类型字节随内容不同；应答都是单帧
// NOTI9，用 2.38.2 原生黑鸦族语法（1452F2620 写入器布局）承载各内容
// 自己的类型值。伊斯侧的字节契约与官服对照证据见 ispins_party.go；
// 维纳斯侧的实机请求向量见 venus_party.go。

// decodeLegionStandbyParty 解码军团待机区建队 CMD12，并校验目标内容的
// 队伍类型字节。容量只接受 4 人——目前各内容的实机样本都是 4 人
// （军团普通队伍默认容量），其它取值等有实机向量再放开。
func decodeLegionStandbyParty(p []byte, partyType byte) ([]byte, error) {
	if len(p) < 36 {
		return nil, fmt.Errorf("军团待机区建队请求不完整")
	}
	n := int(binary.LittleEndian.Uint32(p[2:]))
	if n < 1 || n > 63 || n > len(p)-36 {
		return nil, fmt.Errorf("队伍名称长度无效")
	}
	at := 6 + n
	if binary.LittleEndian.Uint32(p[at:]) != 4 {
		return nil, fmt.Errorf("军团待机区只支持4人队伍")
	}
	if p[at+9] != partyType || binary.LittleEndian.Uint16(p[at+10:]) != 1 {
		return nil, fmt.Errorf("仅支持创建该内容的军团普通队伍")
	}
	name := p[6:at]
	if bytes.IndexByte(name, 0) >= 0 {
		return nil, fmt.Errorf("队伍名称含无效字符")
	}
	return bytes.Clone(name), nil
}

// legionStandbyPartyReply 按 2.38.2 客户端原生 NOTI9 语法构造建队应答，
// 队伍类型/模式等语义字段由调用内容给定。
func legionStandbyPartyReply(name []byte, actor uint16, channel [2]byte, partyType byte) ([]byte, error) {
	if len(name) < 1 || len(name) > 63 {
		return nil, fmt.Errorf("队伍名称无效")
	}
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("队伍成员无效")
	}
	if channel[0] == 0 && channel[1] == 0 {
		return nil, fmt.Errorf("待机区频道上下文缺失")
	}
	p := make([]byte, 107+len(name))
	binary.LittleEndian.PutUint16(p, 1)
	binary.LittleEndian.PutUint16(p[2:], 9999)
	// 成员数=1：这是 2.38.2 读取器的成员计数槽，官服新版帧在此放 0x69=105，
	// verbatim 回放即触发越界解析闪退。
	binary.LittleEndian.PutUint16(p[4:], 1)
	copy(p[6:8], channel[:])
	binary.LittleEndian.PutUint32(p[14:], uint32(len(name)))
	copy(p[18:], name)
	q := p[len(name):]
	q[20] = 4                                // 容量：4 人军团队
	q[25] = 5                                // 普通难度，同黑鸦原生建队发送器
	q[29], q[95] = partyType, partyType      // 队伍类型（各内容自带）
	binary.LittleEndian.PutUint16(q[30:], 1) // 模式 1，同 CMD12 请求
	copy(q[46:54], []byte{1, 1, 2, 4, 7, 7, 7, 7})
	q[69] = 1
	binary.LittleEndian.PutUint16(q[71:], actor)
	// 末尾扩展类型 0：不更新各玩法次数（同黑鸦结论，不借用沉月湖的 41）。
	return p, nil
}
