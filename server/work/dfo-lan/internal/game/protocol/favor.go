package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD 806 REQUEST_NPC_FAVOR_OPERATION 送礼/染色请求。
//
// p[0] 区分操作：0=送礼，1=剧情角色染色。
//
// 送礼实测客户端帧（events.jsonl 捕获两条）：
//
//	00 9d 00 00 00 23 6f 01   npc=157
//	00 79 eb f5 05 23 6c 01   npc=100284281
//
// 结构：p[0]=0 操作码，p[1:5]=u32 NPC id，p[5]=0x23 固定，
// p[6]=礼物槽位（账号材料栏槽位-256，随礼物选择变化），p[7]=0x01 固定。
//
// p[6] 实测（2026-09-27 同一 NPC 连送两次，events.jsonl）：
//
//	00 9d 00 00 00 23 6f 01 → 第一次（无色小晶块）
//	00 9d 00 00 00 23 6c 01 → 第二次（白色小晶块）
//
// 0x6f+256=367=无色小晶块槽位，0x6c+256=364=白色小晶块槽位
// （accountMaterialSlotByTemplate），与物品表 cubepiece_clear/white 双重印证。
//
// 染色(p[0]=1)：客户端选完染色后发送。实测帧：
//
//	01 79 eb f5 05 01 00 11   npc=100002681, count=1, (slot=0, color=0x11)
//
// 结构：p[0]=1 操作码，p[1:5]=u32 NPC id，p[5]=颜色对数量 count，
// 随后 count 组 (slot u8, color u8)。服务端用 FavorDyeAck 原样回显。
type FavorGiftRequest struct {
	Operation byte // 0=送礼, 1=染色
	NPCID     uint32
	Reserved  byte // 0x23
	Slot      byte // 礼物槽位 = 账号材料栏槽位 - 256
	Count     byte // 0x01
}

// IsDye 报告是否为剧情角色染色操作。
func (r FavorGiftRequest) IsDye() bool { return r.Operation == 1 }

// GiftSlot 还原礼物所在的账号材料栏槽位（363..368 小晶块区）。
func (r FavorGiftRequest) GiftSlot() uint16 { return uint16(r.Slot) + 256 }

func DecodeFavorGift(p []byte) (FavorGiftRequest, error) {
	var r FavorGiftRequest
	if len(p) < 8 {
		return r, fmt.Errorf("short favor gift request (%d bytes)", len(p))
	}
	r.Operation = p[0]
	if r.Operation != 0 && r.Operation != 1 {
		return r, fmt.Errorf("unsupported favor operation %d", r.Operation)
	}
	r.NPCID = binary.LittleEndian.Uint32(p[1:5])
	r.Reserved = p[5]
	r.Slot = p[6]
	r.Count = p[7]
	if r.NPCID == 0 {
		return r, fmt.Errorf("favor gift needs an npc id")
	}
	return r, nil
}

// FavorDyeAck：806 剧情角色染色成功 ACK。
// 逆向 DFO.exe CMD806 响应 parser（VA 0x14528f570，2026-09-29 定论）：
// 网络框架先消费 byte0=result（dl=1 才解析），parser 从 byte1 读操作码——
// byte1=1 走染色分支：readInt32 npcID、readByte count，再逐对
// readByte(slot)/readByte(color) 调本地染色函数并提交刷新，该分支
// 完全不读好感度字段、不弹好感度窗；byte1=0 才走送礼分支弹点数窗。
// 故染色应答 = 0x01(result) + 染色请求体原样回显；不扣材料、不动好感度。
// 实测请求 01 79ebf505 01 00 11 → 应答 01 01 79ebf505 01 00 11（9B）。
func FavorDyeAck(request []byte) []byte {
	ack := make([]byte, 0, 1+len(request))
	ack = append(ack, 1)
	ack = append(ack, request...)
	return ack
}

// FavorGiftAckV5：806 送礼成功 ACK。
// 2026-09-27 日志+截图实锤：弹窗"好感度增加了X"的 X 来自 ack 偏移 6。
// 此前偏移 6 放 level，导致无色(level=0)显示+0、红色(level=1)显示+1。
// 布局：[成功@0, 0@1, npcID@2(4B), delta@6(2B LE), point@8(4B)] 共 16B。
// 客户端从 @6 读增量显示弹窗，从 @8 读总点数更新百分比。
func FavorGiftAckV5(npcID, point, delta uint32) []byte {
	ack := make([]byte, 16)
	ack[0] = 1
	binary.LittleEndian.PutUint32(ack[2:], npcID)
	binary.LittleEndian.PutUint16(ack[6:], uint16(delta))
	binary.LittleEndian.PutUint32(ack[8:], point)
	return ack
}

// FavorPointInfoRecord 是 NOTI733 全量好感度列表中的一条 NPC 记录。
type FavorPointInfoRecord struct {
	NPCID uint32
	Point uint32
	Flag  byte // 节点偏移4 的标志字节，与 806 ack 末字节同源；当前恒 0
}

// FavorPointInfo 编码 NOTI733 ENUM_NOTIPACKET_NPC_FAVOR_POINT_INFO——
// 进城镇好感度全量同步（逆向客户端 handler VA 0x1452db190，2026-09-29 定论）：
//
//	count u8
//	repeat count:
//	    npcID      u32 LE
//	    point      u32 LE   （好感度总点数，驱动百分比/染色解锁）
//	    flag       u8      （节点+4 标志，806 ack 也写它，发 0）
//	    colorCount u8      （随后 (slot u8, color u8) 颜色对数量，发 0）
//
// handler 先清空 favor map（0x14436c680 纯释放节点，无 UI 副作用）再逐条
// 插入并提交刷新，全程不派发任何 UI 事件、不弹窗——与 806 送礼 ack "写缓存
// 必弹好感度窗" 不同，这是无弹窗的全量装载入口。count 为 u8，上限 255。
//
// 历史教训：2026-09-27 曾发"npcID+point 共 8B"的 733 试探包，客户端按
// count 列表解析，首字节被当成 count（上百条）后读越界触发 217 冻结，
// 当时误判为"客户端不认识 733"。实际是格式错误，枚举名与 handler 俱在。
func FavorPointInfo(records []FavorPointInfoRecord) []byte {
	n := len(records)
	if n > 255 {
		n = 255
	}
	buf := make([]byte, 0, 1+10*n)
	buf = append(buf, byte(n))
	for i := 0; i < n; i++ {
		r := records[i]
		buf = binary.LittleEndian.AppendUint32(buf, r.NPCID)
		buf = binary.LittleEndian.AppendUint32(buf, r.Point)
		buf = append(buf, r.Flag, 0) // flag, colorCount
	}
	return buf
}

// FavorChangedLegacy：8 字节 NOTI194（npcID + 本次增量 delta）。
// 截图实证（奥菲利亚+200）：弹窗显示"好感度增加了{delta}"，所以这里必须传增量。
// NOTI194 仅弹增量提示，不更新好感度窗口百分比（2026-09-27 实测）；进城镇
// 全量同步走 NOTI733（见 FavorPointInfo）。
func FavorChangedLegacy(npcID, delta uint32) []byte {
	return add32(add32(nil, npcID), delta)
}

// FavorMood：NOTI195 NPC_MOOD。实测（2026-09-27/29）不更新好感度窗口百分比，
// 任何时序都无效；进城镇全量同步已由 NOTI733（FavorPointInfo）解决。保留
// 编码仅供送礼链路沿用，勿再用于入场同步。
func FavorMood(npcID, point uint32) []byte {
	return add32(add32(nil, npcID), point)
}
