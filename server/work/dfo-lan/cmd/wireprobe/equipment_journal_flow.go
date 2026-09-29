package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
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
func equipmentJournalEntryPayload(role storage.Character, rules *catalog.EquipmentJournalRules) ([]byte, error) {
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
func (w *worldSession) equipmentJournalBoard(role storage.Character) ([]outboundPacket, error) {
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
// 成本校验 / 扣料 / 发装备由 `loot.Service.CreateEquipment` 完成，**严格按 `[13]` 扣**。
func (w *worldSession) equipmentCraft(p []byte) ([]outboundPacket, error) {
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
	saved, receipt, applied, e := w.loot.CreateEquipment(ctx, w.role, templates[0], slots[0], int(r.PayOption))
	if e != nil {
		// 拒绝**不能吞**：照常回窗口（客户端本来就会继续显示），但把原因写清楚。
		// 不额外发拒绝包 —— 这一条还没有实机样本证明客户端认哪种拒绝形状。
		log.Printf("equipment craft REFUSED: template=%d slot=%d pay_option=%d: %v",
			templates[0], slots[0], r.PayOption, e)
		return plan, nil
	}
	if !applied {
		// ★ 幂等重放：这条请求之前已经提交过，**没有再扣一次**。回执也取不回来
		// （`CommitAccountMaterialEvent` 在重放路径上只回账号材料的 counts）。
		// ⚠️ 所以这里**绝不能**走下面的 DONE —— 那会打出全零回执，看起来像
		// "成功了但没生效"，实机 2026-09-29 14:21 就是这样白绕了一轮。
		log.Printf("equipment craft REPLAY: already applied, nothing charged (template=%d slot=%d pay_option=%d)",
			templates[0], slots[0], r.PayOption)
		return plan, nil
	}
	w.role = saved
	// ★ 刷新要同时覆盖**两个仓**：背包（新装备 + 金币）与**账号共享材料库**
	// （三档登记证就在那里扣的）。`accountMaterialRefreshPackets` 正好按
	// list35 → list42 → list0 的顺序发，客户端靠最后那包做结算。
	accountRaw, e := w.loot.Store.AccountMaterials(ctx, saved.AccountID)
	if e != nil {
		log.Printf("equipment craft: read account materials after craft: %v", e)
		return plan, nil
	}
	account, e := inventory.ReadAccountMaterials(accountRaw)
	if e != nil {
		log.Printf("equipment craft: decode account materials after craft: %v", e)
		return plan, nil
	}
	refresh, e := accountMaterialRefreshPackets(account, saved)
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
	return plan, nil
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
	if w == nil || w.loot == nil {
		return nil
	}
	return w.loot.Journal
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
	if w == nil || w.loot == nil || w.loot.Store == nil {
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
	saved, _, e := w.loot.Store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID,
		w.role.ConfigVersion, key, "equipment-journal-v1",
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
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
