package main

// 活动 685「奥拉蔻的装备模块研究」面板的帧构造与常量。
//
// ## 与 1093 的关系（同一个活动的两块）
//
//	1093 = `AdditionalDropRewardEvent.ctp`  神秘能量**掉落**（帧 2880/2881）
//	685  = `OracleSimulationResearch.ctp`   面板的**生产 / 组装 / 分解**（帧 2806–2809）
//
// 面板本体是 `Live/Event/Kor/2026/0326_OracleResearch/main.xui`（685 的 `[ui main]`），
// 客户端要窗口 3956 有对象才能刷新（2806 handler 结尾取它）⇒ 与 1294/1379 一样，
// 这些帧必须排在进程序列里"窗口已建立"之后。
//
// ## 本轮只做两帧
//
//	2806 ETC            21 B（整块 → obj+1240）
//	2809 CREATION_BOARD 50 B（10 格 × (u8 tag + u32)）
//
// 2807/2808 是**变长**（先 u32 条数再逐条），要的是"玩家背包里的模块清单"；模块从哪来还没闭环，
// 本轮不发。
//
// ## 未证字段的处置
//
// 两帧字段语义都没证 ⇒ 取值做成启动期开关（**只设在那次启动 cmd 会话的环境里**，
// 不要写进 `configs/pvf-default.json` —— `internal/launcher/profile.go` 的 environment 是严格白名单）：
//
//	DFO_ORACLE_PRODUCED=<十进制>      # 只覆盖"本周已产次数"这一个字段（A/B 用）
import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/oracleresearch"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	// simResearchEtcNotiID 是 NOTI2806（SIMULATION_RESEARCH_EVENT_ETC）。
	simResearchEtcNotiID uint16 = 2806
	// simResearchCreationBoardNotiID 是 NOTI2809（…_CREATION_BOARD）。
	simResearchCreationBoardNotiID uint16 = 2809
	// simResearchEventID 是活动号（op108 行与后续 C→S 命令都指它）。
	simResearchEventID uint16 = 685
	// simResearchInvenAllNotiID 是 NOTI2807（整份模块清单）：u16 条数 + 每条 7 B。
	simResearchInvenAllNotiID uint16 = 2807
	// simResearchInvenItemNotiID 是 NOTI2808（单件明细，带槽位）：u16 条数 + 每条 9 B。
	simResearchInvenItemNotiID uint16 = 2808
	// simResearchWindowID 是面板窗口 id（2806 handler 结尾取它刷新）。
	simResearchWindowID uint16 = 3956
)

// simResearchEtcPayload 编 NOTI2806 的载荷。
//
// ★ 缺省 = **照抄客户端 handler 自己铺的默认**（`sub_1404243D0` 的正文前置）：
//
//	memset(v4, 0, 17);  *(_DWORD *)((char *)&v4[2] + 1) = 2;
//
// 也就是 **17 字节 0 + u32 = 2**。
//
// ⚠️ 2026-10-12 实机教训：一开始送的是 **21 个 0**，结果客户端弹
// "Please try again later." 并且**关掉就再弹、陷入循环**，直到把整个面板关掉 ——
// 说明最后那个 u32 **不是"随便填 0"的东西**（0 被当成非就绪态）。
// 照抄客户端自己的默认值是最保守的取值；要 A/B 用 `DFO_ORACLE_PRODUCED`（只改一个数）。
// simResearchEtcPayload 编 NOTI2806 的载荷（21 字节）。
//
// ★ 2026-10-12 定案：2706 的字段落位（构造器 `sub_140422640` + 实测）——
//
//	[+1240: 8B][+1248: 4B][+1252: 1B][+1253: 4B][+1257: 4B = 2]
//
// 其中 **`+1244`（= payload 第 4~7 字节）与 `+1253`（= 第 13~16 字节）在实测里都表现为
// "已生产次数"** —— 硬编码 1000 时面板按钮显示 `10 × (1000+1) = 10010`（业主截图）✓✓
// ⇒ 这里**填服务端的真实次数**（两处都填，先把 UI 修对；哪个才是正主可后续二分）。
func simResearchEtcPayload(produced uint32) []byte {
	// ★★ 2026-10-12 血泪教训：这里**原先**有一个 `DFO_ORACLE_ETC` 的「整块 21 字节覆盖」口子。
	// 业主在**用户级环境变量**里留了 `DFO_ORACLE_ETC=…e8030000…`（produced=1000 的探针值），
	// 于是**我们算出来的真值永远被它盖掉** —— 面板费用恒为 `10 × (1000+1) = 10010`，
	// 而「改代码 / 改数值都不生效」看起来像客户端 bug，白查了一整天。
	// ⇒ 整块覆盖已**删除**：它能让任何一处形状（如 `[17:21]=2`）被静默改坏。
	// 要 A/B 只改 **produced 这一个数**，不可能伤到别的字段：
	if spec := strings.TrimSpace(os.Getenv("DFO_ORACLE_PRODUCED")); spec != "" {
		if n, perr := strconv.ParseUint(spec, 10, 32); perr == nil {
			produced = uint32(n)
		}
	}
	raw := make([]byte, protocol.SimulationResearchEtcBytes)
	binary.LittleEndian.PutUint32(raw[4:8], produced)
	binary.LittleEndian.PutUint32(raw[13:17], produced)
	binary.LittleEndian.PutUint32(raw[17:21], 2)
	return protocol.SimulationResearchEtc115(raw)
}

// simResearchBoardUsedTag 返回「结果格已用」的 tag。
//
// 官方语义（UI 帮助原文）：组装槽**每个自然周每行只能用一次**、按周二 09:00 UTC 重置；
// 而 `sub_1404264C0` 的判据是 `结果格 tag != 0 ⇒ 该行不可用` ⇒ **未用 = 0**。
// ⚠️ 「已用」具体写哪个非 0 值是**推断**（反编译里 `cmp ...,1` 与客户端默认的 2 都出现过），
// 所以留环境变量做 A/B：`DFO_ORACLE_BOARD_USED_TAG`（默认 2，候选 1/2）。
func simResearchBoardUsedTag() byte {
	tag := byte(2)
	if v := strings.TrimSpace(os.Getenv("DFO_ORACLE_BOARD_USED_TAG")); v != "" {
		if n, e := strconv.ParseUint(v, 10, 8); e == nil {
			tag = byte(n)
		}
	}
	return tag
}

// simResearchCreationBoardPayload 编 NOTI2809 的载荷（10 格 × (u8 tag + u32 value)）。
//
// 板结构是实证的（`sub_1404264C0`）：第 r 行的三格 = `3r / 3r+1 / 3r+2` = (套装, 部位, 产物)，
// 且**判定只看结果格**（第 2/5/8 格）。每格的含义：
//
//	结果格 tag   = 0 表示本自然周还没用过；非 0 表示已用（组装后该槽失效）
//	结果格 value = **该槽解锁所需的「已产次数」**（脚本 `[combine]` 的 `[args]`，本机 0/10/15）
//	              —— 对应官方原文 "Assemble Slots are unlocked sequentially based on the
//	              Module Produce count."，以及 `sub_1404264C0` 里与 `obj+1253` 的那次比较
//	材料格       = 保持客户端自己的中性值 (tag=2, value=0)：判定不看它们，写 0 反而可能
//	              把玩家正在注册的内容清掉
func simResearchCreationBoardPayload(used [3]bool, unlock []uint32) []byte {
	usedTag := simResearchBoardUsedTag()
	out := make([]byte, protocol.SimulationResearchCreationBoardBytes)
	put := func(cell int, tag byte, value uint32) {
		out[cell*5] = tag
		binary.LittleEndian.PutUint32(out[cell*5+1:cell*5+5], value)
	}
	for r := 0; r < 3; r++ {
		put(3*r, 2, 0)
		put(3*r+1, 2, 0)
		tag := byte(0)
		if used[r] {
			tag = usedTag
		}
		var need uint32
		if r < len(unlock) {
			need = unlock[r]
		}
		put(3*r+2, tag, need)
	}
	put(9, 2, 0)
	return out
}

// simResearchModuleEntries 从背包里挑出**本活动的模块**，编成 2807 的条目。
//
// 「本活动的模块」由目录的 `Modules` 清单判定（24 个），不写死区间 —— 源换季时自动跟着变。
// `Flag` 先恒 1（"这一格有东西"）、`Value` 填堆叠数量（两者语义未证，见协议文件注释）。
func simResearchModuleEntries(cat *oracleresearch.Catalog, bag inventory.Bag) []protocol.SimulationResearchModuleEntry115 {
	if cat == nil {
		return nil
	}
	known := make(map[uint32]bool, len(cat.Modules))
	for _, id := range cat.Modules {
		known[id] = true
	}
	// ⚠️ `flag` / `值` 的语义**从未被证实**（只知道宽度 u8 / u16）。
	// 2026-10-12 实测：`reply` 用全 0 与用次数**两种都试过**、提示都在；**2806 关掉也还在**
	// ⇒ 嫌疑锁定在**本帧的内容字段**。所以留两个开关做扫描（不用重编）：
	//
	//	DFO_ORACLE_INVEN_FLAG=0|1        缺省 1
	//	DFO_ORACLE_INVEN_VALUE=count|one|zero   缺省 count（堆叠数量）
	flag := byte(1)
	if strings.TrimSpace(os.Getenv("DFO_ORACLE_INVEN_FLAG")) == "0" {
		flag = 0
	}
	valueMode := strings.ToLower(strings.TrimSpace(os.Getenv("DFO_ORACLE_INVEN_VALUE")))
	var out []protocol.SimulationResearchModuleEntry115
	for _, row := range bag.Items {
		if !known[row.Template] || row.Amount == 0 {
			continue
		}
		v := row.Amount
		switch valueMode {
		case "one":
			v = 1
		case "zero":
			v = 0
		}
		if v > 0xFFFF {
			v = 0xFFFF
		}
		out = append(out, protocol.SimulationResearchModuleEntry115{ID: row.Template, Flag: flag, Value: uint16(v)})
	}
	return out
}

// simResearchCombineRequest 是 CMD2413（组装）请求正文解出来的内容。
type simResearchCombineRequest struct {
	// s0/s1/s2 是客户端给的"行三元组"：(3*行号, 3*行号+1, 3*行号+2)。
	// 它对应脚本 `[combine] [slot]` 9 个槽位里第 行号 行的三个（2 个 material + 1 个 result）。
	s0, s1, s2 uint16
	// SetSlot / PartSlot 是两件模块的**槽位号**（脚本 `[production] [slot]` 的右半）。
	SetSlot  uint32
	PartSlot uint32
}

// SimResearchCombineRow 返回行号（0..2）。
func (r simResearchCombineRequest) Row() uint32 { return uint32(r.s0) / 3 }

// simResearchCombineRequest115 解析 CMD2413 的请求正文。
//
// 依据（2026-10-12，IDA `sub_140426970` + 三次实机样本逐条对齐）：
//
//	v19[0] = 3 * a2;                          // a2 = 行号
//	v19[1] = 3 * a2 + 1;
//	v19[2] = sub_140425090(obj, 套装模块号);   // 模块号 → 槽位号
//	v19[3] = sub_140425090(obj, 部位模块号);
//	v19[4] = 3 * a2 + 2;
//	sub_146D75B10(buf, v19, 10);              // 正文只写 10 字节
//
// 实机三帧（登录后三次组装，行 0/1/2）：
//
//	0000 0100 0100 0f00 0200 ...   → 行0，套装槽 1，部位槽 15
//	0300 0400 0a00 0f00 0500 ...   → 行1，套装槽 10，部位槽 15
//	0600 0700 0b00 1000 0800 ...   → 行2，套装槽 11，部位槽 16
//
// ⚠️ 行三元组必须**自洽**才接受（3 的倍数 + 连续 + 行号 ≤ 2）：宁可拒，也不要按错位的槽位发成品。
func simResearchCombineRequest115(plaintext []byte) (simResearchCombineRequest, error) {
	var out simResearchCombineRequest
	if len(plaintext) < 10 {
		return out, fmt.Errorf("2413 正文只有 %d 字节，期望 ≥10", len(plaintext))
	}
	out.s0 = binary.LittleEndian.Uint16(plaintext[0:2])
	out.s1 = binary.LittleEndian.Uint16(plaintext[2:4])
	out.SetSlot = uint32(binary.LittleEndian.Uint16(plaintext[4:6]))
	out.PartSlot = uint32(binary.LittleEndian.Uint16(plaintext[6:8]))
	out.s2 = binary.LittleEndian.Uint16(plaintext[8:10])
	if out.s0%3 != 0 || out.s1 != out.s0+1 || out.s2 != out.s0+2 || out.s0/3 > 2 {
		return out, fmt.Errorf("2413 行三元组不自洽: (%d,%d,%d)", out.s0, out.s1, out.s2)
	}
	return out, nil
}

// simResearchDisjointEntry 是 CMD2414（分解）请求里的一条：模块槽位号 + 件数。
type simResearchDisjointEntry struct {
	Slot  uint16
	Count uint16
}

const (
	// simResearchDisjointSlots 是正文里的格子数（30 × 4 B = 120 B）。
	simResearchDisjointSlots = 30
	// simResearchDisjointEmptySlot 是空格子的槽位号（客户端自己铺的默认值）。
	simResearchDisjointEmptySlot = 0xFFFF
)

// simResearchDisjointRequest115 解析 CMD2414 的请求正文。
//
// 依据（2026-10-12，反编译 `sub_140426BD0` 的 `(v2 & 0x80)` 分支 + 实机样本）：
//
//	do { *(u32*)v17 = 0xFFFF; v17 += 2; --v18; } while (v18);   // 先铺 30 格 (slot=0xFFFF, count=0)
//	*(v22 - 1) = sub_140425090(a1, *v20);   // 模块号 → 槽位号 (u16)
//	*v22 = *((u16*)v20 + 2);                // 件数 (u16)
//	sub_146D75B10(buf, &v28, 120);          // 正文 120 字节
//
// 实机样本（只填了第 1 格）：
//
//	0700 0100 ffff0000 ffff0000 …   → {槽 7, 件数 1}，其余 29 格 (0xFFFF, 0)
//
// ⚠️ 全空必须拒绝：客户端收到应答后会清空自己的选中列表，全空说明这次没有任何选择。
func simResearchDisjointRequest115(plaintext []byte) ([]simResearchDisjointEntry, error) {
	need := simResearchDisjointSlots * 4
	if len(plaintext) < need {
		return nil, fmt.Errorf("2414 正文只有 %d 字节，期望 ≥%d", len(plaintext), need)
	}
	var out []simResearchDisjointEntry
	for i := 0; i < simResearchDisjointSlots; i++ {
		slot := binary.LittleEndian.Uint16(plaintext[i*4 : i*4+2])
		count := binary.LittleEndian.Uint16(plaintext[i*4+2 : i*4+4])
		if slot == simResearchDisjointEmptySlot || count == 0 {
			continue
		}
		out = append(out, simResearchDisjointEntry{Slot: slot, Count: count})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("2414 正文里没有任何有效条目（%d 格全空）", simResearchDisjointSlots)
	}
	return out, nil
}
