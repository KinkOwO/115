package protocol

import "fmt"

// EvildomPartyType 是次元回廊（Evildom）待机区的队伍类型字节。
//
// 军团家族各内容自带队伍类型，互不相认：末世录 0x26 / 伊斯 0x0b /
// 维纳斯 0x22 / 苏醒之森 0x18·0x19 / **次元回廊 0x0d**。
// 官服抓包 official_20261009-223033_live（session_s30 s2c #435）实证。
const EvildomPartyType byte = 0x0d

// EvildomPartyReply 构造次元回廊待机区建队应答（单帧 NOTI9）。
//
// ★ 复用**本仓已经修好的那条路**：`legionStandbyPartyReply` 是 2.38.2 客户端的
// 原生 NOTI9 语法（107+len(name) 字节，黑鸦/月环/伊斯/森林都在用），本族只换
// 队伍类型字节。
//
// 为什么**不**照官服抓包的 176 字节 N9 原文回放（2026-10-10 两轮实机结论）：
//
//	官服那份是**官服新版客户端布局**；本机客户端是 2.38.2 读取器，解析它会读错
//	字段（ispins_party.go 记录过同一坑：官服 208B 帧的成员数槽位 0x69=105 会让
//	2.38.2 越界解析 → 卡死 + 闪退）。两次照官服字节重排（变长平移、定长窗口）
//	都表现为「输入队名点确认没反应」，而服务端日志里明明有 party_created。
//	判据只能是「客户端界面有没有动」。
//
// 语义字段取自官服 N9（#435）：队名、容量 4、队伍类型 0x0d、模式 1。
func EvildomPartyReply(name []byte, actor uint16, channel [2]byte, capacity byte) ([]byte, error) {
	if capacity < 1 || capacity > 4 {
		return nil, fmt.Errorf("次元回廊队伍人数必须为 1 至 4 人，收到 %d", capacity)
	}
	return legionStandbyPartyReply(name, actor, channel, EvildomPartyType)
}
