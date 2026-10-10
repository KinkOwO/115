package protocol

// 苏醒之森（Forest of Awakening，频道 Type 96）待机区「创建队伍」CMD12 的编解码。
//
// 根因与伊斯/维纳斯待机区完全同型：点「Register」后客户端发出 CMD12，
// 服务端此前无任何处理器应答，UI 毫无反应。实机证据：2026-10-05 09:42 会话
// roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261005_174114_578397_next37
// 的 events.jsonl 两次 id=12 client_frame（09:42:27 队名「33333」、
// 09:46:57 队名「222」，与对话框输入一致），48B 明文与伊斯官服 s4 帧 125
// 同构，仅队伍类型字节不同：0x18 (24)——伊斯 0x0b (11)、维纳斯 0x22 (34)、
// 末世录 0x26 (38)。同一会话四频道互证。
// 第九轮补充：Extreme（ForestOfAwakeningHard）建队的类型字节为 **0x19 (25)**
// （21:33 会话 13:44:33 实机两次，其余字节与 Normal 逐字节同构；此前只认
// 0x18，拒绝码 8 被客户端映射为「You have reached daily entrance limit」弹窗）。
// 应答沿伊斯/维纳斯先例（protocol.VenusStandbyPartyReply）：2.38.2 原生
// 黑鸦族 NOTI9 语法承载请求自带的类型字节，前置队长资料两个 op=2
// （成员列表显示数据源，黑鸦/伊斯/维纳斯同款）。

const (
	// ForestPartyTypeNormal / ForestPartyTypeHard 是 CMD12 的队伍类型字节：
	// 24 = ForestOfAwakeningNormal（普通），25 = ForestOfAwakeningHard（Extreme）。
	ForestPartyTypeNormal byte = 0x18
	ForestPartyTypeHard   byte = 0x19
)

// ForestStandbyPartyRequest is a decoded standby CMD12: the party name and
// which mode the dialog had selected (Normal / Extreme).
type ForestStandbyPartyRequest struct {
	Name []byte
	Hard bool
}

// DecodeForestStandbyParty 解码苏醒之森待机区队伍对话框发送的 CMD12，
// 接受队伍类型 0x18（Normal）/ 0x19（Extreme）的 4 人军团普通队伍。
func DecodeForestStandbyParty(p []byte) (ForestStandbyPartyRequest, error) {
	var r ForestStandbyPartyRequest
	name, err := decodeLegionStandbyParty(p, ForestPartyTypeNormal)
	if err == nil {
		r.Name = name
		return r, nil
	}
	name, err = decodeLegionStandbyParty(p, ForestPartyTypeHard)
	if err != nil {
		return r, err
	}
	r.Name, r.Hard = name, true
	return r, nil
}

// ForestStandbyPartyReply 按 2.38.2 客户端原生 NOTI9 语法（黑鸦族
// 1452F2620 写入器布局）构造苏醒之森待机区建队应答：容量 4、模式 1，
// 队伍类型回显请求自带的 0x18/0x19。语法层的闪退陷阱与字段依据见
// ispins_party.go 的 IspinsStandbyPartyReply 注释。
//
// Extreme（0x19）额外把 q[55] 置 1：官服 2026-10-08 抓包的两份建队应答
// （Normal 208B / Extreme 208B）逐字节对齐后，**只有两处不同** ——
// 队伍类型字节（0x18/0x19）与 q[55]（0/1）。q[55] 的语义未定（疑似
// 「困难模式」标志或等待区序号），但它是官服 Extreme 队伍的既定形状，
// 按原样回放。
func ForestStandbyPartyReply(name []byte, actor uint16, channel [2]byte, hard bool) ([]byte, error) {
	partyType := ForestPartyTypeNormal
	if hard {
		partyType = ForestPartyTypeHard
	}
	body, err := legionStandbyPartyReply(name, actor, channel, partyType)
	if err != nil {
		return nil, err
	}
	if hard && len(body) > len(name)+55 {
		body[len(name)+55] = 1
	}
	return body, nil
}
