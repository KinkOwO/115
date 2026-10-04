package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"fmt"
	"os"
)

// 装备技能栏 / 冷却提醒 / 自定义按键 的持久化（C2S 2254/2256/2257，S2C 2609）。
//
// 事实来源：外部文档 `equipment-skill-persistence-20260929.md`（作者侧称已实机确认：
// 重登后三处设置都保留）。本仓此前**完全没有**这一族处理（既没解析 2254/2256/2257，
// 也没在登录时发 2609）⇒ 客户端只能保住本会话内的配置。
//
//	2254 = 40×i16 装备技能槽位快照（96B 正文）
//	2256 = 10×8B 自定义指令组快照（96B 正文）
//	2257 = 清空该角色的两组设置
//	2609 = 角色进入游戏时恢复（S2C19 之后，80+80+8 = 168B）
//
// **请求侧的 96B 形状是本机实测**：`runtime/*/events.jsonl` 里 67 个会话各 1 帧
// `client_frame id=2256`，全部 96B、`[13]=10`、`[0:13]` 与 `[94:96]` 全零、
// `checksum_ok=true`、`type=1`（此前标着 `unimplemented_sample=true`）。
// 应答侧与 2609 恢复侧沿用外部文档的结论，**本仓未实机验证**。
//
// 应答沿用同一作者在 C2S2382 上的约定（`oath_selection_flow.go`）：
// 先落库，成功后再回 `outboundPacket{name, 1, <同一 op>, []byte{1}}`。
// 落库失败**不回**应答 —— 客户端那边就等价于这次设置没保存成功。
func equipmentSkillEnabled() bool {
	return os.Getenv("DFO_EQUIPMENT_SKILL") != "0"
}

func (w *worldSession) equipmentSkillPackets(ctx context.Context, id uint16, body []byte) ([]outboundPacket, error) {
	if w == nil || w.characters == nil || w.store == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("equipment skill requires a selected character")
	}
	switch id {
	case 2254, 2256:
		want := protocol.EquipmentSkillCount
		which := database.EquipmentSkillSnapshotColumn
		name := "equipment_skill_slots_saved"
		if id == 2256 {
			want = protocol.EquipmentCommandCount
			which = database.EquipmentCommandSnapshotColumn
			name = "equipment_skill_commands_saved"
		}
		snapshot, err := protocol.DecodeEquipmentSkillSnapshot(body, want)
		if err != nil {
			return nil, err
		}
		if err := w.store.SaveEquipmentSkillSnapshot(ctx, w.account, w.role.ID, which, snapshot); err != nil {
			return nil, err
		}
		return []outboundPacket{{name, 1, id, []byte{1}}}, nil
	case 2257:
		if err := w.store.ClearEquipmentSkill(ctx, w.account, w.role.ID); err != nil {
			return nil, err
		}
		return []outboundPacket{{"equipment_skill_cleared", 1, id, []byte{1}}}, nil
	}
	return nil, fmt.Errorf("unsupported equipment skill request %d", id)
}
