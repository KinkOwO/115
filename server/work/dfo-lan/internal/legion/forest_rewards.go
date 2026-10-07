package legion

import (
	"encoding/binary"
	"fmt"
)

// 苏醒之森通关奖励表（2026-10-06 用户方案，模板号 = PVF 编号）。
//
// 显示与发放同源：N2252/N2253 的翻牌展示和 Awarder 入库读同一张表。
//
//	普通：第 1/2 关 = N2253 三条；终局 = N2252 四条（材料 + 3 件随机
//	魔器/神器装备）+ N2253 五条。
//	困难（连战）：终局一段式结算 = N2252 十一条 + N2253 一条。
//	官服无 Extreme 抓包，翻牌布局沿用家族形状（N2252 40B 行 0-1 + 44B
//	行 2+、N2253 40B 记录步长 flag=01），token @7760 照旧。
const (
	ForestRewardForgottenLight  uint32 = 10360622 // Forgotten Light
	ForestRewardFadedLight      uint32 = 10359558 // Faded Light
	ForestRewardHarmonious      uint32 = 10358304 // Harmonious Dimensional Energy
	ForestRewardConquerorToken  uint32 = 10359598 // Conqueror's Token - Forest of Awakening
	ForestRewardPromise         uint32 = 10358902 // Promise of the Harmonious One
	ForestRewardPromiseTradable uint32 = 10359691 // [Tradable] Promise of the Harmonious One
	ForestRewardSilverSprig     uint32 = 10359718 // Unwithering Silver Sprig

	ForestGearSlots = 3 // 普通终局首排的随机装备位数
)

// ForestRewardItem 是一条翻牌奖励（模板 + 数量；Template=0 表示随机装备槽）。
type ForestRewardItem struct {
	Template uint32
	Amount   uint32
}

// ForestNormalStageRewards：Normal 第 1/2 关（N2253 三条）。
func ForestNormalStageRewards() []ForestRewardItem {
	return []ForestRewardItem{
		{ForestRewardForgottenLight, 11},
		{ForestRewardFadedLight, 10},
		{ForestRewardHarmonious, 1},
	}
}

// ForestNormalFinalBasic：Normal 终局 N2252（上面 4 个：材料 + 3 装备槽）。
func ForestNormalFinalBasic() []ForestRewardItem {
	return []ForestRewardItem{
		{ForestRewardForgottenLight, 14},
		{0, 1}, {0, 1}, {0, 1},
	}
}

// ForestNormalFinalAdditional：Normal 终局 N2253（下面 5 个）。
func ForestNormalFinalAdditional() []ForestRewardItem {
	return []ForestRewardItem{
		{ForestRewardForgottenLight, 11},
		{ForestRewardFadedLight, 3},
		{ForestRewardFadedLight, 3},
		{ForestRewardFadedLight, 3},
		{ForestRewardFadedLight, 3},
	}
}

// ForestHardFinalBasic：Hard 终局 N2252（上面 11 个，全材料）。
func ForestHardFinalBasic() []ForestRewardItem {
	return []ForestRewardItem{
		{ForestRewardForgottenLight, 8},
		{ForestRewardForgottenLight, 33},
		{ForestRewardFadedLight, 3},
		{ForestRewardFadedLight, 3},
		{ForestRewardFadedLight, 30},
		{ForestRewardConquerorToken, 1},
		{ForestRewardPromise, 1},
		{ForestRewardPromiseTradable, 1},
		{ForestRewardSilverSprig, 25},
		{ForestRewardForgottenLight, 3},
		{ForestRewardForgottenLight, 3},
	}
}

// ForestHardFinalAdditional：Hard 终局 N2253（下面 1 个）。
func ForestHardFinalAdditional() []ForestRewardItem {
	return []ForestRewardItem{
		{ForestRewardForgottenLight, 30},
	}
}

// ForestBasicClearReward 构建翻牌首排 N2252（7772B 裸字节）：
//   - items 非空时按表写入行槽（Template=0 的槽填抽到的装备，value=1）；
//   - items 为空 = token-only（Normal 第 1/2 关，官服 stage0 原文形状）；
//   - @7760 = 该关 N31 头 2B token。
func ForestBasicClearReward(stage int, items []ForestRewardItem, gear []uint32) ([]byte, error) {
	if stage < 0 || stage > 2 {
		return nil, fmt.Errorf("forest reward stage %d out of range", stage)
	}
	token, err := ForestStageToken(stage)
	if err != nil {
		return nil, err
	}
	raw := make([]byte, 7772)
	gearAt := 0
	for i, item := range items {
		var off int
		if i < 2 {
			off = 40 * i
		} else {
			off = 1600 + 44*(i-2)
		}
		template, value := item.Template, item.Amount
		if template == 0 { // 随机装备槽
			if gearAt >= len(gear) {
				return nil, fmt.Errorf("forest flip gear roll exhausted at row %d", i)
			}
			template, value = gear[gearAt], 1
			gearAt++
		}
		binary.LittleEndian.PutUint32(raw[off:], template)
		binary.LittleEndian.PutUint32(raw[off+4:], value)
	}
	raw[7760], raw[7761] = token[0], token[1]
	return raw, nil
}

// ForestAdditionalReward 构建第二排 N2253（2405B 裸字节，40B 记录步长
// {flag=01, item u32, count u8, const 03}，全部置展示位）。
func ForestAdditionalReward(items []ForestRewardItem) ([]byte, error) {
	raw := make([]byte, 2405)
	for i, item := range items {
		off := 40 * i
		if item.Amount > 255 {
			return nil, fmt.Errorf("forest additional reward count %d exceeds u8", item.Amount)
		}
		raw[off] = 1
		binary.LittleEndian.PutUint32(raw[off+1:], item.Template)
		raw[off+5] = byte(item.Amount)
		raw[off+9] = 3
	}
	return raw, nil
}
