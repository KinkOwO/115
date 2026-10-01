package protocol

import (
	"encoding/binary"
	"fmt"
)

// 选角背景的选择/恢复报文体例已移回 internal/rosterbg（见 rosterbg/wire.go）；
// 本文件只保留不依赖领域中立的背景券 CMD507 布局。

const RosterBackgroundTicketAction uint32 = 209

// DecodeRosterBackgroundTicket 对应2026-09-30实机CMD507：普通背包、动作209，其余参数为0。
func DecodeRosterBackgroundTicket(p []byte) (uint16, error) {
	slot, action, err := DecodeStackableAction(p)
	if err != nil {
		return 0, err
	}
	if p[2] != 0 || action != RosterBackgroundTicketAction {
		return 0, fmt.Errorf("选角背景券使用参数无效")
	}
	return slot, nil
}

// 1459350C0成功分支：取u16槽、u8动作、u32动作码209；分支145936236提示附加成功。
// 原网游执行后至多再回一个客户端快照，因此本包只同步“已快照”，不再发成功包执行。
func RosterBackgroundTicketSuccess(slot uint16) []byte {
	p := make([]byte, 8)
	p[0] = 1
	binary.LittleEndian.PutUint16(p[1:], slot)
	binary.LittleEndian.PutUint32(p[4:], RosterBackgroundTicketAction)
	return p
}
