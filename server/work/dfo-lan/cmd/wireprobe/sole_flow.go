package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"time"
)

// CMD2288 秘宝精度提升（`ENUM_CMDPACKET_SOLE_EQUIPMENT_QUALITY`）。
//
// 协议与规则来源见：
//   - internal/game/protocol/sole.go（24 字节请求 / kind 1 回包，实机与交接书证据在注释里）
//   - internal/catalog/sole_equipment.go（直读 etc/115lvability/soleequipmentsystem.cos）
//
// 回包顺序（照 AI 交接书 §2.3 的硬约束）：
//
//  1. {sole_quality_ack, kind=1, 2288, 01|container|slot}   ← **必须首包**
//     客户端在「期望回包树」里登记了 2288，收不到同 id 包就清不掉等待态 ⇒ 精度窗口卡死
//     （交接书症状 1：提升一次后窗口无响应、需关窗重开）。
//  2. 账号共享材料被扣 → accountMaterialRefreshPackets（list35 → list42 → list0，
//     **list35 必须早于 list0**：list0 的读者会把 363..379 重新收进账号仓库管线）。
//  3. 背包材料 / 装备本体行 → id14 增量行（appendEquipmentUpdates 按容器分流：
//     容器 0 用背包行、容器 3 整体刷新穿戴空间）。
//  4. 名望（精度直接进名望结算）→ appendFameUpdate。
//
// 失败分支：**仍然发同一个 2288 回包**（状态 1、回显客户端报的 container/slot）。
// 理由：客户端不发回包就卡在等待态，而 2288 的失败语义（状态 0 + u16 错误码）**没有任何
// 反编译依据**（交接书 §8）—— 发一个"无依据的失败码"风险更大；发回显包后客户端会刷新
// 精度条，玩家看到的是"数值没变"，与失败观感一致。真实原因写在服务端
// `sole_quality_refused` 事件里。
func (s *equipmentSession) raiseSoleQuality(service *workflow.WearService, w *worldSession, p, raw []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 || w.activeDungeon != nil {
		return nil, inventory.Refuse(inventory.RefusalItems, "秘宝精度提升需要已选角色且位于城镇")
	}
	r, err := protocol.DecodeSoleQuality(p)
	if err != nil {
		return nil, err
	}
	key, err := s.requestKey(raw)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, out, err := service.ApplySoleQuality(ctx, w.role, "sole-quality:"+key, r)
	if err != nil {
		// 失败也要回同 id 包，否则客户端精度窗口会卡在等待态（见文件头注释）。
		event(map[string]any{"kind": "sole_quality_refused", "character_id": w.role.ID,
			"reason": err.Error(), "container": r.Container, "slot": r.Slot,
			"request_hex": hex.EncodeToString(p), "payload_offset": r.PayloadOffset})
		return []outboundPacket{{"sole_quality_ack_failed", 1, protocol.SoleQualityOpcode,
			protocol.SoleQualityReply(r.Container, r.Slot)}}, err
	}
	w.role = saved

	plan := []outboundPacket{{"sole_quality_ack", 1, protocol.SoleQualityOpcode,
		protocol.SoleQualityReply(out.Container, out.Slot)}}

	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return plan, err
	}
	var rows [][protocol.CurrentItemRecordSize]byte
	storageTouched := false
	for _, spent := range out.Spent {
		if spent.Template == 0 {
			continue // 金币行在下面统一处理
		}
		if spent.StorageAmount > 0 {
			storageTouched = true
		}
		// ⚠️ **不因为"这个模板属于账号材料品类"就跳过背包行**：账号材料被清扫进仓库之前
		// 就是以背包堆叠存在的，那时被扣掉的是背包那一份 —— 2026-10-02 实机踩过：
		// 灵魂（10361515）被判定为账号材料后直接 `continue`，结果客户端材料面板数量不变，
		// 玩家看到的是"材料没扣"（DB 里其实扣了）。
		if slot, ok := bagSlotOfTemplate(bag, spent.Template); ok {
			rows = append(rows, bagRowOrEmpty(bag, slot))
		}
	}
	if out.Spent != nil {
		// 扣过金币就刷新金币行（与装备调适同一约定）。
		if goldRow, ok := bag.RowAt(0); ok {
			rows = append(rows, goldRow)
		}
	}
	if out.Container == 0 {
		rows = append(rows, bagRowOrEmpty(bag, out.Slot))
	}
	// ⚠️ **订单关键**：账号材料刷新（list35 → list42 → **list0**）必须排在 id14 增量行**之前**。
	// `list0` 是整仓快照，它的读者会把 363..379 重新收进账号材料管线 —— 放在 id14 之后会把
	// 刚发出的背包行数量盖回旧值（现象：DB 扣了、客户端数量不变）。
	if storageTouched {
		if counts, e := service.Store.AccountMaterials(ctx, saved.AccountID); e == nil {
			if materials, e2 := inventory.ReadAccountMaterials(counts); e2 == nil {
				refresh, e3 := accountMaterialRefreshPackets(materials, saved)
				if e3 == nil {
					plan = append(plan, refresh...)
				}
			}
		}
	}
	plan, err = appendEquipmentUpdates(plan, saved.State, rows, out.Container,
		"sole_quality_inventory", "sole_quality_worn")
	if err != nil {
		return plan, err
	}
	spentDetail := make([]map[string]any, 0, len(out.Spent))
	for _, spent := range out.Spent {
		spentDetail = append(spentDetail, map[string]any{
			"template": spent.Template, "amount": spent.Amount,
			"from_storage": spent.FromStorage, "storage_amount": spent.StorageAmount})
	}
	event(map[string]any{"kind": "sole_quality_committed", "character_id": saved.ID,
		"template": out.Template, "container": out.Container, "slot": out.Slot,
		"selector": out.Request.Selector, "group_index": out.GroupIndex,
		"quality_before": out.QualityBefore, "quality_after": out.QualityAfter,
		"quality_gain": out.QualityGain, "max_quality": out.MaxQuality,
		"record_healed": out.RecordHealed, "gold": out.Gold, "spent": spentDetail,
		"payload_offset": r.PayloadOffset, "request_hex": hex.EncodeToString(p)})
	return w.appendFameUpdate(plan, event), nil
}

// CMD2289 秘宝制作（`ENUM_CMDPACKET_SOLE_EQUIPMENT_CREATE`）。
//
// 协议与规则来源见：
//   - internal/game/protocol/sole.go（24 字节请求、**字段偏移 13**、kind 1 回包）
//   - internal/catalog/sole_equipment.go（直读 etc/115lvability/soleequipmentsystem.cos 的
//     `[create need materials]` 段；与精度提升的 `[quality need materials]` 是两套独立表）
//
// 回包顺序（固定四步，**顺序不能改**）：
//
//  1. {sole_create_ack, kind=1, 2289, 01|模板}   ← **必须首包**
//     客户端在「期望回包树」里登记了 2289，收不到同 id 包就清不掉等待态 ⇒ 制作窗口卡死。
//  2. 账号共享材料被扣 → accountMaterialRefreshPackets（list35 → list42 → list0）。
//     **list35 必须早于 list0**：list0 是整仓快照，它的读者会把 363..379 重新收进账号材料
//     管线 —— 一旦排在第 3 步之后，就会把刚发出的背包行数量盖回旧值
//     （现象：DB 扣了、客户端数字不变）。
//  3. id14 增量行：被扣的材料行、金币行、**新成品所在的背包槽行**。
//     最后那一行是"成品掉进背包"的落点，缺了它玩家看不到成品。
//  4. 名望（新秘宝要进名望结算）→ appendFameUpdate。
//
// 失败分支：**仍然发同一个 2289 回包**（状态 1、回显请求的成品模板）。
// 理由与 2288 相同：2289 的失败语义（状态 0 + 错误码）**没有任何反编译依据**，发一个
// 无依据的失败码风险更大；回显包至少能让客户端复位等待态。真实原因写进
// `sole_create_refused` 事件。
func (s *equipmentSession) raiseSoleCreate(service *workflow.WearService, w *worldSession, p, raw []byte, event func(map[string]any)) ([]outboundPacket, time.Duration, error) {
	started := time.Now() // 探针：见文件末尾的 ack 时机对照实验
	if service == nil || w == nil || w.role.ID == 0 || w.activeDungeon != nil {
		return nil, 0, inventory.Refuse(inventory.RefusalItems, "秘宝制作需要已选角色且位于城镇")
	}
	r, err := protocol.DecodeSoleCreate(p)
	if err != nil {
		return nil, 0, err
	}
	key, err := s.requestKey(raw)
	if err != nil {
		return nil, 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, out, err := service.ApplySoleCreate(ctx, w.role, "sole-create:"+key, r)
	if err != nil {
		// 失败也要回同 id 包，否则客户端制作窗口会卡在等待态（见文件头注释）。
		event(map[string]any{"kind": "sole_create_refused", "character_id": w.role.ID,
			"template": r.Template, "selector": r.Selector, "reason": err.Error(),
			"request_hex": hex.EncodeToString(p), "payload_offset": r.PayloadOffset})
		return []outboundPacket{{"sole_create_ack_failed", 1, protocol.SoleCreateOpcode,
			protocol.SoleCreateReply(r.Template)}}, 0, err
	}
	w.role = saved

	plan := []outboundPacket{{"sole_create_ack", 1, protocol.SoleCreateOpcode,
		protocol.SoleCreateReply(out.Template)}}

	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return plan, 0, err
	}
	var rows [][protocol.CurrentItemRecordSize]byte
	storageTouched := false
	for _, spent := range out.Spent {
		if spent.Template == 0 {
			continue // 金币行在下面统一处理
		}
		if spent.StorageAmount > 0 {
			storageTouched = true
		}
		// ⚠️ **不因为"这个模板属于账号材料品类"就跳过背包行**：账号材料被清扫进仓库之前
		// 就是以背包堆叠存在的，那时被扣掉的是背包那一份（2026-10-02 精度提升实机踩过）。
		if slot, ok := bagSlotOfTemplate(bag, spent.Template); ok {
			rows = append(rows, bagRowOrEmpty(bag, slot))
		}
	}
	if out.Spent != nil {
		// 扣过金币就刷新金币行（与装备调适、精度提升同一约定）。
		if goldRow, ok := bag.RowAt(0); ok {
			rows = append(rows, goldRow)
		}
	}
	// 成品所在槽行：用户诉求「成品秘宝要掉落到背包内」的落点，缺了它客户端看不到成品。
	rows = append(rows, bagRowOrEmpty(bag, out.Slot))
	// ⚠️ **顺序关键**：账号材料刷新（list35 → list42 → **list0**）必须排在 id14 增量行**之前**。
	if storageTouched {
		if counts, e := service.Store.AccountMaterials(ctx, saved.AccountID); e == nil {
			if materials, e2 := inventory.ReadAccountMaterials(counts); e2 == nil {
				refresh, e3 := accountMaterialRefreshPackets(materials, saved)
				if e3 == nil {
					plan = append(plan, refresh...)
				}
			}
		}
	}
	plan, err = appendEquipmentUpdates(plan, saved.State, rows, 0,
		"sole_create_inventory", "sole_create_worn")
	if err != nil {
		return plan, 0, err
	}
	spentDetail := make([]map[string]any, 0, len(out.Spent))
	for _, spent := range out.Spent {
		spentDetail = append(spentDetail, map[string]any{
			"template": spent.Template, "amount": spent.Amount,
			"from_storage": spent.FromStorage, "storage_amount": spent.StorageAmount})
	}
	event(map[string]any{"kind": "sole_create_committed", "character_id": saved.ID,
		"template": out.Template, "slot": out.Slot, "selector": r.Selector,
		"group_index": out.GroupIndex, "gold": out.Gold, "spent": spentDetail,
		"movie_time": out.MovieTime, "wait_time": out.WaitTime,
		"handle_ms": time.Since(started).Milliseconds(),
		"payload_offset": r.PayloadOffset, "request_hex": hex.EncodeToString(p)})
	plan = w.appendFameUpdate(plan, event)
	// —— 结果刷新包的时机实验（2026-10-02）——
	//
	// 实机（用户观察）：三件里只有时长最短的 Nabel 播了演出；另两件"结算很快就结束"、
	// 制作完毕的瞬间各有一次小卡帧 —— 看起来是**演出刚起来就被结果刷新包收掉了**。
	// 服务端这侧三次的处理路径与入站帧完全一致；上一版把**整批** plan 延后十几秒也无效
	// （那次还因为演出时长解析成 0 而静默失效，见 catalog 里 `create movie time` 的注释）。
	//
	// 所以改成：**ack 立即发**（必须，否则客户端清不掉等待态、窗口卡死），
	// **其余刷新包延后 followDelay 再由 main.go 发出** —— 分段发送在 main.go 的 2289 分支。
	// 默认 followDelay = 该件源里的 `[create movie time]`，`DFO_SOLE_CREATE_FOLLOWUP_DELAY=0` 关闭。
	followDelay := soleCreateFollowupDelay(out.WaitTime, out.MovieTime)
	event(map[string]any{"kind": "sole_create_ack_flushed", "template": out.Template,
		"movie_time": out.MovieTime, "wait_time": out.WaitTime, "packets": len(plan),
		"followup_delay_ms": followDelay.Milliseconds(),
		"total_ms":          time.Since(started).Milliseconds()})
	return plan, followDelay, nil
}

// soleCreateFollowupDelay 决定 ack 之后的结果刷新包要不要延后 —— **默认 0（与 ack 一起立即发）**。
//
// 背景（2026-10-02 对照实验）：实机发现三件秘宝里**只有 Nabel 播了制作演出**，另两件
// "结算很快就结束"、制作完毕瞬间各有一次小卡帧（像**演出刚起来就被结果包收掉**）。于是把
// 结果刷新包延后到该件自己的演出时长之后再发（Venus 18s / Nabel 13.5s / Diregie 18.8s）。
//
// **结论：无效** —— 三件里仍然只有 Nabel 播。真正的原因是 Client 资源：
// `Sole\Venus\*` 与 `Sole\Diregie\*` 那 4 个 `.bk2` 是**裸 Bink2**（开头就是 "KB2n"，缺那
// 32 字节 `Neople Video Fil` 头；而全客户端 203 个 `.bk2` 里其余 199 个都带这个头）。
// 把 Nabel 的文件复制成 Venus/Diregie 的名字时**能正常播**，证明加载路径没问题、是文件本身；
// 手工补那 32 字节头也**不还原**（帧数据本身也有差异）。属**客户端资源替换**范畴，
// 服务端不再介入（详见 analysis/tasks/next153 的排查记录）。
//
// 保留这个开关只为以后能一键复现该对照：`DFO_SOLE_CREATE_FOLLOWUP_DELAY=<毫秒>`。
// 各件秘宝的演出时长仍会从源里解析并写进事件（movie_time / wait_time），便于排查。
func soleCreateFollowupDelay(_, _ int) time.Duration {
	if v, ok := os.LookupEnv("DFO_SOLE_CREATE_FOLLOWUP_DELAY"); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return 0
}
