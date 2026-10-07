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
//     未调适也有基础分）与 `etc/115lvability/setpointinfo.cos`（套装积分，两层映射：
//     模板 →`etc/equipmentgrouping.etc` 的 `[ability group]`→ 能力组，再按
//     `[group] + [awakening]` 查表；`[part set index] = -1` 的行用本件套装号补）。
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

// wornPointItems 取当前穿戴里**参与套装积分**的件：与名望侧
// `character.fame.EquipmentFameBreakdown` 同一批槽位（排除副手 24/30 与幻化 11/32）。
//
// 为什么必须带 `PartSetIndex`：套装积分的两层映射是
// 「模板 →(equipmentgrouping.etc)→ 能力组」+「能力组+档位 →(setpointinfo.cos)→ 每件分」，
// 而 `setpointinfo` 里 `[part set index] = -1` 的行要用**本件**的套装号补上。少了它，
// 那些行一律被跳过（耳环就是这样：能力组 306 有行，但耳环没有 `[part set index]`，
// 所以它确实 0 分——这是源的性质，不是算错）。
func wornPointItems(service *inventory.ItemService, state []byte) ([]catalog.PointItem, map[uint32]uint16, error) {
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return nil, nil, err
	}
	slotOf := make(map[uint32]uint16, len(bag.Worn))
	rows := make([]inventory.BagEquipment, 0, len(bag.Worn))
	templates := make([]uint32, 0, len(bag.Worn))
	for _, w := range bag.Worn {
		if w.Template == 0 || w.Slot == 24 || w.Slot == 30 || w.Slot == 11 || w.Slot == 32 {
			continue
		}
		rows = append(rows, w)
		templates = append(templates, w.Template)
		slotOf[w.Template] = w.Slot
	}
	partSets, err := pointPartSetIndexes(service, templates)
	if err != nil {
		return nil, nil, err
	}
	items := make([]catalog.PointItem, 0, len(rows))
	for _, w := range rows {
		awakening := int32(0)
		if len(w.Record) > 170 {
			// 与名望计算 internal/character/fame.go 同一格（181 字节记录 +170 的调适阶段）。
			awakening = int32(w.Record[170])
		}
		items = append(items, catalog.PointItem{
			Template:     w.Template,
			Awakening:    awakening,
			PartSetIndex: partSets[w.Template],
		})
	}
	return items, slotOf, nil
}

// pointPartSetIndexes 取每件模板的套装号；**没有装备目录时返回全 -1**。
//
// 为什么允许没有目录：誓约积分（`oathpointinfo.cos`）按模板索引，整套表与装备目录无关；
// 缺目录时它仍应算得出来。套装积分则相反 —— 没有套装号，`[part set index] = -1` 的行
// 一律命不中，`SetPoints` 的命中数会明显偏低。这条日志用来区分"角色确实没分"与
// "目录没装上"，避免把后者当成前者。
func pointPartSetIndexes(service *inventory.ItemService, templates []uint32) (map[uint32]int32, error) {
	out := make(map[uint32]int32, len(templates))
	if service == nil || service.Equipment == nil {
		for _, id := range templates {
			out[id] = -1
		}
		log.Printf("set points: equipment catalog unavailable; %d worn templates cannot resolve [part set index]", len(templates))
		return out, nil
	}
	return service.Equipment.PartSetIndexes(templates)
}

// oathPointItems 取当前穿戴里**参与誓约积分**的件：晶体 36..46 + 誓约核心 47。
//
// 调适档位与套装号来自 wornPointItems（同一份口径）。誓约表按**模板**索引，套装号只参与
// 「按套装号更精确的那一行」回退；晶体/誓约核心在源里没有 `[part set index]` ⇒ 取到 -1，
// 与改动前一致。
func oathPointItems(service *inventory.ItemService, state []byte) ([]catalog.PointItem, error) {
	all, slotOf, err := wornPointItems(service, state)
	if err != nil {
		return nil, err
	}
	items := make([]catalog.PointItem, 0, len(all))
	for _, it := range all {
		if slot := slotOf[it.Template]; slot >= 36 && slot <= 47 {
			items = append(items, it)
		}
	}
	return items, nil
}

// oathPointPackets 组 NOTI2634 的候选帧。算不出规则（未装载积分表）时返回 nil ——
// 宁可不发，也不发一对 0 把"未知"写成"该角色积分为 0"。
//
// 载荷顺序按实机定案：`u16 键 + u32 值A（OathPoint，落 实体+1872）+ u32 值B（SetPoint，
// 落 实体+1876，"套装积分"读它）`。SetPoint 与 OathPoint 各自独立聚合：SetPoint 取全部
// 穿戴装备（与名望同一批槽位），OathPoint 只取 36..47。
func (w *worldSession) oathPointPackets() []outboundPacket {
	if w == nil || w.items == nil || w.items.Points == nil || len(w.role.State) == 0 {
		return nil
	}
	setItems, _, err := wornPointItems(w.items, w.role.State)
	if err != nil {
		log.Printf("set points: cannot read worn items: %v", err)
		return nil
	}
	oathItems, err := oathPointItems(w.items, w.role.State)
	if err != nil {
		log.Printf("oath points: cannot read worn items: %v", err)
		return nil
	}
	oath, hits := w.items.Points.OathPoints(oathItems)
	setPoints, setHits := w.items.Points.SetPoints(setItems)
	log.Printf("oath points: oath=%d (%d/%d worn oath items) set=%d (%d/%d worn items)",
		oath, hits, len(oathItems), setPoints, setHits, len(setItems))
	return []outboundPacket{{
		"oath_part_set_point", 0, protocol.PartSetPointOpcode,
		protocol.PartSetPoint(oathPartSetPointKey, oath, setPoints),
	}}
}
