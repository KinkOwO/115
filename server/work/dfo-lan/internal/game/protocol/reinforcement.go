package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD80：当前客户端 145AEB720 的发送顺序；20260925 实机向量：
// 00030c00c2c20506004100ffff0e000000c3f0c1fabda3b0cdc2b3c4b7bfcbffffffff00。
// 名称只是显示信息，不能据此决定目标装备、强化等级或费用。
type ReinforcementRequest struct {
	Mode, EquipmentSpace     byte
	EquipmentSlot            uint16
	EquipmentTemplate        uint32
	TicketSpace              byte
	TicketSlot, MaterialSlot uint16
	NPC, ProtectionSlot      uint16
	Multiple                 byte
}

func DecodeReinforcement(p []byte) (ReinforcementRequest, error) {
	var r ReinforcementRequest
	if len(p) < 22 {
		return r, fmt.Errorf("强化请求长度不足")
	}
	n := binary.LittleEndian.Uint32(p[13:17])
	if n > 512 || uint64(n)+22 > uint64(len(p)) {
		return r, fmt.Errorf("强化请求名称长度无效")
	}
	// CMD80 使用序号 10 的四字节对齐编码。正文长度不是四的倍数时，
	// 解密结果会保留末尾零填充；不能把填充误判为名称长度错误。
	// 同时保留原生发送器尚未补齐的直接向量，拒绝非零或多余尾部。
	end := int(n) + 22
	if len(p) != end && len(p) != (end+3)/4*4 {
		return r, fmt.Errorf("强化请求尾部长度无效")
	}
	if err := padding(p[end:], 4); err != nil {
		return r, err
	}
	r.Mode, r.EquipmentSpace = p[0], p[1]
	r.EquipmentSlot = binary.LittleEndian.Uint16(p[2:4])
	r.EquipmentTemplate = binary.LittleEndian.Uint32(p[4:8])
	r.TicketSpace = p[8]
	r.TicketSlot = binary.LittleEndian.Uint16(p[9:11])
	r.MaterialSlot = binary.LittleEndian.Uint16(p[11:13])
	tail := p[17+int(n):]
	r.NPC, r.ProtectionSlot = binary.LittleEndian.Uint16(tail), binary.LittleEndian.Uint16(tail[2:])
	r.Multiple = tail[4]
	return r, nil
}

// FixedReinforcementSupported ...
func FixedReinforcementSupported(level byte) bool { return level >= 1 && level <= 15 }

// ★★ CMD80 回包 [1] 的取值规则（改过两次，这里是最终版，别再改回去）：
//
// 客户端唯一处理 CMD80 回包的函数 sub_14529B2F0 把 [1] 读进 v152，结构是：
//
//	if (v152 == 10)      { v52 = _RTDynamicCast(item, &off_14DCB4FF0, &off_14DCB71E8); ... }
//	else if (v152 == 11) { v52 = _RTDynamicCast(item, &off_14DCB4FF0, &off_14DCB71E8); ... }
//	else                 { v52 = _RTDynamicCast(item, &off_14DCB4FF0, &off_14DCB64C8); ... }
//
// 进 10/11 分支之后还要过一道门禁：`if (v52 || v149 == 3)`。
// v149 是结果码，3 = 摧毁。所以：
//
//	result == 3（摧毁）→ v149==3 成立，无论 dynamic_cast 是否成功都能进分支 ✅
//	result != 3        → 只能靠 _RTDynamicCast 到 off_14DCB71E8 成功才进得去 ❌
//
// 这台单机服务端的装备对象转不到 off_14DCB71E8，于是非摧毁结果一旦写 10/11
// 就进不去分支，直接落到分支链之后的公共落点弹 dstr 1658「No items are available.」
// —— 这就是「强化和增幅都被修坏了」的根因（2026-09-28）。
//
// 因此最终规则是：
//
//	非摧毁（result 0/1/2）：[1] 原样回显请求里的 mode（0=强化 / 1=增幅）
//	                        → 走兜底 else 分支，转型目标是宽松得多的 off_14DCB64C8，
//	                          这是长期实机验证可用的老路径。
//	摧毁（result == 3）  ：[1] 必须写成 10（强化）/ 11（增幅）
//	                        → 只有这里才需要硬闯 10/11 分支，因为摧毁对话框和
//	                          36 字节尾部布局都长在分支里面；不写就永远等不到
//	                          结束信号，窗口刷不出来，游戏卡死（伴随 CMD217）。
//
// 10 = 强化（分支里 56 字节结果对象 sub_1412DCDE0，注册 NOTI 2478）
// 11 = 增幅（分支里 80 字节结果对象 sub_1410E8CD0，注册 NOTI 2581）
const (
	cmd80KindReinforce byte = 10
	cmd80KindAmplify   byte = 11
)

// cmd80UpgradeKind 决定回包 [1] 到底写什么。见上面常量块里的完整推导。
// 只有摧毁（result==3）才升级成 10/11，其余一律回显请求 mode。
func cmd80UpgradeKind(mode, result byte) byte {
	if result == 3 {
		if mode == 1 {
			return cmd80KindAmplify
		}
		return cmd80KindReinforce
	}
	return mode
}

// cmd80Tail 拼 CMD80 回包 [17] 之后的尾部。两个布局不同，别混用：
//
//	非摧毁（整包 35 字节）: [17..18]=槽位 0xffff  [19..22]=0  [23]=0  [24]=0  [25..34]=10 字节
//	摧毁  （整包 36 字节）: [17]=额外条目数       再接上面那 18 字节
//
// 摧毁分支（result==3）会先读一个 u8 当「额外条目数」，再按 (u16 槽, u32, u32)
// 循环读那么多条。如果把非摧毁布局的 0xffff 留在 [17]，客户端会当 255 条去读，
// 一个 35 字节的包要读 2500 字节 —— 直接读爆，这就是 CMD217 的来源。
func cmd80Tail(p []byte, destroyed bool) []byte {
	if destroyed {
		p = append(p, 0) // 额外条目数：没有
	}
	p = binary.LittleEndian.AppendUint16(p, 0xffff) // 槽位：无
	p = binary.LittleEndian.AppendUint32(p, 0)
	p = append(p, 0, 0)
	p = append(p, make([]byte, 10)...)
	return p
}

// CMD80 成功体按 14529B2F0 的全部 reader 构造，包括末尾材料与外观字段。
// result=0 表示强化成功，result=1 表示概率失败且等级不变；均消耗一张券。
func ReinforcementTicketReply(r ReinforcementRequest, remaining uint32, old, level, result byte) ([]byte, error) {
	if r.Mode != 0 || (r.EquipmentSpace != 0 && r.EquipmentSpace != 3) ||
		r.TicketSpace != 0 || result > 1 ||
		(result == 0 && !FixedReinforcementSupported(level)) || (result == 1 && old != level) {
		return nil, fmt.Errorf("强化券结果不符合客户端固定券规则")
	}
	// 固定券永远不会摧毁（result 只可能是 0/1），所以这里直接回显 mode。
	p := []byte{1, cmd80UpgradeKind(r.Mode, result)}
	p = binary.LittleEndian.AppendUint16(p, r.TicketSlot)
	p = binary.LittleEndian.AppendUint32(p, remaining)
	p = binary.LittleEndian.AppendUint16(p, 0xffff) // 没有保护券。
	p = append(p, 1, old, result, level, r.EquipmentSpace)
	p = binary.LittleEndian.AppendUint16(p, r.EquipmentSlot)
	p = cmd80Tail(p, false)
	// 与当前服务端 EquippedAppearance 的 WeaponA/B/WeaponTail 默认值一致。
	return p, nil
}

// CMD80 金币强化（材料 + 金币）的成功体。与固定券共用 14529B2F0 的 reader 布局，
// 槽位与剩余量换成被消耗的材料；差别只有两点：
//
//  1. 等级上限按客户端实机上限 15（超过会出现 ADD_HACKTYPE_CNT 与窗口锁死）；
//  2. 失败分支与固定券一样要求 old == level —— 实机把 8→7 的降级写进回包同样被判定异常，
//     真实掉级改由随后的装备行下发（客户端按行更新显示）。
func ReinforcementGoldReply(r ReinforcementRequest, remaining uint32, old, level, result byte) ([]byte, error) {
	if r.Mode != 0 || (r.EquipmentSpace != 0 && r.EquipmentSpace != 3) ||
		r.TicketSpace != 0 || result > 1 || level > GoldReinforcementMaxLevel || old > GoldReinforcementMaxLevel ||
		(result == 0 && level > GoldReinforcementResultCap) ||
		(result == 1 && old != level) {
		return nil, fmt.Errorf("金币强化结果不符合客户端规则")
	}
	// 金币强化同样不会摧毁（result > 1 已被上面拒绝），回显 mode 即可。
	p := []byte{1, cmd80UpgradeKind(r.Mode, result)}
	p = binary.LittleEndian.AppendUint16(p, r.TicketSlot)
	p = binary.LittleEndian.AppendUint32(p, remaining)
	p = binary.LittleEndian.AppendUint16(p, 0xffff)
	p = append(p, 1, old, result, level, r.EquipmentSpace)
	p = binary.LittleEndian.AppendUint16(p, r.EquipmentSlot)
	p = cmd80Tail(p, false)
	return p, nil
}

// AmplifyUpgradeReply 是 CMD80 **mode=1（增幅）** 的结果体。
//
// 增幅不是独立 opcode，而是 CMD80 的 mode=1（2026-09-28 实机抓包确认：44 字节明文，
// 与强化共用 ReinforcementRequest，唯一差别是 [0]=1）。客户端处理 CMD80 回包的
// handler 只有一个（sub_14529B2F0），所以增幅回包**沿用与强化完全相同的布局**，
// 只把 [1] 的升级种类写成 11；[11]/[13] 换成增幅的旧/新等级。
//
//	[0]=1 状态字节（分派器消费）
//	[1]=增幅种类：非摧毁回显 mode=1；result==3（摧毁）时才写成 11
//	    —— 见 cmd80UpgradeKind 的推导，无脑写 11 会让非摧毁结果弹 1658。
//	[2..3]=增幅材料槽 [4..7]=材料剩余 [8..9]=0xffff
//	[10]=1 [11]=旧增幅等级 [12]=结果(0成功/1失败不变/2降级/3摧毁) [13]=新增幅等级 [14]=装备空间
//	[15..16]=装备槽，之后接 cmd80Tail：非摧毁 35 字节，摧毁多一个条目数字节 → 36 字节
func AmplifyUpgradeReply(r ReinforcementRequest, remaining uint32, oldLevel, newLevel, result byte) ([]byte, error) {
	if r.Mode != 1 {
		return nil, fmt.Errorf("增幅回包的 mode 必须是 1，收到 %d", r.Mode)
	}
	// 结果码必须与等级变化自洽：客户端 handler（sub_14529B2F0）对 result==1 要求
	// old == new、对 result==2 要求 old > new；不一致会被判定异常并锁死增幅窗口
	// （强化那边 ReinforcementGoldReply 已有同样约束）。
	switch result {
	case 0:
		if int(newLevel) != int(oldLevel)+1 {
			return nil, fmt.Errorf("增幅成功回包的新等级必须是旧等级 +1")
		}
	case 1:
		if newLevel != oldLevel {
			return nil, fmt.Errorf("增幅失败（等级不变）回包要求新旧等级相同")
		}
	case 2:
		if int(oldLevel) <= int(newLevel) {
			return nil, fmt.Errorf("增幅失败（降级）回包要求旧等级 > 新等级")
		}
	case 3:
		// 摧毁：等级已无意义，不做约束。
	default:
		return nil, fmt.Errorf("未知的增幅结果码 %d", result)
	}
	p := []byte{1, cmd80UpgradeKind(r.Mode, result)}
	p = binary.LittleEndian.AppendUint16(p, r.TicketSlot)
	p = binary.LittleEndian.AppendUint32(p, remaining)
	p = binary.LittleEndian.AppendUint16(p, 0xffff)
	p = append(p, 1, oldLevel, result, newLevel, r.EquipmentSpace)
	p = binary.LittleEndian.AppendUint16(p, r.EquipmentSlot)
	// result==3（摧毁）必须走 36 字节布局：客户端先读 [17] 当条目数再循环。
	p = cmd80Tail(p, result == 3)
	return p, nil
}

// AmplifyTicketReply 是 CMD80 mode=1 下使用**增幅券**（固定等级券）的结果体。
//
// 与 AmplifyUpgradeReply（材料增幅，每级 +1）的关键差别：增幅券是**跳级**券。
// PVF 段 [equipment amplify reinforcement ticket] 的四个值是
// [目标等级, 成功率, 'fixed', -1]，例如 +10 券会把装备从当前等级**直接**拉到 +10，
// 而不是 +1。所以成功回包不能强制 newLevel == oldLevel+1，否则 +0 用 +10 券会被
// 服务端自己判定为「结果不自洽」而整单拒绝 —— 这就是「增幅券不能直接附加到装备上」。
//
// 这与普通强化券的 ReinforcementTicketReply 完全对称：固定券本来就允许跳级
// （+0 → +10 已实机验证可用），增幅券共用同一个 CMD80 handler（sub_14529B2F0）
// 与同样的 35 字节布局，唯一的差别是 mode=1。
//
// 布局与 AmplifyUpgradeReply 一致：
//
//	[0]=1 [1]=回显 mode（非摧毁） [2..3]=券槽 [4..7]=券剩余 [8..9]=0xffff
//	[10]=1 [11]=旧等级 [12]=结果(0 成功/1 失败不变) [13]=新等级 [14]=装备空间 [15..16]=装备槽
//
// 增幅券失败只消耗券、等级不变（与官方固定券一致：不降级、不摧毁），所以尾部
// 恒为非摧毁的 35 字节布局。
func AmplifyTicketReply(r ReinforcementRequest, remaining uint32, old, level, result byte) ([]byte, error) {
	if r.Mode != 1 {
		return nil, fmt.Errorf("增幅券回包的 mode 必须是 1，收到 %d", r.Mode)
	}
	if result > 1 {
		return nil, fmt.Errorf("增幅券不降级也不摧毁，结果码只能是 0/1，收到 %d", result)
	}
	if result == 0 {
		// 固定券的目标等级上限与强化券同为一个等级字节能表达的 1..15。
		if !FixedReinforcementSupported(level) {
			return nil, fmt.Errorf("增幅券目标等级 %d 超出客户端支持范围（1..15）", level)
		}
		// 跳级券允许 old+1 之外的跨度，但必须真的往上走。
		if int(level) <= int(old) {
			return nil, fmt.Errorf("增幅券成功回包的新等级必须高于旧等级（old=%d new=%d）", old, level)
		}
	}
	if result == 1 && old != level {
		return nil, fmt.Errorf("增幅券失败（等级不变）回包要求新旧等级相同")
	}
	p := []byte{1, cmd80UpgradeKind(r.Mode, result)}
	p = binary.LittleEndian.AppendUint16(p, r.TicketSlot)
	p = binary.LittleEndian.AppendUint32(p, remaining)
	p = binary.LittleEndian.AppendUint16(p, 0xffff)
	p = append(p, 1, old, result, level, r.EquipmentSpace)
	p = binary.LittleEndian.AppendUint16(p, r.EquipmentSlot)
	p = cmd80Tail(p, false)
	return p, nil
}

// GoldReinforcementMaxLevel 是装备实例行偏移 10 低五位能表达的最高强化等级。
const GoldReinforcementMaxLevel = 31

// GoldReinforcementResultCap 是客户端 CMD80 reader 实机接受的结果等级上限（成功分支）。
const GoldReinforcementResultCap = 15

// AmplifyUpgradeResultCap 是客户端 CMD80 **成功回包**实机接受的新增幅等级上限。
//
// 服主 2026-10-07 实机：+1..+15 增幅成功都正常，只有 +16 成功会触发
// ADD_HACKTYPE_CNT（客户端字符串 60225 "Don't even think about making any
// unauthorized upgrade attempts!"）并锁死增幅窗口。
//
// ⚠️ 这与「装备行能渲染 +31」**并不冲突**，是两道互相独立的检查：
//   - 装备行 reader：认 0..31（client_trace 实证：+16 / +31 都正常渲染）
//   - CMD80 成功回包 [13]：只认到 15（服主实机：只有 16 卡）
// 另外 PVF 的 etc/amplifyupgrade.etc 写 [max upgrade level by rarity]=50、
// 90US 官方源码写 upgradeMaximumCurrentLevelBeforeReject=30（→+31），
// 都指的是「装备能拥有的等级」，不是这条回包校验。
//
// 因此超过上限时：**落库仍写真实等级**，回包改用 result=1 + new=old 过检，
// 真实等级靠随后的装备行刷新下发 —— 与仓库处理「降级 8→7 写进回包会
// ADD_HACKTYPE_CNT」的既有解法同一套路。
//
// 上游默认取 15（方案 B 兜底）：无需客户端补丁即可安全运行，+16 播失败
// 动画但实际成功、装备行刷新真实等级。若已用 tools/patch_client_amplify.py
// 给客户端 DFO.exe 打过补丁（上限 10→30，与官方 31 上限对齐），可改为 31
// 让成功回包直接带真实等级、播放成功动画。
const AmplifyUpgradeResultCap = 15

// ClientAmplifyPatch 记录 115 客户端 DFO.exe 的本地补丁位置，便于回滚与复查。
//
// sub_14529B2F0（CMD80 回包 handler）内的等级判据：
//   0x14529bb33  lea eax, [rdx-1]        ; rdx = 回包 [13] 新等级
//   0x14529bb36  cmp eax, 0xc   -> ja    ; 新等级 > 15 时走 fallback 分支
//   ...
//   0x14529bb71  lea eax, [rdx-1]
//   0x14529bb74  cmp al, 0xa    -> ja    ; ★ 新等级-1 > 10 就上报 ADD_HACKTYPE_CNT
// ⇒ 原逻辑：新等级 16 时 16-3=13>12 进 fallback，再 16-1=15>10 → hack。
//
// 补丁：文件偏移 0x529bb75 处 0x0a(10) → 0x1e(30)，
//       使上限变成「新等级-1 <= 30」即**新等级 <= 31**，与官方 31 上限对齐。
// 原值 0x0a，备份见 .workbuddy/backup-client-20261007/DFO.exe。
const ClientAmplifyPatchOffset = 0x529bb75
const ClientAmplifyPatchOrig = 0x0a
const ClientAmplifyPatchValue = 0x1e
