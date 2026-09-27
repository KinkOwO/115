package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD 806 REQUEST_NPC_FAVOR_OPERATION 送礼请求。
//
// 实测客户端帧（events.jsonl 捕获两条）：
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
type FavorGiftRequest struct {
	NPCID    uint32
	Reserved byte // 0x23
	Slot     byte // 礼物槽位 = 账号材料栏槽位 - 256
	Count    byte // 0x01
}

// GiftSlot 还原礼物所在的账号材料栏槽位（363..368 小晶块区）。
func (r FavorGiftRequest) GiftSlot() uint16 { return uint16(r.Slot) + 256 }

func DecodeFavorGift(p []byte) (FavorGiftRequest, error) {
	var r FavorGiftRequest
	if len(p) < 8 {
		return r, fmt.Errorf("short favor gift request (%d bytes)", len(p))
	}
	if p[0] != 0 {
		return r, fmt.Errorf("unsupported favor operation %d", p[0])
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

// FavorChangedLegacy：8 字节 NOTI194（npcID + 本次增量 delta）。
// 截图实证（奥菲利亚+200）：弹窗显示"好感度增加了{delta}"，所以这里必须传增量。
// 2026-09-27 实测：16B 形态无弹窗但稳定；NOTI733 客户端不认识（217 冻结），已废弃。
func FavorChangedLegacy(npcID, delta uint32) []byte {
	return add32(add32(nil, npcID), delta)
}

// FavorMood：NOTI195 NPC_MOOD。194/733 都无法更新客户端好感度窗口的百分比
// （窗口始终 0%），195 是剩余唯一未尝试的好感度相关通知，推测负责向客户端
// 同步 NPC 好感度状态。格式试探：npcID + point（与 194 同构）。若客户端回
// CMD217 点名 195 则立即废弃。
func FavorMood(npcID, point uint32) []byte {
	return add32(add32(nil, npcID), point)
}
