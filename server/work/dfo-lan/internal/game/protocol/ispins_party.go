package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// next79 §21（2026-10-03）：伊斯待机区「创建队伍」点确定无反应的根因是
// CMD12 无应答。官服 10-02 s4 抓包实证（tshark 包时间戳对齐，op=12 之后
// 0.5s 内的唯一下行）：应答为单帧 NOTI9，内嵌队名「111」、party_id=9999
//（与黑鸦/月环一致）、容量 4、队伍类型 0x0b、模式 1。客户端此后直接点
// 开战（CMD2043），全程无其它队伍握手帧。
// 注意：官服帧本体（208B）是官服新版客户端布局，2.38.2 私服客户端解析
// 它会按 105 个成员越界读 → 卡死 + 闪退（见 IspinsStandbyPartyReply），
// 因此不能用 verbatim 模板，只借鉴其语义字段。

// DecodeIspinsStandbyParty 解码伊斯待机区队伍对话框发送的 CMD12。
// 私服客户端实测与官服 s4 帧 125 逐字节一致（48B）：
// [u16 0][u32 名长][队名][u32 容量=4][5 零][u8 队伍类型 0x0b]
// [u16 模式 1][1,1,2,4][7×4][ff×4][零尾]。
func DecodeIspinsStandbyParty(p []byte) ([]byte, error) {
	if len(p) < 36 {
		return nil, fmt.Errorf("伊斯待机区建队请求不完整")
	}
	n := int(binary.LittleEndian.Uint32(p[2:]))
	if n < 1 || n > 63 || n > len(p)-36 {
		return nil, fmt.Errorf("伊斯队伍名称长度无效")
	}
	at := 6 + n
	if binary.LittleEndian.Uint32(p[at:]) != 4 {
		return nil, fmt.Errorf("伊斯待机区只支持4人队伍")
	}
	if p[at+9] != 0x0b || binary.LittleEndian.Uint16(p[at+10:]) != 1 {
		return nil, fmt.Errorf("仅支持创建军团普通队伍")
	}
	name := p[6:at]
	if bytes.IndexByte(name, 0) >= 0 {
		return nil, fmt.Errorf("伊斯队伍名称含无效字符")
	}
	return bytes.Clone(name), nil
}

// IspinsStandbyPartyReply 按 2.38.2 客户端原生 NOTI9 语法（黑鸦
// BlackPurgatorySoloParty / 月环 MoonSoloParty115 同族，1452F2620 写入器
// 布局）构造伊斯待机区建队应答。
//
// 2026-10-03 两次闪退实证（06:32 / 06:41，后者 actor 已本地化仍崩）：
// 官服 10-02 抓包的 208B 帧是官服新版客户端布局，其 p[4:6]=0x69(105) 落在
// 2.38.2 读取器的成员数槽位（黑鸦/月环语法此位=1）——客户端按 105 个成员
// 解析 208B，越界读出垃圾长度 → ReadLengthPrefixedBlob 巨量拷贝（卡死）
// → 空指针闪退（CrashDump 0xc0000005 @0x0，收帧 1.2s 后 op=682）。
// 必须用本客户端实证可解析的黑鸦族语法承载伊斯语义：
// 容量 4、队伍类型 0x0b（军团普通队伍，客户端请求自带）、模式 1。
func IspinsStandbyPartyReply(name []byte, actor uint16, channel [2]byte) ([]byte, error) {
	if len(name) < 1 || len(name) > 63 {
		return nil, fmt.Errorf("伊斯队伍名称无效")
	}
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("伊斯队伍成员无效")
	}
	if channel[0] == 0 && channel[1] == 0 {
		return nil, fmt.Errorf("伊斯待机区频道上下文缺失")
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
	q[20] = 4                              // 容量：伊斯 4 人军团队
	q[25] = 5                              // 普通难度，同黑鸦原生建队发送器
	q[29], q[95] = 0x0b, 0x0b              // 队伍类型 11 = 军团普通队伍
	binary.LittleEndian.PutUint16(q[30:], 1) // 模式 1，同 CMD12 请求
	copy(q[46:54], []byte{1, 1, 2, 4, 7, 7, 7, 7})
	q[69] = 1
	binary.LittleEndian.PutUint16(q[71:], actor)
	// 末尾扩展类型 0：不更新各玩法次数（同黑鸦结论，不借用沉月湖的 41）。
	return p, nil
}
