package protocol

// 末世录待机区（频道 Type 119 / 内容 107）「创建队伍」CMD12 的编解码。
//
// 与维纳斯（0x22=34）、伊斯（0x0b=11）同构，只有队伍类型字节不同：
// 末世录是 0x26（38），即本包里的 ApocalypsePartyMode115，与
// `contents/system/legionsystem/legionsystem.cos` 登记的
// online119/route87/content107/mode38 一致（见 legion_party115.go 的注释）。
//
// 请求样本（抓包 D:\zhuabao\captures\20261008-105227 的 CMD12，61B）：
//
//	0000040000003232323104000000 0000000000 26 0100 01010204 07070707 ffffffff 0000...
//	└─ u16 0 ─┴ u32 名长=4 ┴ 队名"2221" ┴ u32 容量=4 ┴5 零┴类型┴模式=1┴槽过滤┴…
//
// 也就是说玩家是在**城镇**用普通建队对话框发的这条 CMD12（不是待机区专属
// 对话框）：它落进通用队伍链路，由 CMD9 PARTY_INFO 应答。本文件的解码器是
// 为了在待机区（navigationroom_village 区域）点创建队伍时也能接管——那里的
// 请求形状与城镇相同，只是服务端此前没有对应处理器（见
// D:\115US-001\rz\001 修复日志第 1184 行的遗留事项）。

// DecodeApocalypseStandbyParty 解码末世录待机区队伍对话框发送的 CMD12，
// 只接受队伍类型 0x26（38）的 4 人军团普通队伍。
func DecodeApocalypseStandbyParty(p []byte) ([]byte, error) {
	return decodeLegionStandbyParty(p, ApocalypsePartyMode115)
}

// ApocalypseStandbyPartyReply 按 2.38.2 客户端原生 NOTI9 语法（黑鸦族
// 1452F2620 写入器布局）构造末世录待机区建队应答：容量 4、队伍类型 0x26、
// 模式 1。语法层的闪退陷阱与字段依据见 ispins_party.go 的
// IspinsStandbyPartyReply 注释；类型字节取自 ApocalypsePartyMode115。
func ApocalypseStandbyPartyReply(name []byte, actor uint16, channel [2]byte) ([]byte, error) {
	return legionStandbyPartyReply(name, actor, channel, ApocalypsePartyMode115)
}
