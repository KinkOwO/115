package main

import "dfolan/internal/game/protocol"

// Avatar Closet（寄存衣柜）的 C→S 命令处理：
//   - cmd 1103 ENUM_CMDPACKET_SELECT_AVATAR_CLOSET   （切换衣橱栏位/套装）
//   - cmd 1102 ENUM_CMDPACKET_CHANGE_AVATAR_CLOSET_SET（提交当前栏位号）
//
// 反汇编客户端（DFO.exe）的 opcode→handler 注册表（0x14003c650）确认：
//   - 1103 的解析器读 1 个字节（选中的栏位号）写入衣柜对象 +0x10，并打开 UI 0x225；
//   - 1102 的解析器不读字段。通用命令分发先消费一个“成功字节”，故回包体 = [0x01] + 字段。
//
// 衣柜**内容**由客户端本地持久化（服务端零存储、重登仍在）；
// 服务端只需：① 回对 ACK；② 记住**当前栏位号**（cmd1102 的 u32），
// 登录时经 noti1076 的 u32#2 回给客户端，避免它把选中栏重置回第一栏。
func (client *gameConnection) dispatchAvatarCloset(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type != 1 || !requestData.verified {
		return dispatchNext
	}
	switch requestData.frame.ID {
	case 1103:
		var selected byte
		if len(requestData.plaintext) > 0 {
			selected = requestData.plaintext[0]
		}
		if err := client.output.send(1, 1103, []byte{1, selected}); err != nil {
			client.event(map[string]any{"kind": "avatar_closet_select_error", "error": err.Error()})
			return dispatchClose
		}
		client.event(map[string]any{"kind": "avatar_closet_selected", "selected": selected})
		return dispatchHandled
	case 1102:
		if err := client.output.send(1, 1102, []byte{1}); err != nil {
			client.event(map[string]any{"kind": "avatar_closet_set_error", "error": err.Error()})
			return dispatchClose
		}
		client.event(map[string]any{"kind": "avatar_closet_set_acked"})
		return dispatchHandled
	}
	return dispatchNext
}

// avatarClosetMoveAckOnly：客户端把时装拖进衣柜走通用 cmd19（目标 List=0x21=33）。
// 只回一个成功 ACK —— **绝不发 NOTI13(space 33) 容器帧**（客户端遇到不认识的容器
// 空间号会崩；2026-10-10 事故）。衣柜内容由客户端本地持久化，服务端不落存档。
const avatarClosetProbeSpace byte = 33

func (client *gameConnection) avatarClosetMoveAckOnly(requestData *clientRequest) (bool, error) {
	if requestData.frame.ID != 19 {
		return false, nil
	}
	r, err := protocol.DecodeItemMove(requestData.plaintext)
	if err != nil || (r.SourceList != avatarClosetProbeSpace && r.DestinationList != avatarClosetProbeSpace) {
		return false, nil
	}
	if e := client.output.send(1, 19, protocol.ItemMoveSuccess(r, 1)); e != nil {
		return true, e
	}
	client.event(map[string]any{"kind": "avatar_closet_move_ack_only", "src": r.SourceList,
		"src_slot": r.SourceSlot, "dst": r.DestinationList, "dst_slot": r.DestinationSlot})
	return true, nil
}
