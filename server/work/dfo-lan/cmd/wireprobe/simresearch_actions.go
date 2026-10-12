package main

// 活动 685 的三条**动作命令**（C→S）的派发段：生产 / 组装 / 分解。
//
//	cmd 2412 SIMULATION_RESEARCH_EVENT_PRODUCTION         生产
//	cmd 2413 SIMULATION_RESEARCH_EVENT_COMBINE            组装
//	cmd 2414 SIMULATION_RESEARCH_EVENT_DISJOINT_STACKABLE 分解
//
// ## 为什么第一步只回"中性应答"
//
// 逆出来的事实（`ida_probe_685_cmd.py`）：三条命令的客户端 handler 在服务端方向分支里
// **都会读一段定长正文并刷新窗口 3956**，公共头还清 `obj+2114`（"进行中"标志）
// ⇒ **服务端不回应答，按钮就一直卡在在途**（实机 2026-10-12：点了没反应）。
//
// 所以这一步的目标是**把在途状态收掉、让按钮有反馈**：
// 按逆出来的长度回一条**同 id、type-1** 的帧，取值中性（全 0），
// 并留环境变量开关做 A/B（`DFO_ORACLE_PRODUCTION` / `_COMBINE` / `_DISJOINT`）。
// ⚠️ 这三个是"整块覆盖"：**做完全部 A/B 必须确认它们未设置**（2026-10-12 就是被同一类
// 遗留环境变量坑了一整天 —— 见 simresearch_flow.go 的教训注释）。
//
// ⚠️ **业务逻辑（扣能量 / 发模块 / 组装 / 分解）不在本文件** —— 那是下一步（685-3）。
// 在它落地之前，这里的应答只是"收到了，但没有产出"，**不会给任何物品**。
import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/oracleresearch"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"time"
)

// oracleActionCommand 描述一条动作命令的应答形状。
type oracleActionCommand struct {
	ID    uint16
	Label string
	Bytes int
	// Env 是这个命令 A/B 覆盖用的环境变量名。
	Env string
}

// 生产邮件的发信人与正文。
// TODO(685-3)：ctp 的 `[production] [mail info]` 有两条文案（字符串池引用），
// 等确认它们对应"标题/正文"之后再改成从目录读；先给一个可读的固定值。
const (
	oracleProductionMailSender = "Dr. Oracle"
	oracleProductionMailText   = "Your module research is complete."
	// 分解的结果**走邮件**（客户端 2414 handler 回包后弹的 dstr 101039452 明写
	// "Items will be sent by mail."），文案同样先给固定值。
	oracleDisjointMailSender = "Dr. Oracle"
	oracleDisjointMailText   = "Your module dismantling is complete."
)

var oracleActionCommands = map[uint16]oracleActionCommand{
	2412: {2412, "production", protocol.SimulationResearchProductionReplyBytes, "DFO_ORACLE_PRODUCTION"},
	2413: {2413, "combine", protocol.SimulationResearchCombineReplyBytes, "DFO_ORACLE_COMBINE"},
	2414: {2414, "disjoint", protocol.SimulationResearchDisjointReplyBytes, "DFO_ORACLE_DISJOINT"},
}

// oracleActionReply 按命令号编应答载荷（中性全 0，可用环境变量送一份十六进制覆盖）。
func oracleActionReply(cmd oracleActionCommand) []byte {
	raw := make([]byte, cmd.Bytes)
	if spec := strings.TrimSpace(os.Getenv(cmd.Env)); spec != "" {
		if b, err := hex.DecodeString(strings.ReplaceAll(spec, " ", "")); err == nil {
			raw = b
		}
	}
	switch cmd.ID {
	case 2412:
		return protocol.SimulationResearchProductionReply115(raw)
	case 2413:
		return protocol.SimulationResearchCombineReply115(raw)
	default:
		return protocol.SimulationResearchDisjointReply115(raw)
	}
}

// oracleReplySub 是应答**第 1 个 u16** 的取值。
//
// 依据（IDA `sub_140423D90` 2412 / `sub_1404234E0` 2413 的 `a2 == 0` 分支）：
// 白名单就两种 —— `{22, 101, 118, 320}`，不在里面就弹 dstr 33187 =
// "Please try again later."（业主要的那句提示就是这么来的）。两条命令**同一套**白名单。
func oracleReplySub() uint16 {
	sub := uint16(22)
	if v := strings.TrimSpace(os.Getenv("DFO_ORACLE_REPLY_SUB")); v != "" {
		if n, e := strconv.ParseUint(v, 10, 16); e == nil {
			sub = uint16(n)
		}
	}
	return sub
}

// dispatchSimResearchAction 是入站派发链里的一站：只管 2412/2413/2414 三条。
//
// 与 313 的领奖段同形：只认已解密已判校验和的 type==1 帧，别的原样放行（dispatchNext）。
func (client *gameConnection) dispatchSimResearchAction(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type != 1 || !requestData.verified || !client.bootstrapped {
		return dispatchNext
	}
	cmd, ok := oracleActionCommands[requestData.frame.ID]
	if !ok {
		return dispatchNext
	}
	reply := oracleActionReply(cmd)
	// ★ 2412 生产：真正扣能量 + 推进计数 + 发邮件（685-3）。
	// 应答 10 B —— 客户端取**第 2 个 u32** 喂 sub_140613BE0(窗口 3956, 值) ⇒ 放**新的已产次数**。
	producedAfter := uint32(0)
	// combineUpdate 是组装成功后给客户端的 N14（背包变化行），在下面统一进 plan。
	var combineUpdate []byte
	if cmd.ID == 2412 {
		if w := client.worldState; w != nil && w.oracle685 != nil && w.store != nil {
			svc := &workflow.Oracle685Service{Store: w.store, Catalog: w.oracle685,
				Loot: w.loot.Catalog, Bag: w.loot.BagRules, Equipment: w.loot.Equipment}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, receipt, applied, perr := svc.Produce(ctx, w.role, time.Now(),
				oracleProductionMailSender, oracleProductionMailText)
			cancel()
			if perr != nil {
				client.event(map[string]any{
					"kind": "oracle_production_refused", "character_id": w.role.ID,
					"error":       perr.Error(),
					"request_hex": hex.EncodeToString(requestData.plaintext),
				})
				// 拒绝也回应答（长度必须对，否则客户端的在途状态收不掉）：全 0 = 次数不变。
				reply = oracleActionReply(cmd)
			} else {
				// 应答第 2 个 u32 = **新的已产次数**。
				// 证据（2026-10-12，探针 `ida_probe_notice.py`）：`sub_140613BE0(win, 值)` 干的只是
				// ```c
				//   *(win + 2392) = 值;  *(win + 2400) = 2;  然后刷新
				// ```
				// ⇒ 它是**存值**（不是提示码通道）；`+2400 = 2` 那个 2 与 2806 handler 的默认值同源。
				// 所以这里就把它当成"面板要显示的那个数"（次数）。
				// ★★★ **门禁在应答自己的第 1 个 u16 上**（2026-10-12 定案，业主提示的"运行期探针"思路）：
				// `sub_140423D90(a1, a2, a3)` 的 `a2 == 0` 分支 = **收到的帧**，
				// 而 `a3` = **该帧的第 1 个 u16** ⇒ 不在 `{22, 101, 118, 320}` 就弹 dstr 33187。
				// 我们之前的应答前 2 字节一直是 00 00 ⇒ 必然弹；而改状态帧、改通道、改时序**全都无效**，
				// 正因为门禁在**应答自身**里。第 2 个 u32 仍是"存进窗口 +2392 的值"。
				sub := oracleReplySub()
				raw := make([]byte, protocol.SimulationResearchProductionReplyBytes)
				binary.LittleEndian.PutUint16(raw[0:2], sub)
				binary.LittleEndian.PutUint32(raw[4:8], receipt.Produced)
				reply = protocol.SimulationResearchProductionReply115(raw)
				producedAfter = receipt.Produced
				client.event(map[string]any{
					"kind": "oracle_production_done", "character_id": w.role.ID,
					"applied": applied, "week_start": receipt.WeekStart,
					"cost": receipt.Cost, "module": receipt.Module,
					"result_item": receipt.ResultItem, "result_count": receipt.ResultCount,
					"produced_after": receipt.Produced,
				})
			}
		}
	}
	// ★★ 2413 组装：**扣掉两件模块、把成品直接塞回背包**。
	// 请求正文 = 10 B = 5 × u16，解出来的两件模块见 simResearchCombineRequest115。
	// 成品走背包而不是邮件：脚本里 `[combine]` **没有** `[mail info]`
	//（`[production]` 与 `[disjoint stackable]` 才有）—— 这条依据写在 workflow.Combine 的注释里。
	if cmd.ID == 2413 {
		if w := client.worldState; w != nil && w.oracle685 != nil && w.store != nil {
			req, rerr := simResearchCombineRequest115(requestData.plaintext)
			if rerr != nil {
				client.event(map[string]any{
					"kind": "oracle_combine_refused", "character_id": w.role.ID,
					"error": rerr.Error(), "request_hex": hex.EncodeToString(requestData.plaintext),
				})
			} else {
				svc := &workflow.Oracle685Service{Store: w.store, Catalog: w.oracle685,
					Loot: w.loot.Catalog, Bag: w.loot.BagRules, Equipment: w.loot.Equipment}
				cctx, ccancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, creceipt, cerr := svc.Combine(cctx, w.role, int(req.Row()), req.SetSlot, req.PartSlot)
				ccancel()
				if cerr != nil {
					client.event(map[string]any{
						"kind": "oracle_combine_refused", "character_id": w.role.ID,
						"error": cerr.Error(), "request_hex": hex.EncodeToString(requestData.plaintext),
					})
				} else {
					// 应答 4 B：第 1 个 u16 与 2412 同一套门禁（见 oracleReplySub）。
					raw := make([]byte, protocol.SimulationResearchCombineReplyBytes)
					binary.LittleEndian.PutUint16(raw[0:2], oracleReplySub())
					reply = protocol.SimulationResearchCombineReply115(raw)
					// N14：把「扣掉的两件模块 + 新装备」一次性告诉客户端（与军团奖励同一条通道）。
					if upd, uerr := protocol.InventoryUpdate(creceipt.ChangedRows); uerr == nil {
						combineUpdate = upd
					}
					client.event(map[string]any{
						"kind": "oracle_combine_done", "character_id": w.role.ID,
						"row": req.Row(), "set_slot": req.SetSlot, "part_slot": req.PartSlot,
						"set_module": creceipt.SetModule, "part_module": creceipt.PartModule,
						"result_item": creceipt.ResultItem, "result_count": creceipt.ResultCount,
						"changed_rows": len(creceipt.ChangedRows), "rows_used": creceipt.RowsUsed,
					})
				}
			}
		}
	}
	// ★★ 2414 分解：按 (槽位号, 件数) 扣模块，产出 Doom Oracle Box，**走系统邮件**。
	// 请求正文 = 120 B = 30 × (u16 槽位号, u16 件数)，空格子 (0xFFFF, 0)。
	if cmd.ID == 2414 {
		if w := client.worldState; w != nil && w.oracle685 != nil && w.store != nil {
			entries, eerr := simResearchDisjointRequest115(requestData.plaintext)
			if eerr != nil {
				client.event(map[string]any{
					"kind": "oracle_disjoint_refused", "character_id": w.role.ID,
					"error": eerr.Error(), "request_hex": hex.EncodeToString(requestData.plaintext),
				})
			} else {
				picks := make([]workflow.DisjointPick, 0, len(entries))
				for _, e := range entries {
					picks = append(picks, workflow.DisjointPick{Slot: uint32(e.Slot), Count: uint32(e.Count)})
				}
				svc := &workflow.Oracle685Service{Store: w.store, Catalog: w.oracle685,
					Loot: w.loot.Catalog, Bag: w.loot.BagRules, Equipment: w.loot.Equipment}
				dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, dreceipt, dapplied, derr := svc.Disjoint(dctx, w.role, picks,
					oracleDisjointMailSender, oracleDisjointMailText)
				dcancel()
				if derr != nil {
					client.event(map[string]any{
						"kind": "oracle_disjoint_refused", "character_id": w.role.ID,
						"error": derr.Error(), "request_hex": hex.EncodeToString(requestData.plaintext),
					})
				} else {
					// 应答 12 B：
					//   [0:2]  = 门禁子码（与 2412/2413 同一套）
					//   [4:8]  = **获得物品的物品号** —— handler 里 `sub_14500A970(HIDWORD(v19))`
					//            用它去查 名称/数量，然后格式化进 dstr 101039452
					//            （"…Obtained %d %s. Items will be sent by mail."）弹出来。
					//            ⚠️ 2026-10-12 实机：这格填 0 时弹框显示 **"Obtained 0 NONE"**。
					//   [8:10] = 获得数量（同一个弹框的 %d）
					obtained := dreceipt.ResultCount
					if obtained > 0xFFFF {
						obtained = 0xFFFF
					}
					raw := make([]byte, protocol.SimulationResearchDisjointReplyBytes)
					binary.LittleEndian.PutUint16(raw[0:2], oracleReplySub())
					binary.LittleEndian.PutUint32(raw[4:8], dreceipt.ResultItem)
					binary.LittleEndian.PutUint16(raw[8:10], uint16(obtained))
					reply = protocol.SimulationResearchDisjointReply115(raw)
					if upd, uerr := protocol.InventoryUpdate(dreceipt.ChangedRows); uerr == nil {
						combineUpdate = upd
					}
					client.event(map[string]any{
						"kind": "oracle_disjoint_done", "character_id": w.role.ID,
						"applied": dapplied, "modules": dreceipt.Modules,
						"result_item": dreceipt.ResultItem, "result_count": dreceipt.ResultCount,
						"changed_rows": len(dreceipt.ChangedRows),
					})
				}
			}
		}
	}
	// ★ 生产成功后**必须推状态帧**：实机 2026-10-12 证明客户端在生产后**不发任何刷新命令**
	// （只顺带发了与活动无关的 2423）⇒ 不推的话面板会一直停在"已用尽"，必须关界面重开才能再点。
	// 这里推 2806（面板状态 → obj+1240）与 2807（模块清单 → 图鉴点亮）。
	// ★ 2413（组装）也要重推 2807：它把两件模块扣掉了，图鉴/格子计数必须跟着降。
	var invenAll []byte
	// ★ 顺带把「本自然周哪些组装槽已用过」读出来（同一批角色行，不多花一次查询）。
	var boardUsed [3]bool
	if (cmd.ID == 2412 || cmd.ID == 2413 || cmd.ID == 2414) && client.worldState != nil &&
		client.worldState.store != nil && client.worldState.oracle685 != nil {
		rctx, rcancel := context.WithTimeout(context.Background(), 5*time.Second)
		if roles, rerr := client.worldState.store.Characters(rctx, client.worldState.account); rerr == nil {
			for _, r := range roles {
				if r.ID != client.worldState.role.ID {
					continue
				}
				if bag, berr := inventory.ReadBag(r.State); berr == nil {
					invenAll = protocol.SimulationResearchInvenAll115(
						simResearchModuleEntries(client.worldState.oracle685, bag))
				}
				if st, serr := oracleresearch.ReadState(r.State); serr == nil {
					st = st.Normalize(oracleresearch.WeekStartOf(time.Now()))
					boardUsed = st.RowsUsed
				}
				break
			}
		}
		rcancel()
	}
	client.event(map[string]any{
		"kind":          "oracle_action_received",
		"command":       cmd.Label,
		"id":            cmd.ID,
		"request_hex":   hex.EncodeToString(requestData.plaintext),
		"request_bytes": len(requestData.plaintext),
		"reply_bytes":   len(reply),
		"note":          "中性应答，仅收掉在途状态；业务逻辑见 685-3",
	})
	// ★ 2026-10-12 实测结论：**生产后什么都不推、只发这条应答 ⇒ 提示依旧**
	// ⇒ 提示来**自这条应答**，不是我们推的 S2C 帧。
	// 而 2412 在客户端**两个注册助手都有**（CMD 侧 `sub_140423D90` 读 10 B +
	// NOTI 侧 `sub_1417CB860`）⇒ 应答很可能要**按两条通道各发一份**（业主"组合帧"的判断）。
	// ★★ 2026-10-12 定案：**缺省只走 CMD 通道**（业主用 `DFO_ORACLE_REPLY_CH=cmd` 验过可用）。
	// 依据：`sub_1404234E0/90` 的 `a2 == 0` 分支是一张**本地错误文案表**
	//（22="Not enough materials."、101/118/320 同族、**default="Please try again later."**）。
	// 客户端是**用 CMD 发请求**的 ⇒ 应答也必须是 CMD；再镜像一份到 NOTI 会掉进那张表，
	// 于是左侧系统消息被塞进一句莫名其妙的英文 —— 业主最初看到的
	// "Please try again later." 就是这么来的（当时 u16[0]=0 正好命中 default）。
	// 要回退做 A/B：`DFO_ORACLE_REPLY_CH=both`。
	chMode := strings.ToLower(strings.TrimSpace(os.Getenv("DFO_ORACLE_REPLY_CH")))
	if chMode == "" {
		chMode = "cmd"
	}
	plan := []outboundPacket{}
	if chMode == "both" || chMode == "cmd" {
		plan = append(plan, outboundPacket{"oracle_action_reply_sent", 1, cmd.ID, reply})
	}
	if chMode == "both" || chMode == "noti" {
		plan = append(plan, outboundPacket{"oracle_action_reply_noti_sent", 0, cmd.ID, reply})
	}
	if cmd.ID == 2412 || cmd.ID == 2413 || cmd.ID == 2414 {
		// ★★ **超集实验**（业主 2026-10-12 12:0x 建议："先做一次范围更大的改动，成了再逐步收缩"）：
		// 生产成功后把**本子系统里所有该刷新的 S→C 帧**一次推齐，用开关逐条关掉做收缩。
		// 依据（`next201 §13` 的帧清单）：2806 ETC → obj+1240；2807 INVEN_ALL / 2809 CREATION_BOARD
		// 都会刷新**窗口 3956**。三帧各自的开关（默认全开）：
		//
		//	DFO_ORACLE_PUSH_ETC=off / _INVEN=off / _BOARD=off
		//
		// ⚠️ 开关只设在那次启动的 cmd 会话里（profile 的 environment 是严格白名单）。
		// 2026-10-12 收缩进度：提示的元凶已定案为**应答第 1 个 u16**（见上文门禁注释）
		// ⇒ 2806 与提示无关，且它是面板"下次生产费用"的唯一来源
		// （业主截图 10010 = 10×(1000+1) 的由来）⇒ **2806 恢复默认开**（与登录那条一致）。
		// 2807 默认开（图鉴彩点靠它刷新）；**2809 默认关**（组装格子内容未证）。
		// 逐条关掉做 A/B 用 `…=off`（只在那个 cmd 会话里设）。
		// ★ 2026-10-12：已证明**这三条都不是** "Please try again later." 的元凶
		// （那句提示的真相见 `ida_probe_dstr_refs.py` 的结论：客户端按自己算出的
		// 子类型 `a3` 不在 {22,101,118,320} 时自弹 dstr 33187）。
		// ⇒ **2807 恢复默认开**（图鉴靠它刷新，业主确认过）；2806/2809 仍默认关（内容未证）。
		push := func(env, def string) bool {
			v := strings.ToLower(strings.TrimSpace(os.Getenv(env)))
			if v == "" {
				v = def
			}
			return v == "on"
		}
		// ⚠️ 2806 只给 **2412**：它带的是「本周已产次数」，而 `producedAfter` 在 2413 路径上是 0
		// ⇒ 组装后推 2806 会把面板费用打回 10。（本仓今天的教训：字段被静默改坏会伪装成客户端 bug。）
		if cmd.ID == 2412 && push("DFO_ORACLE_PUSH_ETC", "on") {
			plan = append(plan, outboundPacket{"sim_research_etc_sent", 0, simResearchEtcNotiID, simResearchEtcPayload(producedAfter)})
		}
		// ★ 必须判 nil：`sendPlan` 走 sendPacketPlan，**不过滤 nil 载荷**
		// （只有 preparePackets 过滤）⇒ 把 nil 放进 plan 会发出一条**空体**的 2807，
		// 而 2807 handler 先读 u16 条数 ⇒ 会从**下一帧**借字节，整条流错位。
		// 读背包失败时宁可不推（并记事件）。
		if len(combineUpdate) > 0 {
			plan = append(plan, outboundPacket{"oracle_combine_inventory_update", 0, 14, combineUpdate})
		}
		if push("DFO_ORACLE_PUSH_INVEN", "on") {
			if invenAll == nil {
				client.event(map[string]any{
					"kind": "sim_research_inven_all_skipped", "character_id": client.selectedCharacterID,
					"reason": "module list unavailable",
				})
			} else {
				plan = append(plan, outboundPacket{"sim_research_inven_all_sent", 0, simResearchInvenAllNotiID, invenAll})
			}
		}
		// ★ 2809（CREATION_BOARD）的 handler 结尾会**刷新窗口 3956**（`sub_1404242F0`），
		// 所以它同时是"面板重绘"的载体。业主 2026-10-12 反馈：组装成功后 Get 按钮不复位、
		// 再点还弹确认框 —— 就是因为**组装后一次都没刷新过面板**（2807 只刷新图鉴，不重绘行）。
		// ⇒ 2412 与 2413 都推，并改为**默认开**（内容仍是客户端自己的中性默认：10 格 tag=2/value=0）。
		if push("DFO_ORACLE_PUSH_BOARD", "on") {
			var boardUnlock []uint32
			if client.worldState != nil && client.worldState.oracle685 != nil {
				boardUnlock = client.worldState.oracle685.CombineUnlock
			}
			plan = append(plan, outboundPacket{"sim_research_creation_board_sent", 0, simResearchCreationBoardNotiID, simResearchCreationBoardPayload(boardUsed, boardUnlock)})
		}
	}
	if client.sendPlan(plan, client.logWorldResponseBody) != nil {
		return dispatchClose
	}
	return dispatchHandled
}
