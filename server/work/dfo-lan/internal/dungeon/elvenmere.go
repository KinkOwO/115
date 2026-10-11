package dungeon

// ElvenmereFloorWeeklyReward 返回指定层数（1..100）的周常奖励道具ID和数量。
// 对应 Contents/2022/Elvenmere/Etc/Elvenmere.cos [weekly reward]：
// 1~60 层产出 10336354（叶之金币/Seed Coin）
// 61~100 层产出 10332382（金叶之金币）
func ElvenmereFloorWeeklyReward(floor byte) (template uint32, count uint32) {
	if floor < 1 || floor > 100 {
		return 0, 0
	}
	switch {
	case floor >= 1 && floor <= 34:
		return 10336354, 5
	case floor == 35:
		return 10336354, 30
	case floor >= 36 && floor <= 59:
		return 10336354, 7
	case floor == 60:
		return 10336354, 40
	case floor >= 61 && floor <= 84:
		return 10332382, 15
	case floor == 85:
		return 10332382, 100
	case floor >= 86 && floor <= 99:
		return 10332382, 20
	case floor == 100:
		return 10332382, 150
	}
	return 0, 0
}

// ElvenmereFloorSeasonReward 返回指定层数（逢5层）的赛季首通奖励道具ID和数量。
// 对应 Contents/2022/Elvenmere/Etc/Elvenmere.cos [season reward]：
func ElvenmereFloorSeasonReward(floor byte) (template uint32, count uint32) {
	switch floor {
	case 5:
		return 10333027, 1
	case 10:
		return 10333726, 1
	case 15:
		return 10332755, 1
	case 20:
		return 10332756, 1
	case 25:
		return 10333719, 5
	case 30:
		return 10333033, 1
	case 35:
		return 10333023, 1
	case 40:
		return 10332761, 1
	case 45:
		return 10332757, 1
	case 50:
		return 10333035, 1
	case 55:
		return 10333036, 1
	case 60:
		return 10332758, 1
	case 65:
		return 10332764, 1
	case 70:
		return 10332759, 1
	case 75:
		return 10333037, 1
	case 80:
		return 10333038, 1
	case 85:
		return 10333039, 1
	case 90:
		return 10333025, 1
	case 95:
		return 10333026, 1
	case 100:
		return 10333022, 1
	}
	return 0, 0
}

// ElvenmereFloorClearExp 返回指定层数（1..100）的通关经验奖励。
// 艾尔芬米尔是快速练级爬塔地下城，每层提供随层数递增的丰厚升级经验。
func ElvenmereFloorClearExp(floor byte) uint64 {
	if floor < 1 || floor > 100 {
		return 0
	}
	return uint64(floor)*30000 + 60000
}
