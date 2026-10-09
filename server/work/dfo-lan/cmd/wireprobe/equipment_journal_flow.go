package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// 装备库「制作 / 变换」（CMD2259）的应答参数。
//
// handler `sub_145277D00` 用应答的 u8@4 选窗口：非 0 → 3937，0 → 2145；
// 后者再由 u8@5 选子分支。**具体哪个窗口是"制作界面"还没定**，
// 所以做成 flag（`-equipment-craft-window` / `-equipment-craft-variant`，
// 也可用 DFO_EQUIPMENT_CRAFT_WINDOW / DFO_EQUIPMENT_CRAFT_VARIANT 环境变量），
// 改环境变量即可换，不必重编。
var (
	equipmentCraftWindow  byte = 1
	equipmentCraftVariant byte = 0
	// 第二步（"确定"）的应答。
	//
	// ⚠️ 实机 2026-09-29 12:20 修正：**3937 就是「装备变换」窗口本身**，
	// 而「变换确认」那层是**客户端本地弹的**（点"确定"后客户端自己关掉它）。
	// 我们一度把 3937 当"确认窗"、2145 当"结果窗" —— 两次都回 3937 才与玩家
	// 看到的"点确定就回到装备变换界面"一致。2145 保留为可调（env），默认不用。
	equipmentCraftConfirmWindow  byte = 1
	equipmentCraftConfirmVariant byte = 0
	// 装备生成（请求头 [12] == 0）走**另一扇窗**。
	//
	// 实机 2026-09-29 12:46：在「生成单个部位」里选件 → 客户端发 `action=0` 的 2259 →
	// 我们回 3937（= 装备变换）⇒ 客户端很可能就此离开了生成窗口，玩家再点「生成」毫无反应
	// （日志里只有那一条 2259）。handler 的 u8@4 **只有两支**（非 0 → 3937；0 → 2145），
	// 3937 已被实机确认是「装备变换」⇒ 生成那一支只能是 **2145**。
	equipmentCraftGenerateWindow byte = 0
	// equipmentCraftGenerateVariant 是窗口 2145 上的**方法选择**（见 protocol.EquipmentCraftReply）：
	//
	//	0  → `EquipmentTransformWindow::setState(win, succ ? 3 : 4)`（**强行**推进窗口状态）
	//	≠0 → 只写 `win[13280] = succ`，**不动 UI 状态**
	//
	// ★ 默认取 **1**（实机 2026-09-29 14:36 定案）：`payload[5]==0` 会强制状态 3，而
	// `setState` 的调用者里**客户端自己只用 0 与 2**（`sub_14150AF40` 三处 `mov edx, 2`、
	// `sub_1415086F0` 一处 `xor edx, edx`），**3 / 4 只有我们这个回包会传**
	// ⇒ 状态 3 是"客户端自己不会进、也没有出口"的状态，进去以后依赖状态 2 的控件
	// （**材料切换按钮**）就失灵了（玩家实测："生成一次后切换材料没响应"）。
	// 只落标志则窗口保持自己的状态与可交互性，玩家可以连着生成 / 换付法。
	//
	// ⚠️ 两支都**没有**关窗调用 —— 生成界面常驻是客户端自己的设计，服务端关不掉它。
	// 想回退到"强制状态 3"的观感：`set DFO_EQUIPMENT_CRAFT_GENERATE_VARIANT=0`。
	equipmentCraftGenerateVariant byte = 1
)

// equipmentCraftExecute 决定是否**真的执行**装备生成。
// 由 `-equipment-craft-execute` / `DFO_EQUIPMENT_CRAFT_EXECUTE=0` 控制（默认开）。
var equipmentCraftExecute = true

// equipmentCraftExecuteOn 决定**在哪一次请求上执行**：
//
//	"confirm"（默认）= 同一指纹第二次请求（"变换" → "确定" 那个模式）
//	"first"         = 第一次请求就执行（装备生成可能没有确认弹窗）
//	"never"         = 只回窗、不动存档
var equipmentCraftExecuteOn = "confirm"

// equipmentTransformApply 决定 action=1（装备变换）怎么执行：
//
//	"apply"（默认） = 真的换装：按目标稀有度扣灵魂 + 把该部位的装备换成选中的，然后回「打开窗口」；
//	"observe"       = 只把请求本身写进 events.jsonl，不动存档（一键回退用）。
var equipmentTransformApply = "apply"

// equipmentCraftConfirmGap 是"同一正文要隔多久才算第二步"的下限。
// 取 1 秒是为了不把玩家的**连点**（想再开一次确认窗）误判成"确认"。
const equipmentCraftConfirmGap = int64(1_000_000_000) // 1s in UnixNano

// 装备库（装备图鉴）的会话接线。
//
// 协议面只有三个包（都由客户端 2.38.3.25 的二进制定案，见 protocol/equipment_journal.go）：
//
//	CMD  2264 收藏     C→S 32B / S→C **非空**（空正文的帧会被 preparePackets 丢掉）
//	CMD  2265 收录/创建 S→C 6B
//	NOTI 2610 完整状态 S→C **恰好 16444B**
//
// 两处时序是硬要求：
//   - 2264 的**成功应答在后**：先提交账本，再回 2264，然后补发 2610。
//     只回成功而不发 2610，客户端的"乐观显示"就不会被权威值覆盖（客户端会先自己点亮 ★）。
//   - 2610 只在**已提交**的角色状态上构建，不从请求或本地缓存推算。

// equipmentJournalEntryPayload 读角色账本并组 2610 正文。
//
// 空账本返回 nil —— 此时不发这一帧：客户端自己就是全零初值，发一份全零没有信息量，
// 而"空正文的帧会被丢弃"这条约束本来也不允许我们发空包。
func equipmentJournalEntryPayload(role database.Character, rules *catalog.EquipmentJournalRules) ([]byte, error) {
	if rules == nil {
		return nil, nil
	}
	ledger, e := inventory.ReadEquipmentJournal(role.State)
	if e != nil {
		return nil, e
	}
	if len(ledger.Counts) == 0 && len(ledger.Favorites) == 0 {
		return nil, nil
	}
	return protocol.EquipmentJournalBody(ledger.Counts, ledger.Favorites)
}

// equipmentJournalBoard 是「回成功 + 补发 2610」这一对。
func (w *worldSession) equipmentJournalBoard(role database.Character) ([]outboundPacket, error) {
	body, e := equipmentJournalEntryPayload(role, w.journalRules())
	if e != nil {
		return nil, e
	}
	plan := []outboundPacket{{"equipment_journal_favorite_ack", 1, 2264, protocol.EquipmentFavoriteReply()}}
	if len(body) > 0 {
		plan = append(plan, outboundPacket{"equipment_journal_restored", 0, protocol.EquipmentJournalOpcode, body})
	}
	return plan, nil
}

// logTransformCostPreview 打印并落盘一次「装备变换」的**成本预览**（纯诊断，不动存档）。
//
// [DIAG-20261009-TRANSFORM-COST] 只在 `-equipment-transform observe` 下调用。
//
// 背景：CMD2259 的应答只有「窗口 + 方法」两个字节，**没有失败与禁用语义**，变换按钮能不能
// 点是客户端拿自己的成本×库存判的。当出现「客户端没禁用、服务端却以材料不足拒绝」时，服务端
// 无法通知客户端 —— 只能先把**自己算的成本**摊开，拿去和变换界面显示的数字对账，判定分歧是
// ① 按件 vs 按套（差 11 倍）② 魂的种类对不上 ③ 客户端读的材料仓与我们扣的仓不同。
func (w *worldSession) logTransformCostPreview(r protocol.EquipmentCraftRequest, slots, templates []uint32, event func(map[string]any)) {
	if w == nil || w.items == nil {
		log.Printf("equipment craft TRANSFORM-COST: item service unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lines := w.items.InspectTransformCost(slots, templates, int(r.PayOption))
	var account inventory.AccountMaterials
	if w.store != nil {
		raw, e := w.store.AccountMaterials(ctx, w.role.AccountID)
		if e != nil {
			log.Printf("equipment craft TRANSFORM-COST: read account materials: %v", e)
		} else if m, e := inventory.ReadAccountMaterials(raw); e != nil {
			log.Printf("equipment craft TRANSFORM-COST: decode account materials: %v", e)
		} else {
			account = m
		}
	}
	bagGold := uint32(0)
	if bag, e := inventory.ReadBag(w.role.State); e == nil {
		bagGold = bag.Gold
	}

	describe := func(list []inventory.MaterialCost) string {
		if len(list) == 0 {
			return "-"
		}
		out := ""
		for _, m := range list {
			out += fmt.Sprintf("%d×%d ", m.Count, m.Template)
		}
		return out
	}
	// 累计口径：真正扣料是逐件累加的，所以"够不够"必须按累计量看（这正是
	// PrepareEquipmentTransform 里那处判定与扣款不一致的缺陷所在）。
	var goldTotal uint32
	total, running, have, shortage := map[uint32]uint32{}, map[uint32]uint32{}, map[uint32]uint32{}, map[uint32]uint32{}
	var singleGold uint32
	singleAccount := map[uint32]uint32{}
	for i, ln := range lines {
		goldTotal += ln.Gold
		if i == 0 {
			singleGold = ln.Gold
			for _, m := range ln.AccountMats {
				singleAccount[m.Template] += m.Count
			}
		}
		flags := ""
		if ln.Unpriced {
			flags += " unpriced(源[create cost]无此件)"
		}
		if ln.Problem != "" {
			flags += " problem=" + ln.Problem
		}
		detail := ""
		for _, m := range ln.AccountMats {
			total[m.Template] += m.Count
			running[m.Template] += m.Count
			if have[m.Template] == 0 {
				have[m.Template] = account.Count(m.Template)
			}
			detail += fmt.Sprintf("%d×%d(累计%d/有%d) ", m.Count, m.Template, running[m.Template], have[m.Template])
		}
		if detail == "" {
			detail = "-"
		}
		log.Printf("equipment craft TRANSFORM-COST item: slot=%d template=%d rarity=%d(%s) gold=%d bag=%s account=%s%s",
			ln.Slot, ln.Template, ln.Rarity, ln.RarityName, ln.Gold, describe(ln.BagMats), detail, flags)
	}
	for t, need := range total {
		if h := account.Count(t); need > h {
			shortage[t] = need - h
		}
	}
	groups := make([]uint32, 0, len(r.Entries))
	for _, en := range r.Entries {
		groups = append(groups, en.Group)
	}
	log.Printf("equipment craft TRANSFORM-COST: requested=%d pay_option=%d groups=%v gold_total=%d bag_gold=%d account_need=%v account_have=%v shortage=%v",
		len(lines), int(r.PayOption), groups, goldTotal, bagGold, total, have, shortage)
	log.Printf("equipment craft TRANSFORM-COST: 单件口径 gold=%d account=%v —— 若界面按「一套只算一次」显示，应接近这一行",
		singleGold, singleAccount)
	if event != nil {
		event(map[string]any{"kind": "equipment_craft_transform_cost_preview", "character_id": w.role.ID,
			"requested": len(lines), "pay_option": int(r.PayOption), "groups": groups,
			"lines": lines, "gold_total": goldTotal, "bag_gold": bagGold,
			"account_need": total, "account_have": have, "shortage": shortage,
			"single_piece_gold": singleGold, "single_piece_account": singleAccount})
	}
}

// equipmentCraft 处理 CMD2259：装备库「制作 / 变换」。
//
// ★ 装备生成已于 2026-09-29 13:51 / 13:52 实机验收通过（两件都进了背包）。
//
// 应答 6 字节（正文 7，头 1 字节是分派器吃掉的成功前缀）的语义见
// protocol.EquipmentCraftReply：`payload[4]` 选窗口、`payload[5]` 选窗口上的方法。
// **其中没有任何关窗语义** —— 生成界面常驻是客户端自己的设计（见 next127 §10）。
//
// 请求侧两个关键字节（`EquipmentCraftRequest`）：
//
//	[12] Action    = 0 装备生成 / 1 装备变换
//	[13] PayOption = 玩家点的**付款方式序号**（1 起，对应 [create cost] 里 [cost] 行的顺序）
//
// 执行时机：`[12]==1`（变换）实机是「变换 → 确定」**两次**同指纹请求 ⇒ 等第二次；
// `[12]==0`（生成）**只有一次**请求 ⇒ 单步执行（`-equipment-craft-execute-on` 可覆盖）。
//
// 成本校验 / 扣料 / 发装备由 `inventory.ItemService.CreateEquipment` 完成，**严格按 `[13]` 扣**。
//
// `action == 1`（装备变换）则走 `inventory.ItemService.TransformEquipment`：把该部位的装备换成
// 图鉴里选中的那件、打造效果跟着走、源自动登记进图鉴。
//
// `event` 用于把**不落库**的判定（拒绝原因、逐件跳过明细、变换结果）写进 events.jsonl ——
// 这些分支都不回包，日志是唯一的取证入口。
func (w *worldSession) equipmentCraft(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil {
		return nil, fmt.Errorf("equipment craft unavailable")
	}
	r, e := protocol.DecodeEquipmentCraftRequest(p)
	if e != nil {
		return nil, e
	}
	slots, templates := r.Wanted()
	now := time.Now().UnixNano()
	key := craftFingerprint(r, slots, templates)
	confirm := key != "" && key == w.craftPending && now-w.craftPendingAt >= equipmentCraftConfirmGap
	// 应答窗口按动作号选：生成（[12]==0）→ 2145 那一支；变换（[12]==1）→ 3937 那一支。
	step, window, variant := "open", equipmentCraftWindow, equipmentCraftVariant
	name := "equipment_craft_opened"
	switch {
	case r.Action == 0:
		window, variant = equipmentCraftGenerateWindow, equipmentCraftGenerateVariant
	case confirm:
		step, window, variant = "confirm", equipmentCraftConfirmWindow, equipmentCraftConfirmVariant
		name = "equipment_craft_confirmed"
	}
	if confirm {
		w.craftPending, w.craftPendingAt = "", 0
	} else {
		w.craftPending, w.craftPendingAt = key, now
	}
	log.Printf("equipment craft %s: panel=%d context=%#x action=%d pay_option=%d slots=%v templates=%v reply_window=%d variant=%d execute_on=%s",
		step, r.Panel, r.Context, r.Action, r.PayOption, slots, templates, window, variant, equipmentCraftExecuteOn)

	plan := []outboundPacket{{
		Name:    name,
		Kind:    1,
		ID:      protocol.EquipmentCraftOpcode,
		Payload: protocol.EquipmentCraftReply(window, variant),
	}}
	// [ALIGN-20260930-TRANSFORM] 「装备变换」（action=1）。
	//
	// 语义（2026-09-30 实机取证确定）：客户端把「玩家在图鉴里选中的**已收录**目标装备」+
	// 「它们各自的部位槽位」一起发上来，服务端要**把身上穿的这些部位换成目标**；成本按目标的
	// **稀有度**取对应的灵魂（`[create cost]` 那张表没有武器档、也没有太初档 ⇒ 不走它）。
	//
	// 请求里的这批件本来就只有一部分是"能换的"（客户端会把整屏都报上来，实测 11 件里只有
	// 一部分已登记 / 有档位）⇒ **逐件跳过，不整体拒绝**。
	//
	// 开关 `-equipment-transform` / `DFO_EQUIPMENT_TRANSFORM_APPLY`：
	//   apply  （默认）= 真的换装（扣灵魂 + 换装 + 源登记进图鉴）；
	//   observe        = 只记日志、不动存档（一键回退）。
	//
	// ⚠️ 外部包里那个 observe 分支会顺带输出 `[void soul]` 诊断 —— 本仓**不引入**那张表：
	//   2026-09-30 的证据已推翻"它是变换表"的假设（请求里的目标装备根本不在那张表里，
	//   也不受任何表约束），所以这里只记请求本身。
	//
	// ★ 无论哪条失败分支，都**只记日志、绝不回包** —— 客户端在 2259 上没有失败分支，
	//   两次实测收到 Error 都 `exit=0xC0000005`。
	if (r.Action == 1 || w.boostJournalSwap(r)) && len(templates) > 0 {
		if equipmentTransformApply == "observe" {
			log.Printf("equipment craft TRANSFORM-PLAN (observe): requested=%d slots=%v templates=%v",
				len(templates), slots, templates)
			// [DIAG-20261009-TRANSFORM-COST] 把**服务端算的成本**摊开，拿去和变换界面显示的数字对账。
			//
			// 为什么必须这么做：2259 的应答只有「窗口 + 方法」两个字节，**没有失败与禁用语义**，
			// 变换按钮由客户端拿自己的成本×库存判断。出现「客户端没禁用、服务端却以材料不足拒绝」
			// 时，服务端无法通知客户端，只能先把自己的口径打出来，才能判定分歧在哪
			// （按件 vs 按套 / 魂的种类 / 客户端读的仓）。纯日志，不动存档、不扣料。
			w.logTransformCostPreview(r, slots, templates, event)
			if event != nil {
				event(map[string]any{"kind": "equipment_craft_transform_planned", "character_id": w.role.ID,
					"requested": len(templates), "slots": slots, "templates": templates})
			}
			return plan, nil
		}
		if w.items == nil {
			log.Printf("equipment craft TRANSFORM-REFUSED: loot service unavailable")
			return plan, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		saved, receipt, applied, e := (&workflow.ItemService{Store: w.store, Items: w.items}).TransformEquipment(ctx, w.role, slots, templates, int(r.PayOption))
		if e != nil {
			log.Printf("equipment craft TRANSFORM-REFUSED: requested=%d: %v", len(templates), e)
			if event != nil {
				event(map[string]any{"kind": "equipment_craft_transform_refused", "character_id": w.role.ID,
					"requested": len(templates), "reason": e.Error()})
			}
			return plan, nil
		}
		if applied {
			w.role = saved
		}
		// ★ 刷新下发（这一步之前一直缺，所以"动画播完却没换装"）：
		//
		// 实机 trace 证明客户端在 2259 应答之后**不会再发任何请求**（Seq 129 之后全是
		// PROCESS_SCAN 心跳）—— 它**不会自己重读装备栏**。服务端改完 `worn` 必须**主动把
		// 身上装备栏再送一遍**，与强化/附魔/锻造/装备继承后的做法完全一致：
		// `inventory.WornSpaceUpdate` + NOTI14（见 reinforcement_flow.go / enchant_flow.go /
		// refine_flow.go / inherit_flow.go，都是这个形状）。
		if applied {
			if wornBody, e := inventory.WornSpaceUpdate(w.role.State); e == nil && len(wornBody) > 0 {
				plan = append(plan, outboundPacket{"equipment_transform_worn_refreshed", 0, 14, wornBody})
			}
			// 金币在扣费时变了，主物品栏也补一份 NOTI13 list0（与账号材料刷新同一形状）。
			if bag, e := inventory.ReadBag(w.role.State); e == nil {
				if body, e := protocol.InventoryRestore(bag.Rows(), bag.Expansion); e == nil {
					plan = append(plan, outboundPacket{"equipment_transform_inventory_refreshed", 0, 13, body})
				}
			}
			// ★ 变换会改图鉴（目标 −1、旧件 +1 登记回去），必须补发 NOTI2610 权威快照。
			//
			// 本仓既有规则（`equipment_journal_flow.go` CMD2264 § / `disjoint_flow.go` CMD26）：
			// 客户端的图鉴计数**只认 2610**，只回 ACK 时它那套"乐观显示"不会被权威值覆盖。
			// 变换此前只发 14/13，没发 2610 —— 与第 9 关分解当初的缺陷同型。顺序照参考行为，
			// 排在背包刷新之后、金库金币刷新之前。组包失败**不能吞掉已经提交的变换**：只记日志，
			// 重登仍会在入场拿到快照。
			if len(receipt.Pairs) > 0 {
				body, jErr := equipmentJournalEntryPayload(w.role, w.journalRules())
				if jErr != nil {
					if event != nil {
						event(map[string]any{"kind": "equipment_transform_journal_refresh_failed",
							"character_id": w.role.ID, "reason": jErr.Error()})
					}
				} else if len(body) > 0 {
					plan = append(plan, outboundPacket{"equipment_transform_journal_refreshed", 0,
						protocol.EquipmentJournalOpcode, body})
				}
			}
			// 金币不够时从**账号金库**调取过 ⇒ 必须补发金库金币显示包，否则金库界面停在旧值、
			// 客户端本地校验会把存取卡住（"塞满了取不出放不进"）。
			plan = append(plan, w.vaultGoldRefreshPackets(ctx, receipt.VaultGold)...)
			// ★ 第 10 关的关卡推进：变换把身上那件换成了图鉴里的目标，而第 10 关的任务
			// `transform equip journal or equip item` 与 `equip item` 走的是**同一个穿戴类判定**
			// （`boostup.Step.WearRequirement`，按 `[equip grouping]` 比对**身上穿的**，
			// 见 internal/boostup/equipment_mission.go:38）—— 判定事实已经具备，
			// 缺的只是**没人重算并补帧**：
			// 实机 2026-10-06 03:34:08 `TRANSFORM: pairs=1 gold=0 applied=true` 之后，
			// 整个会话里一帧 `boost_equipment_mission_progress` 都没有 ⇒ 任务面板不动、
			// 第 10 关不推进（业主报告「变化成功了但活动不认为我成功」）。
			// 与 CMD19 穿戴（equipment_flow.go）/ CMD272 附魔（enchant_flow.go）同一口径：
			// 推进不能等客户端来问 —— 它收到 2259 应答之后只发心跳。
			// 失败只记日志，**不回滚已提交的变换**。
			plan = append(plan, w.reconcileBoostEquipment()...)
		}
		log.Printf("equipment craft TRANSFORM: requested=%d pairs=%d gold=%d option=%d skipped=%d applied=%t",
			len(templates), len(receipt.Pairs), receipt.Gold, receipt.Option, len(receipt.Skipped), applied)
		if event != nil {
			event(map[string]any{"kind": "equipment_craft_transform_done", "character_id": w.role.ID,
				"requested": len(templates), "pairs": receipt.Pairs, "materials": receipt.Materials,
				"gold": receipt.Gold, "option": receipt.Option, "skipped": receipt.Skipped,
				"applied": applied, "source": receipt.Source})
		}
		return plan, nil
	}
	// 执行时机：
	//   「变换」([12]==1) 实机是"变换 → 确定"**两次**请求（同指纹）⇒ 等第二次；
	//   「生成」([12]==0) 实机**只有一次**请求（2026-09-29 12:46 / 13:13 两次会话都只 1 条，
	//     点「装备生成」不会再来第二条）⇒ 单步执行。
	want := len(templates) > 0 && (r.Action == 0 || confirm)
	switch equipmentCraftExecuteOn {
	case "first":
		want = len(templates) > 0
	case "never":
		want = false
	}
	if !equipmentCraftExecute || !want {
		return plan, nil
	}

	// 第二步 = 执行「装备生成」：模板必须已在账本登记，成本来自 [create cost] 表。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, applied, e := (&workflow.ItemService{Store: w.store, Items: w.items}).CreateEquipment(ctx, w.role, templates[0], slots[0], int(r.PayOption))
	if e != nil {
		// [ALIGN-20260930-CRAFT-VISIBLE] 拒绝**不能吞** —— 但**也不能瞎回包**。
		//
		// 2026-09-30 00:37 实测教训：原先这里追加过一个拒绝包
		//   {"equipment_craft_refused_response", 1, 2259, protocol.Refusal(19)}
		// 结果客户端**崩溃**（client.log `exit=0xC0000005`）。完整时序（client_trace）：
		//   SEND ENUM_CMDPACKET_EQUIPMENT_TRANSFORM (141B)
		//   RECV ENUM_CMDPACKET_EQUIPMENT_TRANSFORM (8B) Result : Ok          ← 打开窗口应答
		//   RECV ENUM_CMDPACKET_EQUIPMENT_TRANSFORM (8B) Result : Error ErrCode : 19   ← 拒绝包
		// 客户端**确实认得这个形状**（解析出了 `ErrCode : 19`），但在「先成功、再 Error」
		// 这个序列下掉了线 —— 很可能是它已经按成功开了窗，随后又走"未开窗"的失败清理分支，
		// 对象不匹配。**在拿到正确形状/正确序列之前，不要再发这个包。**
		//
		// 现在只做两件安全的事：① 拒因写进 events.jsonl；② 照常回「打开窗口」应答。
		log.Printf("equipment craft REFUSED: template=%d slot=%d pay_option=%d: %v",
			templates[0], slots[0], r.PayOption, e)
		if event != nil {
			event(map[string]any{"kind": "equipment_craft_create_refused", "character_id": w.role.ID,
				"template": templates[0], "slot": slots[0], "pay_option": r.PayOption,
				"requested": len(templates), "reason": e.Error()})
		}
		// [ALIGN-20260930-CRAFT-NO-REFUSAL] 拒绝**不发任何出站包**（也不发 Error）。
		//
		// 两次实测（都只看客户端自述，不看猜测）：
		//   00:37  「先回成功打开窗口、再回 Error」→ trace 收到 `Result : Ok` + `Result : Error ErrCode : 19` → **崩溃 exit=0xC0000005**
		//   00:42  「只回 Error、不回窗口」        → trace 只收到 `Result : Error ErrCode : 19`        → **仍然崩溃 exit=0xC0000005**
		// ⇒ **客户端在 2259 这条链上没有失败分支**：它认得这个形状（能解析出 ErrCode 19），
		//   但拿到 Error 就走进了会崩的路径。所以在拿到真正的成功产物规则之前，
		//   **唯一安全的行为是「什么都不说」**（照常回"打开窗口"应答，客户端只播动画、不掉线，
		//   与 00:29 那次一致）。
		//
		// ⚠️ 这不是修复，只是把客户端保住；真正的修复必须让 2259 **成功**（见
		//   `docs/` 里 CMD2259 的取证记录）。拒因仍写进 events.jsonl 供后续分析。
		return plan, nil
	}
	if !applied {
		// ★ 幂等重放：这条请求之前已经提交过，**没有再扣一次**。回执也取不回来
		// （`CommitAccountMaterialEvent` 在重放路径上只回账号材料的 counts）。
		// ⚠️ 所以这里**绝不能**走下面的 DONE —— 那会打出全零回执，看起来像
		// "成功了但没生效"，实机 2026-09-29 14:21 就是这样白绕了一轮。
		log.Printf("equipment craft REPLAY: already applied, nothing charged (template=%d slot=%d pay_option=%d)",
			templates[0], slots[0], r.PayOption)
		if event != nil {
			event(map[string]any{"kind": "equipment_craft_replayed", "character_id": w.role.ID,
				"template": templates[0], "slot": slots[0], "pay_option": r.PayOption})
		}
		return plan, nil
	}
	w.role = saved
	// ★ 刷新要同时覆盖**两个仓**：背包（新装备 + 金币）与**账号共享材料库**
	// （三档登记证就在那里扣的）。`accountMaterialRefreshPackets` 正好按
	// list35 → list42 → list0 的顺序发，客户端靠最后那包做结算。
	accountRaw, e := w.store.AccountMaterials(ctx, saved.AccountID)
	if e != nil {
		log.Printf("equipment craft: read account materials after craft: %v", e)
		return plan, nil
	}
	account, e := inventory.ReadAccountMaterials(accountRaw)
	if e != nil {
		log.Printf("equipment craft: decode account materials after craft: %v", e)
		return plan, nil
	}
	refresh, e := accountMaterialRefreshPackets(account, saved, w.activeDungeon != nil)
	if e != nil {
		log.Printf("equipment craft: build account material refresh: %v", e)
		return plan, nil
	}
	for _, p := range refresh {
		p.Name = "equipment_craft_" + p.Name
		plan = append(plan, p)
	}
	log.Printf("equipment craft DONE: template=%d placed_slot=%d group=%d cost_option=%d gold=%d materials=%v",
		receipt.Template, receipt.Slot, receipt.Group, receipt.Cost, receipt.Gold, receipt.Materials)
	if event != nil {
		event(map[string]any{"kind": "equipment_craft_done", "character_id": w.role.ID,
			"template": receipt.Template, "placed_slot": receipt.Slot, "group": receipt.Group,
			"cost_option": receipt.Cost, "gold": receipt.Gold})
	}
	return plan, nil
}

// boostJournalPanel / boostJournalContext 是 **662 教学期装备库窗口**的标识
// （实机 2026-10-04 帧：`u32@0=36`、`u32@8=0x054131D0`）。
//
// 该窗口的 `[12]` 实测为 **0**，但它与既有取证的两对来源不符
// （`panel=164 / context=0x46ece836` = 变换，`panel=0 / context=0x005ff2f9` = 生成，
// 见 `analysis/tasks/next126-装备库制作CMD2259阶段一落地.md` §9），且请求点名的是
// 「槽位 + 图鉴已登记的目标」—— 正是变换的输入形状（把图鉴那件换到点名槽位）。
// ⇒ 该窗口在教学轨道内按**变换**分派。
// ⚠️ **只看 panel，不要钉 context**：实测两天两个不同的值
// （2026-10-04 、2026-10-05 ）—— 它随窗口实例变，不是窗口标识。
// 前一次就是因为把 context 钉死成常量而没命中，学员号仍走了生成路径（落背包、不是互换）。
const boostJournalPanel = 36

// boostJournalSwap 判定这次 2259 是否来自教学期图鉴窗口、且角色仍在 662 训练轨道。
// 两个条件都满足才改派为变换；出关或换窗口一律回到按 `[12]` 分派。
func (w *worldSession) boostJournalSwap(r protocol.EquipmentCraftRequest) bool {
	if w == nil || w.boostup == nil || w.role.ID == 0 {
		return false
	}
	if r.Panel != boostJournalPanel {
		return false
	}
	st, e := boostup.ReadState(w.role.State)
	return e == nil && st.Activated && !st.Training.Finished
}

// craftFingerprint 把一次 2259 请求压成一个字符串指纹。
// 只用**语义字段**（面板 + 被点选的槽与模板），不含每帧变化的密钥材料 ——
// 那正是"两步同一指纹"的前提。
func craftFingerprint(r protocol.EquipmentCraftRequest, slots, templates []uint32) string {
	if len(slots) == 0 {
		return ""
	}
	parts := make([]string, 0, len(slots))
	for i := range slots {
		parts = append(parts, fmt.Sprintf("%d:%d", slots[i], templates[i]))
	}
	return fmt.Sprintf("%d/%s", r.Panel, strings.Join(parts, ","))
}

func (w *worldSession) journalRules() *catalog.EquipmentJournalRules {
	if w == nil || w.items == nil {
		return nil
	}
	return w.items.Journal
}

// equipmentFavorite 处理 CMD2264：**全量替换**一个类别的收藏列表。
//
// 实机 8 条样本确认的语义（见 next116）：
//   - 每次发的是该类别的**整张列表**（不是增量）⇒ 这里按"覆盖"写，取消收藏就是发一张更短的列表；
//   - 槽位升序、最多 3 个（请求给 4 个槽，实测第 4 槽恒 0；多出来的会被存下但不参与 2610 编码）；
//   - 类别 0..4 共 5 类，UI 只摆了 4 个标签页（类别 2 没有入口）。
//
// 幂等：同一个请求重放时 CommitCharacterEvent 命中回执，不再重复写账本；应答照发。
func (w *worldSession) equipmentFavorite(p []byte) ([]outboundPacket, error) {
	if w == nil || w.items == nil || w.store == nil {
		return nil, fmt.Errorf("equipment journal unavailable")
	}
	if w.role.ID <= 0 || w.account <= 0 || w.role.AccountID != w.account {
		return nil, fmt.Errorf("equipment journal requires the selected owned actor")
	}
	if w.activeDungeon != nil {
		// 参考行为：收藏只在城镇处理。
		return nil, fmt.Errorf("equipment journal is town-only")
	}
	r, e := protocol.DecodeEquipmentFavoriteRequest(p)
	if e != nil {
		return nil, e
	}
	slots := r.Slots[:]
	key := fmt.Sprintf("journal-favorite:%d:%d:%d:%d", r.Category, slots[0], slots[1], slots[2])
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, e := w.store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID,
		w.role.ConfigVersion, key, "equipment-journal-v1",
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			ledger, e := inventory.ReadEquipmentJournal(current.State)
			if e != nil {
				return nil, nil, e
			}
			if r.Kind != 1 {
				// Kind(+12) 实测恒 1，但没定死语义 —— 只记诊断，不拒绝（拒绝会变成"点了没反应"）。
				// 这里用不上，交给上层日志。
				_ = r.Kind
			}
			ledger, normalized, e := ledger.ReplaceFavorites(uint32(r.Category), slots)
			if e != nil {
				return nil, nil, e
			}
			updated, e := inventory.SaveEquipmentJournal(current.State, ledger)
			if e != nil {
				return nil, nil, e
			}
			receipt, e := json.Marshal(map[string]any{
				"category": r.Category, "slots": normalized,
			})
			return updated, receipt, e
		})
	if e != nil {
		return nil, e
	}
	saved.WireID = w.role.WireID
	w.role = saved
	return w.equipmentJournalBoard(saved)
}
