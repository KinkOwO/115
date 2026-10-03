package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"log"
)

// NOTI2634「每角色一对 Set Point / Oath Point」的组装与推送。
//
// 取证与结论（详见 docs/protocol/oath-set-points-20261004.md §7）：
//   - 客户端**不自己算总分**：它把服务端推来的这一对值原样写进角色实体
//     （handler `sub_1452C9840` → `sub_145F06760(实体)` → `sub_147435FF0(实体+128, A, B)`
//     → `实体+1872 = A`、`实体+1876 = B`），誓约页签显示的就是这两个数；
//   - 我们此前**从未发过 2634**（opcode 全表里也没有对应的 C2S 查询包）⇒ 用户看到
//     `0/750次` 与「已添加的 誓约积分: 0」。这就是"誓约没有积分"的直接根因；
//   - **逐件**分数来自 PVF：`etc/115lvability2/oathpointinfo.cos`（誓约/晶体按**模板**索引，
//     未调适也有基础分）与 `etc/115lvability/setpointinfo.cos`（按客户端物品类别号索引，
//     该"模板 → 类别号"映射仍在取证 ⇒ 当前 SetPoint 记 0 并打日志，不冒充"角色积分为 0"）。
//
// oathPartSetPointKey 是 NOTI2634 载荷里的 u16 实体键，**已由实机定案**。
//
// 取证链（pt36/pt40/pt41）：客户端"自身部位对象"是用**印章解码出来的明文**当键创建的 ——
// `sub_14023D2F0` → `sub_146E920A0(&dword_14EF2CA00,&key)` → `sub_145F0AB10(reg,key)`；
// 印章的初始化明文就是 `0xFFFF`（`sub_14005D4C0`: `mov [rsp+arg_0],0FFFFh; call sub_146E922E0`，
// 校验和 `dword_14EF2CA04 = 明文 + 存储值 + 196`）。
//
// **实机验证（2026-10-04）**：用"每把候选带可区分数值"的探针跑一次 —— 誓约页签出现 **670**
// （= 下标 0 的那个值，即 0xFFFF）⇒ 键确认为 `0xFFFF`，写入路径与字段顺序同时被证实。
// 探针（多候选 + 高位指纹）已按 server/AGENTS §6 一并删除。
const oathPartSetPointKey uint16 = 0xFFFF

// oathPointItems 取当前穿戴里**参与誓约积分**的件：晶体 36..46 + 誓约核心 47。
//
// 调适档位取自装备实例行 `Record[170]`（与名望计算 `internal/character/fame.go` 同一格：
// 181 字节记录的 +170 映射到实例 +285 的调适阶段）。没有实例 record 的老件按 0 档算 ——
// 源表 0 档同样有基础分，所以这不会把分数抹成 0；档位行查不到时聚合函数会再回退 0 档。
func oathPointItems(state []byte) ([]catalog.PointItem, error) {
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return nil, err
	}
	var items []catalog.PointItem
	for _, w := range bag.Worn {
		if w.Slot < 36 || w.Slot > 47 {
			continue
		}
		awakening := int32(0)
		if len(w.Record) > 170 {
			awakening = int32(w.Record[170])
		}
		items = append(items, catalog.PointItem{
			Template:     w.Template,
			Awakening:    awakening,
			PartSetIndex: -1,
		})
	}
	return items, nil
}

// oathPointPackets 组 NOTI2634 的候选帧。算不出规则（未装载积分表）时返回 nil ——
// 宁可不发，也不发一对 0 把"未知"写成"该角色积分为 0"。
func (w *worldSession) oathPointPackets() []outboundPacket {
	if w == nil || w.items == nil || w.items.Points == nil || len(w.role.State) == 0 {
		return nil
	}
	items, err := oathPointItems(w.role.State)
	if err != nil {
		log.Printf("oath points: cannot read worn items: %v", err)
		return nil
	}
	oath, hits := w.items.Points.OathPoints(items)
	setPoints, setHits := w.items.Points.SetPoints(items)
	log.Printf("oath points: oath=%d (%d/%d worn oath items) set=%d (%d hits; set-point table key still unverified)",
		oath, hits, len(items), setPoints, setHits)
	// 载荷顺序按**实机**定案（2026-10-04）：`u16 键 + u32 A + u32 B`，A 落 `实体+1872`
	// （誓约页签读它）、B 落 `实体+1876`（"套装积分"读它）。实机把 A=0/B=670 发出去后，
	// 客户端把 670 显示在「套装积分」、誓约页签仍为 0 ⇒ 顺序与最初假设相反；
	// 对调后誓约页签显示 670（用户实机确认）。
	return []outboundPacket{{
		"oath_part_set_point", 0, protocol.PartSetPointOpcode,
		protocol.PartSetPoint(oathPartSetPointKey, oath, setPoints),
	}}
}
