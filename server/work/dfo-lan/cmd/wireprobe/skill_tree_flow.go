package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"time"
)

// changeAnotherSkillTree 处理 CMD260 ENUM_CMDPACKET_CHANGE_ANOTHER_SKILL_TREE，
// 也就是技能窗口右下角那个"技能类型替换按钮"。
//
// 115 实机明文（2026-10-09 17:51:09，业主点按钮，events.jsonl 的 client_frame）：
//
//	00 3e 74 5e 7a 00 00 00
//
// body[0] 是客户端认为的当前技能类型 wire 索引（0 = 技能类型 1），与服务端存档一致；
// 其余字节在 115 上没有证据，原样回显不做解释。服务端**以存档为准**取当前页
// （86JP SkillHandler 同样先用 repo.LoadSkillTreeIndex 覆盖客户端上报值）。
//
// 应答照 86JP SkillHandler.BuildChangeAnotherSkillTreeAck 的形状：body[0] 置 1
// （成功标志 —— 与 115 收包分发器"body[0] 当成功标志"的既有约定一致，见
// protocol.OpenSkinSlotReply 的注释），body[1] 覆盖为服务端确认的新索引，其余
// 原样回显。
func (w *worldSession) changeAnotherSkillTree(p []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.characters == nil {
		return nil, fmt.Errorf("skill tree switch before character selection")
	}
	if len(p) == 0 {
		return nil, fmt.Errorf("short skill tree switch request")
	}
	var st character.State
	if e := json.Unmarshal(w.role.State, &st); e != nil {
		return nil, e
	}
	acked := append([]byte{1}, p...)
	current := character.SkillTreeWireIndex(st)
	if current > 1 {
		// 未解锁：回锁定值并保持原状态（86JP locked 分支）。
		acked[1] = protocol.SkillTreeLocked
		return []outboundPacket{{"skill_tree_switch_locked", 1, 260, acked}}, nil
	}
	// ★ 幂等不能拿 body 当键：实机（2026-10-09 17:58 会话）证明客户端在同一个
	// 会话里**复用同一个 body** —— 切到第二页发 `003e745e7a000000`，切回第一页发
	// `01b2ce8d59000000`，反复点击就是这两个包交替。用 body 哈希做 key 会让第二次
	// 点击变成"重放"而不生效，服务端与客户端状态就此错开（正是业主报的"切不回第一页"）。
	// 正确判据是**客户端上报的当前页 vs 服务端存档**：一致 = 新点击，执行切换；
	// 不一致 = 重发或状态漂移，只回包纠正、不动存档。
	next := byte(1)
	if current == 1 {
		next = 0
	}
	reported := p[0]
	if reported <= 1 && reported != current {
		acked[1] = current
		return []outboundPacket{{"skill_tree_switch_corrected", 1, 260, acked}}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// 每次真实切换都用新 key（不再依赖 body），重发由上面的 reported 判据挡住。
	key := fmt.Sprintf("skill-tree-switch:%d:%d", w.role.ID, time.Now().UnixNano())
	saved, applied, e := w.characters.SetSkillTreeType(ctx, w.role, key, next+1)
	if e != nil {
		return nil, e
	}
	saved.WireID = w.role.WireID
	w.role = saved
	acked[1] = next
	name := "skill_tree_switched"
	if !applied {
		name = "skill_tree_switch_replayed"
	}
	plan := []outboundPacket{{name, 1, 260, acked}}
	// 只回 ACK 的话页签切了、内容还是上一页的：客户端不会在切页后自己重新拉
	// 进化／突破，面板就停在上一页的配置上（业主 2026-10-10 00:04 反馈）。
	// 这里补一帧 id=29（当前存档 = 切换后的那一页的 VP），与公共出口
	// skillMutationResponsePlan 的做法一致。
	//
	// ⚠️ **故意不发 NOTI19**：NOTI19 会让客户端重建技能窗口并把当前页重置为 0
	// （2026-10-09 多轮实测的既有行为），那等于把刚切好的页签又打回第一页。
	// VP 帧不是角色重建帧，没有 mode0/mode1 与 S→C CMD260 那两次 0xC0000005 的风险。
	variation, e := w.characters.VariationRestore(saved)
	if e != nil {
		return nil, e
	}
	if len(variation) > 0 {
		plan = append(plan, outboundPacket{"skill_variation_response", 1, 29, variation})
	}
	return plan, nil
}

// ⛔⛔ 不要再往技能命令里注入任何"纠正客户端当前页"的帧。两次实测都失败：
//
//  1. 第八轮：mode0+mode1（unlockRefresh）⇒ 0xC0000005。等于在技能窗口打开时重建
//     整个场景角色，窗口持有的技能对象随即悬垂。
//  2. 第十一轮：主动发一个 S→C 的 **CMD260 通知**（只 9 字节）⇒ 同样 0xC0000005。
//
// 对照：第十轮的版本（什么都不发）在同一台机器上 `exit=0x0` 正常退出。
//
// ⇒ **服务端没有"不重建角色又能纠正客户端页签"的途径。** 客户端在 reset / auto-set /
// cmd2179 之后重建技能窗口、并把当前页重置为 0，是**它自己的行为**：
// 玩家点一下"技能类型替换按钮"就能切回第二页（cmd260 那条路已实机验证可用）。
// 要彻底解决必须改客户端，或找到有抓包/逆向证据的别的途径 —— 别再在服务端试。
