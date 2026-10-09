package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type skillSession struct {
	nonce           [16]byte
	initialized     bool
	commandSequence uint64
}

func (s *skillSession) saveCommands(cs *character.Service, w *worldSession, p []byte) (int, error) {
	if cs == nil || w == nil || w.role.ID == 0 {
		return 0, fmt.Errorf("skill commands require owned selected character")
	}
	req, err := protocol.DecodeSkillCommands(p)
	if err != nil {
		return 0, err
	}
	if !s.initialized {
		if _, err = rand.Read(s.nonce[:]); err != nil {
			return 0, err
		}
		s.initialized = true
	}
	s.commandSequence++
	key := fmt.Sprintf("skill-commands-v1:%x:%d", s.nonce, s.commandSequence)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, err := cs.SaveSkillCommands(ctx, w.role, key, req)
	if err != nil {
		return 0, err
	}
	w.role = saved
	return len(req.Entries), nil
}

// skillTreeRefreshRequired reports whether the id19 full skill-tree restore has
// to follow this mutation.
//
// A VP apply (CMD29 carrying variation slots) must not: its own response
// already carries the VP block, while id19 has none — the client overwrites the
// panel it just rendered with an empty variation state, which reads as "Apply
// silently reset my VP choices". CMD28 moves are already applied locally by
// the client; an id19 after their ACK also plays the learn sound and clears the
// displayed VP choices. Other mutations keep the restore to refresh the palette
// or return the client to stored state after a refused/idempotent request.
func skillTreeRefreshRequired(id uint16, applied, varied bool) bool {
	if id == 28 {
		return false
	}
	if !applied {
		return true
	}
	return !(id == 29 && varied)
}

func skillMutationResponsePlan(cs *character.Service, saved database.Character, id uint16, body []byte, applied, varied bool) ([]outboundPacket, error) {
	plan := []outboundPacket{{"skill_committed_response", 1, id, body}}
	if skillTreeRefreshRequired(id, applied, varied) {
		restore, err := cs.EntrySkills(saved)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"skill_state_restored", 0, 19, restore})
		// 全量技能树会清空进化／突破显示；批量换位、普通加点和
		// 幂等重放完成刷新后，需要恢复当前存档中的完整配置。
		variation, err := cs.VariationRestore(saved)
		if err != nil {
			return nil, err
		}
		if len(variation) > 0 {
			plan = append(plan, outboundPacket{"skill_variation_response", 1, 29, variation})
		}
		plan, err = appendSkillPresetRestore(plan, cs, saved, "skill_preset_restored_after_skill_state")
		if err != nil {
			return nil, err
		}
		plan, err = appendComboSkillRestore(plan, cs, saved)
		if err != nil {
			return nil, err
		}
	}
	return plan, nil
}

func (s *skillSession) handle(cs *character.Service, w *worldSession, id uint16, p, raw []byte) ([]outboundPacket, error) {
	if cs == nil || w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("skill request requires owned selected character")
	}
	if len(raw) < 16 {
		raw = append([]byte(nil), raw...)
		raw = append(raw, make([]byte, 16-len(raw))...)
	}
	log.Printf("[skill-debug] id=%d len=%d hex=%s", id, len(p), hex.EncodeToString(raw[:16]))
	if !s.initialized {
		if _, e := rand.Read(s.nonce[:]); e != nil {
			return nil, e
		}
		s.initialized = true
	}
	h := sha256.Sum256(raw)
	key := fmt.Sprintf("skill:%x:%x", s.nonce, h)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var saved database.Character
	var applied bool
	var varied bool
	var body []byte
	var e error
	switch id {
	case 28:
		var r protocol.SkillMove
		r, e = protocol.DecodeSkillMove(p)
		if e == nil {
			saved, applied, e = cs.MoveSkill(ctx, w.role, key, r)
		}
		if e == nil {
			body = protocol.SkillMoveSuccess(r)
		}
	case 29:
		var r protocol.SkillPurchase
		r, e = protocol.DecodeSkillPurchase(p)
		if e != nil {
			log.Printf("[skill-debug] 29 DECODE ERR tree=%d %v", r.Tree, e)
			break
		}
		log.Printf("[skill-debug] 29 tree=%d entries=%d mode=%d preset=%d inten=%v opt=%v", r.Tree, len(r.Entries), r.Mode, r.Preset, r.Intensions != nil, r.Options != nil)
		varied = r.Intensions != nil || r.Options != nil
		saved, applied, e = cs.Learn(ctx, w.role, key, r)
		if e != nil {
			log.Printf("[skill-debug] 29 LEARN ERR tree=%d %v", r.Tree, e)
		}
		if e == nil {
			body, e = cs.LearningResponse(saved, r)
		}
	case 2179:
		// CHANGE_SKILLSLOT_TOTAL: the 自动加点 shortcut-bar layout. The client
		// sends this swap list right after the auto-set learn burst and waits
		// for the acknowledgement; with no handler it stayed an unimplemented
		// sample (zero response), so the bar kept the server's own order and the
		// preview the player confirmed was never applied.
		//
		// ⚠️ 实验七（21:5x）：受理落库后**补发 NOTI19**——客户端 autoset 后等
		// NOTI19 应用布局（预览只是模拟，第六轮零刷新帧导致当次显示旧排列）。
		// 只带 NOTI19，不带 NOTI29/preset/combo（页2 崩点候选隔离）。
		// ACK 的 tree 字节仍强制 0（实验六已证 tree=1 响应帧是崩点）。
		var r protocol.SkillSlotTotal
		r, e = protocol.DecodeSkillSlotTotal(p)
		if e == nil {
			saved, applied, e = cs.MoveSkillTotal(ctx, w.role, key, r)
		}
		if e == nil {
			w.role = saved
			ack := r
			ack.Tree = 0
			restore, err := cs.EntrySkills(saved)
			if err != nil {
				return nil, err
			}
			plan := []outboundPacket{{"skill_committed_response", 1, 2179, protocol.SkillSlotTotalSuccess(ack)}}
			plan = append(plan, outboundPacket{"skill_state_restored", 0, 19, restore})
			// ⚠️ NOTI19 的包体**不含 VP（进化／突破）数据**，客户端收到后会用
			// 空 VP 覆盖刚显示正确的面板 —— 这正是 2026-09-22 踩过的同一个坑
			// （《技能系统修复_20260922》第一节："点 Apply 后 id=29 之后再发
			// id=19 ⇒ 显示重置，重选角色才正常"）。
			// 公共出口 skillMutationResponsePlan 已经在 19 之后补了 id=29，
			// 但本分支是提前 return 的，漏了这一步 ⇒ auto set 之后 enhance
			// 看起来没设置、重登才恢复。这里补回同一个 VariationRestore。
			variation, err := cs.VariationRestore(saved)
			if err != nil {
				return nil, err
			}
			if len(variation) > 0 {
				plan = append(plan, outboundPacket{"skill_variation_response", 1, 29, variation})
			}
			return plan, nil
		}
	case 2346:
		var r protocol.SkillPreset
		r, e = protocol.DecodeSkillPresetSave(p)
		if e == nil {
			saved, applied, e = cs.SaveSkillPreset(ctx, w.role, key, r)
		}
		if e == nil {
			w.role = saved
			// The editor already applied this configuration locally. ACK only;
			// NOTI2758 is used on entry and after a skill-tree rebuild.
			return []outboundPacket{{"skill_preset_saved", 1, 2346, protocol.SkillPresetSaveSuccess()}}, nil
		}
	case 483:
		// Two request shapes share this command. The Skill Reset window's
		// Confirm frame is at least 8 bytes with a (style, mask) layout:
		// p[0]=style (0/1), p[1]==0, p[2]=mask restricted to the defined bits.
		// Everything else is the Reset/Auto Set button's opaque body (reading
		// it as a (tree,mask) pair yielded tree 111 / mask 40) and clears all
		// three groups of the main tree, letting the client lay out its own
		// recommended shortcuts as it does natively.
		//
		// ⚠️ 2026-10-09：这两条判据目前**无法区分**两个按钮（实测 Reset 的确认帧恰好
		// 8 字节，而 Auto Set 的形状未取到），所以这里保持上游原样不动 —— 等拿到
		// Auto Set 按钮的明文再定向修。
		if len(p) >= 8 && p[1] == 0 && p[0] <= 1 && p[2]&^(character.ResetOrdinarySkills|character.ResetEnhance|character.ResetEvolve) == 0 {
			style, mask := p[0], p[2]
			// ⚠️ 2026-10-09 试过把 style 改成"从存档取当前页"——**实测无效且更危险，已回退**：
			//   存档页与客户端显示页并不同步（存档 SkillTreeType=1 时客户端正停在第二页），
			//   以存档为准会去洗玩家根本没在看的那页。
			//   实机取证：autoset 确认帧 p[0]=0，紧随其后的 cmd29 也报 Tree=0 —— 两者一致，
			//   说明 **p[0] 就是客户端的真实页**，照用即可。
			//   真正让 enhance 不刷新的不是页，是 cmd29 响应里 mode 用了 req.Mode（见
			//   internal/character/learning.go LearningResponse）。
			log.Printf("[skill-debug] 483 RESET-WINDOW branch style=%d mask=%d", style, mask)
			saved, _, e = cs.ResetSkills(ctx, w.role, key, style, mask)
			if e != nil {
				break
			}
			w.role = saved
			restore, e := cs.EntrySkills(saved)
			if e != nil {
				return nil, e
			}
			body, e = cs.ResetResponse(saved, style)
			if e != nil {
				return nil, e
			}
			// Reset window responses always lead with the full skill tree and
			// close with the variation frame; the open Evolve/Enhance panel
			// renders the last variation frame it receives.
			plan := []outboundPacket{{"skill_state_restored", 0, 19, restore}}
			plan, e = appendSkillPresetRestore(plan, cs, saved, "skill_preset_restored_after_skill_reset")
			if e != nil {
				return nil, e
			}
			// ⛔ 这里**不能**补发 mode0+mode1 去"把页签拉回第二页"：实机 2026-10-09
			// 18:26 在技能命令里注入 unlockRefresh 之后，客户端以 0xC0000005（访问冲突）
			// 退出 —— 那等于在技能窗口打开时重建整个场景角色，窗口持有的技能对象随即悬垂。
			// 客户端在 reset/autoset 后回到第一页是它**自己的**行为，改由玩家点"技能类型
			// 替换按钮"(cmd260) 切回即可（那条路已实机验证）。
			return append(plan, outboundPacket{"skill_variation_reset_response", 1, 29, body}), nil
		}
		if len(p) < 3 {
			return nil, fmt.Errorf("short reset request")
		}
		// ⚠️ auto-set 必须作用于**当前页**。写死 0 时，在第二页点 autoset 会去洗第一页
		// ⇒ 服务端状态与客户端所在的页错开 ⇒ 客户端崩。实机 2026-10-09 20:42 的对照很干净：
		// 第二页的**手动加点**（cnt=1/6/14，走 Learn 那条路）全都不崩，**只有 autoset 崩**，
		// 而 autoset 正是走这条分支。
		// body 是 opaque（按 (tree,mask) 读得到 tree111/mask40），只能从存档取当前页。
		log.Printf("[skill-debug] 483 AUTO-SET branch p0=%d p1=%d p2=%d len=%d", p[0], p[1], p[2], len(p))
		autoTree := byte(0)
		var autoState character.State
		if json.Unmarshal(w.role.State, &autoState) == nil && character.SkillTreeWireIndex(autoState) == 1 {
			autoTree = 1
		}
		saved, e = cs.ResetAutoSet(ctx, w.role, key, autoTree, 7)
		if e != nil {
			return nil, e
		}
		w.role = saved
		restore, e := cs.EntrySkills(saved)
		if e != nil {
			return nil, e
		}
		plan := []outboundPacket{{"skill_state_restored", 0, 19, restore}}
		plan, e = appendSkillPresetRestore(plan, cs, saved, "skill_preset_restored_after_auto_set")
		if e != nil {
			return nil, e
		}
		variation, e := cs.VariationRestore(saved)
		if e != nil {
			return nil, e
		}
		if len(variation) > 0 {
			plan = append(plan, outboundPacket{"skill_variation_response", 1, 29, variation})
		}
		// ⛔ 这里**不要**补任何"纠正页签"的帧：mode0+mode1（第八轮）和 S→C CMD260
		// 通知（第十一轮）都实测 0xC0000005。客户端自己回到第一页，玩家点按钮切回即可。
		return plan, nil
	case 2347:
		// Chain / skill preset reset is not covered by the confirmed save/restore
		// protocol. Keep the existing zero-mutation response. Omitting id29 would make the client render
		// Enhance/Evolve/VP as cleared until the next login.
		saved = w.role
		restore, e := cs.EntrySkills(saved)
		if e != nil {
			return nil, e
		}
		plan := []outboundPacket{{"skill_state_restored", 0, 19, restore}}
		variation, e := cs.VariationRestore(saved)
		if e != nil {
			return nil, e
		}
		if len(variation) > 0 {
			plan = append(plan, outboundPacket{"skill_variation_chain_response", 1, 29, variation})
		}
		// ⛔ 同上：不补"纠正页签"的帧。
		return plan, nil
	case 260:
		// CHANGE_ANOTHER_SKILL_TREE：技能窗口的"技能类型替换按钮"。
		// 它只改选择位，不需要下面的技能事务计划（不重建技能树）。
		return w.changeAnotherSkillTree(p)
	default:
		return nil, fmt.Errorf("unsupported skill mutation")
	}
	if e != nil {
		return nil, e
	}
	before := w.role
	w.role = saved
	plan, e := skillMutationResponsePlan(cs, saved, id, body, applied, varied)
	if e != nil {
		return nil, e
	}
	// 训练关卡在技能事务里完成时（第三关技能进化点，character.completeBoostVPSave）
	// 不会自己带进度帧，实机 2026-10-04 会话里加点完成后客户端只发心跳：这里比较本次
	// 事务前后的训练状态，只在确实前进时补发一条 NOTI2638，任务面板才会刷新。
	plan = append(plan, boostTrainingProgress(w.boostup, before, saved)...)
	// ⛔ 不补"纠正页签"的帧：mode0+mode1（第八轮）与 S→C CMD260 通知（第十一轮）
	// 都实测过 0xC0000005。客户端重绘 NOTI19 后回到第一页是它自己的行为。
	return plan, nil
}
