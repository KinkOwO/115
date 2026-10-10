package main

// Avatar Closet（寄存衣柜）的 C→S 命令处理：
//   - cmd 1103 ENUM_CMDPACKET_SELECT_AVATAR_CLOSET   （点 select，选背包里的时装）
//   - cmd 1102 ENUM_CMDPACKET_CHANGE_AVATAR_CLOSET_SET（提交衣柜搭配）
//
// 反汇编客户端（DFO.exe）的 opcode→handler 注册表（0x14003c650）确认：
//   - 1103 的解析器读 1 个字节（选中的套装号）写入衣柜对象 +0x10，并打开 UI 0x225；
//   - 1102 的解析器不读字段，只把衣柜对象 +8 清 0。
// 通用命令分发会先消费一个“成功字节”，故回包体 = [0x01] + 各自字段：
//   - 1103 → [0x01, 选中的字节]
//   - 1102 → [0x01]
//
// ⚠️ 这是从反汇编**推断**的实现，尚未见过官方样本（待业主实测验证）。
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
