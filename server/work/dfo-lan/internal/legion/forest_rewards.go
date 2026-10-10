package legion

import (
	"encoding/binary"
	"fmt"
)

// 苏醒之森通关奖励表。真源分两类：
//
//   - Normal（内容 104）：2026-10-02 官服抓包（N2252 四条 + N2253 五条），
//     以及用户 2026-10-06 给的方案表。
//   - Extreme（内容 105）：**2026-10-08 官服抓包**（official_20261008-220048_live
//     的 session_s13，Extreme 三关全清）。官服 N2252/N2253 是 zlib 压缩发送的，
//     解压后 N2252 = 7772B（七行数据）、N2253 = 2405B（一行数据）。
//
// 显示与发放同源：N2252/N2253 的翻牌展示和 Awarder 入库读同一张表。
//
//	普通：第 1/2 关 = N2253 三条；终局 = N2252 四条（材料 + 3 件随机
//	魔器/神器装备）+ N2253 五条。
//	困难（连战）：终局一段式结算 = N2252 **七行**（首两行留空）+ N2253 一条。
const (
	ForestRewardForgottenLight  uint32 = 10360622 // Forgotten Light
	ForestRewardFadedLight      uint32 = 10359558 // Faded Light
	ForestRewardHarmonious      uint32 = 10358304 // Harmonious Dimensional Energy
	ForestRewardConquerorToken  uint32 = 10359598 // Conqueror's Token - Forest of Awakening
	ForestRewardPromise         uint32 = 10358902 // Promise of the Harmonious One
	ForestRewardPromiseTradable uint32 = 10359691 // [Tradable] Promise of the Harmonious One
	ForestRewardSilverSprig     uint32 = 10359718 // Unwithering Silver Sprig

	// Extreme 官服终局翻牌里三种只出现在困难难度的材料（2026-10-08 抓包
	// stackable/10361001/10361705.stk、10362001/10362184.stk、
	// 10403001/10403248.stk）。PVF 里 [name] 段为空（名字在客户端本地化表），
	// 因此按模板号命名、不臆造中文名。
	ForestHardRewardSprig3    uint32 = 10361705 // rarity 2
	ForestHardRewardSprig4    uint32 = 10362184 // rarity 3
	ForestHardRewardSelectBox uint32 = 10403248 // rarity ?（N2252 行尾 +40 标志位 = 1）
	ForestHardRewardPromise   uint32 = 10360946 // rarity 3（Extreme N2253 唯一一行）

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

// ForestHardFinalBasic：Extreme 终局 N2252 —— 官服 2026-10-08 抓包解压后的
// 七行（全部落在 44B 记录区 @1600+44k，40B 步长的头两行留空）：
//
//	@1600 10360622 × 33
//	@1644 10359558 × 30
//	@1688 10358304 × 20
//	@1732 10361705 × 3
//	@1776 10362184 × 1
//	@1820 10403248 × 1（行尾 +40 标志位 = 1）
//	@1864 10360622 × 16
func ForestHardFinalBasic() []ForestRewardItem {
	return []ForestRewardItem{
		{ForestRewardForgottenLight, 33},
		{ForestRewardFadedLight, 30},
		{ForestRewardHarmonious, 20},
		{ForestHardRewardSprig3, 3},
		{ForestHardRewardSprig4, 1},
		{ForestHardRewardSelectBox, 1},
		{ForestRewardForgottenLight, 16},
	}
}

// ForestHardFinalAdditional：Extreme 终局 N2253 —— 官服原文一行
// （flag=01、模板 10360946 ×1、@9=00；Normal 的同类行 @9 是 03）。
func ForestHardFinalAdditional() []ForestRewardItem {
	return []ForestRewardItem{
		{ForestHardRewardPromise, 1},
	}
}

// forestHardRowFlag 返回 N2252 44B 行尾 +40 的标志位。官服 Extreme 七行里
// 只有 10403248（可选箱）那一行是 1，其余为 0。
func forestHardRowFlag(template uint32) uint32 {
	if template == ForestHardRewardSelectBox {
		return 1
	}
	return 0
}

// ForestBasicClearReward 构建翻牌首排 N2252（7772B 裸字节）：
//   - items 非空时按表写入行槽（Template=0 的槽填抽到的装备，value=1）；
//   - items 为空 = token-only（Normal 第 1/2 关，官服 stage0 原文形状）；
//   - @7760 = 该关 N31 头 2B token（**按模式取**：Extreme 三关 token 与
//     Normal 不同，用错模式令牌客户端翻牌面板不显示）。
//
// hard=true 时用官服 Extreme 布局：首两行（40B 步长）留空，七行全部写在
// 44B 记录区 @1600+44k。
func ForestBasicClearReward(stage int, items []ForestRewardItem, gear []uint32, hard bool) ([]byte, error) {
	if stage < 0 || stage > 2 {
		return nil, fmt.Errorf("forest reward stage %d out of range", stage)
	}
	token, err := ForestStageTokenFor(stage, hard)
	if err != nil {
		return nil, err
	}
	raw := make([]byte, 7772)
	gearAt := 0
	for i, item := range items {
		var off int
		switch {
		case hard:
			off = 1600 + 44*i
		case i < 2:
			off = 40 * i
		default:
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
		if off+8 > len(raw) {
			return nil, fmt.Errorf("forest flip row %d out of range", i)
		}
		binary.LittleEndian.PutUint32(raw[off:], template)
		binary.LittleEndian.PutUint32(raw[off+4:], value)
		if hard {
			binary.LittleEndian.PutUint32(raw[off+40:], forestHardRowFlag(template))
		}
	}
	raw[7760], raw[7761] = token[0], token[1]
	return raw, nil
}

// ForestAdditionalReward 构建第二排 N2253（2405B 裸字节，40B 记录步长
// {flag=01, item u32, count u8, 尾字节}）。尾字节官服 Normal 是 03，
// Extreme 那一行是 00 —— 按模式区分。
func ForestAdditionalReward(items []ForestRewardItem, hard bool) ([]byte, error) {
	raw := make([]byte, 2405)
	for i, item := range items {
		off := 40 * i
		if off+10 > len(raw) {
			return nil, fmt.Errorf("forest additional row %d out of range", i)
		}
		if item.Amount > 255 {
			return nil, fmt.Errorf("forest additional reward count %d exceeds u8", item.Amount)
		}
		raw[off] = 1
		binary.LittleEndian.PutUint32(raw[off+1:], item.Template)
		raw[off+5] = byte(item.Amount)
		if !hard {
			raw[off+9] = 3
		}
	}
	return raw, nil
}
