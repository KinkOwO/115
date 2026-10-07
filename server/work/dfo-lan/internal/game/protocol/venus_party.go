package protocol

// 维纳斯待机区（频道 Type 99）「创建队伍」CMD12 的编解码。
//
// 根因与伊斯待机区完全同型（next79 §21 的频道 99 复现）：点「确定」后
// 客户端发出 CMD12，服务端此前无任何处理器应答，UI 毫无反应。
// 实机证据：2026-10-04 10:03 会话
// roles_persist_select_actor_..._20261004_100335_903957_next37 的
// events.jsonl 两次 id=12 client_frame（02:16:25 / 02:16:59，玩家两次点击），
// 48B 明文与伊斯官服 s4 帧 125 同构，仅两处不同：
//   - 队名「1」（1 字节）；
//   - 队伍类型字节 0x22 (34)，伊斯为 0x0b (11)。
// 0x22 与协议台账（包规格-全流程/10-军团末世录专有/2043-VENUS_START.md）
// 记录的「频道99／内容106／mode34／route44」互相印证。
// 应答沿伊斯先例（protocol.IspinsStandbyPartyReply）：2.38.2 原生黑鸦族
// NOTI9 语法承载维纳斯类型 0x22，前置队长资料两个 op=2（成员列表显示
// 数据源，黑鸦/伊斯同款）。

// DecodeVenusStandbyParty 解码维纳斯待机区队伍对话框发送的 CMD12，
// 只接受队伍类型 0x22（34）的 4 人军团普通队伍。
func DecodeVenusStandbyParty(p []byte) ([]byte, error) {
	return decodeLegionStandbyParty(p, 0x22)
}

// VenusStandbyPartyReply 按 2.38.2 客户端原生 NOTI9 语法（黑鸦族
// 1452F2620 写入器布局）构造维纳斯待机区建队应答：容量 4、队伍类型
// 0x22、模式 1。语法层的闪退陷阱与字段依据见 ispins_party.go 的
// IspinsStandbyPartyReply 注释；类型字节取自本频道实机请求。
func VenusStandbyPartyReply(name []byte, actor uint16, channel [2]byte) ([]byte, error) {
	return legionStandbyPartyReply(name, actor, channel, 0x22)
}
