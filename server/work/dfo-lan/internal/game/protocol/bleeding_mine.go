package protocol

import (
	"encoding/binary"
	"fmt"
)

type BleedingMineMember struct {
	Server byte
	Slot   int32
}

type BleedingMineTeam struct {
	Group   uint32
	Members [4]BleedingMineMember
}

// NOTI1474：1452AE370读取两个u32；频道106直接以毫秒调用场景虚表+88，
// 145B3E4E0启动1B98计时器。矿区不使用其它玩法的第二计时参数。
func BleedingMineTimer(remainingMilliseconds uint32) []byte {
	p := make([]byte, 8)
	binary.LittleEndian.PutUint32(p, remainingMilliseconds)
	return p
}

// NOTI2706：14073DDE0固定读取26字节；首字节0进入挑战失败分支，
// 关闭战斗/复活弹窗并停止矿区音效。失败分支不读取阶段奖励字段。
func BleedingMineFailed() []byte {
	return make([]byte, 26)
}

// 14073DDE0：结果、剩余毫秒、阶段、保留字段、是否结束、结束分支标记、
// 本阶段耗时及累计耗时；四阶段耗时写入客户端290数组。
func BleedingMineStageResult(stage, remaining, elapsed, total uint32, finished bool) []byte {
	p := make([]byte, 26)
	p[0] = 1
	binary.LittleEndian.PutUint32(p[1:], remaining)
	binary.LittleEndian.PutUint32(p[5:], stage)
	if finished {
		p[13] = 1
	}
	binary.LittleEndian.PutUint32(p[18:], elapsed)
	binary.LittleEndian.PutUint32(p[22:], total)
	return p
}

// CMD2320、2326原生发送21字节：13字节栈前缀和两个u32。
func DecodeBleedingMineStage(p []byte) (uint32, uint32, error) {
	if len(p) != 21 && len(p) != 24 {
		return 0, 0, fmt.Errorf("矿区阶段请求长度无效")
	}
	group := binary.LittleEndian.Uint32(p[13:])
	if group >= 3 {
		return 0, 0, fmt.Errorf("矿区难度编号无效")
	}
	return group, binary.LittleEndian.Uint32(p[17:]), nil
}

// 141416CF0 / 141416F70：组号后跟一个结束选项，原生业务长度18字节。
func DecodeBleedingMineEnd(p []byte) (uint32, bool, error) {
	if len(p) != 18 && len(p) != 24 {
		return 0, false, fmt.Errorf("矿区结束请求长度无效")
	}
	group := binary.LittleEndian.Uint32(p[13:])
	if group >= 3 || p[17] > 1 {
		return 0, false, fmt.Errorf("矿区结束选项无效")
	}
	return group, p[17] != 0, nil
}

// 1414343E0：13字节栈前缀、u32数量、八个i32奖励池槽号。
func DecodeBleedingMineRewardSelection(p []byte) ([]uint32, error) {
	if len(p) != 49 && len(p) != 56 {
		return nil, fmt.Errorf("矿区奖励选择长度无效")
	}
	n := binary.LittleEndian.Uint32(p[13:])
	if n > 8 {
		return nil, fmt.Errorf("矿区最多选择八份奖励")
	}
	var slots []uint32
	seen := map[uint32]bool{}
	for i := uint32(0); i < n; i++ {
		slot := binary.LittleEndian.Uint32(p[17+4*i:])
		if slot >= 100 || seen[slot] {
			return nil, fmt.Errorf("矿区奖励槽号无效或重复")
		}
		seen[slot] = true
		slots = append(slots, slot)
	}
	return slots, nil
}

// 14143800D：13字节栈前缀后写入两个奖励袋槽号。
func DecodeBleedingMineCompose(p []byte) ([2]uint32, error) {
	var slots [2]uint32
	if len(p) != 21 && len(p) != 24 {
		return slots, fmt.Errorf("矿区合成请求长度无效")
	}
	slots[0], slots[1] = binary.LittleEndian.Uint32(p[13:]), binary.LittleEndian.Uint32(p[17:])
	if slots[0] >= 100 || slots[1] >= 100 || slots[0] == slots[1] {
		return slots, fmt.Errorf("矿区合成格位无效")
	}
	return slots, nil
}

// 14073DBF0固定读取532字节；14143B590将前八个物品ID投影为已选奖励，
// 后100条为物品ID和状态，数组位置就是CMD2322使用的稳定槽号。
func BleedingMineRewardInfo(selected [8]uint32, candidates []uint32) ([]byte, error) {
	if len(candidates) > 100 {
		return nil, fmt.Errorf("矿区奖励池超过100格")
	}
	p := make([]byte, 532)
	for i, id := range selected {
		binary.LittleEndian.PutUint32(p[i*4:], id)
	}
	for i, id := range candidates {
		binary.LittleEndian.PutUint32(p[32+i*5:], id)
	}
	return p, nil
}

// 14143479E的放弃请求：13字节原生栈前缀，随后u32组号；加密补齐至24字节。
func DecodeBleedingMineGiveUp(p []byte) (uint32, error) {
	if len(p) != 17 && len(p) != 24 {
		return 0, fmt.Errorf("赤红铁矿放弃请求长度无效")
	}
	group := binary.LittleEndian.Uint32(p[13:17])
	if group >= 3 {
		return 0, fmt.Errorf("赤红铁矿放弃请求组号无效")
	}
	return group, nil
}

// 141436BD0 的 CMD2317 写入65字节；实机解密后按8字节补齐为72字节。
// 前13字节和成员内的三个对齐字节来自未初始化的原生栈，不作为业务数据。
func DecodeBleedingMineTeam(p []byte) (BleedingMineTeam, error) {
	var team BleedingMineTeam
	if len(p) != 65 && len(p) != 72 {
		return team, fmt.Errorf("赤红铁矿编队请求长度无效：%d", len(p))
	}
	team.Group = binary.LittleEndian.Uint32(p[13:])
	if team.Group >= 3 {
		return team, fmt.Errorf("赤红铁矿编队编号无效")
	}
	for i := range team.Members {
		r := p[17+i*12:]
		member := BleedingMineMember{Server: r[0], Slot: int32(binary.LittleEndian.Uint32(r[8:]))}
		if binary.LittleEndian.Uint32(r[4:]) != 0 || member.Slot < -1 || (member.Slot == -1) != (member.Server == 0) {
			return team, fmt.Errorf("赤红铁矿编队成员%d无效", i+1)
		}
		team.Members[i] = member
	}
	return team, nil
}

func bleedingMineTeamRecord(team BleedingMineTeam) []byte {
	p := make([]byte, 60)
	binary.LittleEndian.PutUint32(p, team.Group)
	for i, member := range team.Members {
		offset := 12 + i*12
		p[offset] = member.Server
		binary.LittleEndian.PutUint32(p[offset+8:], uint32(member.Slot))
	}
	return p
}

// 1444FB350 固定读取64字节：组号和一条60字节编队记录。
// 只有成员身份参与名单更新，其余状态保持未开战初值。
func BleedingMineTeamInfo(team BleedingMineTeam) []byte {
	p := make([]byte, 64)
	binary.LittleEndian.PutUint32(p, team.Group)
	copy(p[4:], bleedingMineTeamRecord(team))
	return p
}

// 1444FB900 固定读取194字节。184为编队开关，193保留原生初值。
func BleedingMineProfile(teams [3]BleedingMineTeam) []byte {
	p := make([]byte, 194)
	for i, team := range teams {
		team.Group = uint32(i)
		copy(p[i*60:], bleedingMineTeamRecord(team))
	}
	p[184], p[193] = 1, 1
	return p
}
