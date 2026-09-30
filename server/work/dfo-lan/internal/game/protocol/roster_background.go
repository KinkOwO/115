package protocol

import (
	"dfolan/internal/rosterbg"
	"encoding/binary"
	"fmt"
)

type SelectRosterBackgroundRequest struct {
	Background rosterbg.Background
	Page       uint16
}

// DecodeSelectRosterBackground 对应原生 140232880 的 CMD1725：u8 类别、u16 编号、u16 页号。
func DecodeSelectRosterBackground(p []byte) (SelectRosterBackgroundRequest, error) {
	var r SelectRosterBackgroundRequest
	if len(p) < 5 {
		return r, fmt.Errorf("选角背景请求长度不足")
	}
	// CMD1725 使用槽 3，按 16 字节补齐；2026-09-30 实机请求的 5 字节字段后有 11 字节尾零。
	if err := padding(p[5:], 16); err != nil {
		return r, err
	}
	r.Background = rosterbg.Background{Category: p[0], ID: binary.LittleEndian.Uint16(p[1:])}
	r.Page = binary.LittleEndian.Uint16(p[3:])
	if r.Page >= rosterbg.Pages || !r.Background.Valid() {
		return r, fmt.Errorf("选角背景类别、编号或页号无效")
	}
	return r, nil
}

// RosterBackgroundRestore 对应 NOTI1759 的原生 reader 1452E2C30。
// 五组选择交错排列，随后是原生标志和拥有数，再跟每条 u8/u16/u32。
// 基础背景由 1401FDF60 从 PVF 初始化；这里只传已解锁特殊背景。
func RosterBackgroundRestore(state rosterbg.State) ([]byte, error) {
	if err := state.Validate(); err != nil {
		return nil, err
	}
	p := make([]byte, 17+7*len(state.Owned))
	for page, b := range state.Selected {
		p[page*3] = b.Category
		binary.LittleEndian.PutUint16(p[page*3+1:], b.ID)
	}
	// 原生标志的业务语义尚未闭合，保持默认 0，不挪用为其它设置。
	p[16] = byte(len(state.Owned))
	for i, b := range state.Owned {
		at := 17 + 7*i
		p[at] = b.Category
		binary.LittleEndian.PutUint16(p[at+1:], b.ID)
		binary.LittleEndian.PutUint32(p[at+3:], b.ExpiresAt)
	}
	return p, nil
}

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

// 1459350C0成功分支读取u16槽、u8容器、u32动作；209分支在145936236显示添加成功。
// 原生回执会消耗一次客户端库存，所以重放只同步绝对库存，不再发送成功回执。
func RosterBackgroundTicketSuccess(slot uint16) []byte {
	p := make([]byte, 8)
	p[0] = 1
	binary.LittleEndian.PutUint16(p[1:], slot)
	binary.LittleEndian.PutUint32(p[4:], RosterBackgroundTicketAction)
	return p
}
