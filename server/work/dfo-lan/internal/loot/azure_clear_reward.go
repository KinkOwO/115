package loot

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
)

// AzureMainDungeonID 是蔚蓝号（Azure Main）的副本号。
const AzureMainDungeonID = 100004131

// AzureMainCardModel 是蔚蓝号专属的清关奖单模型名 —— 与通用卡池（reference90-free-gold-v1）
// 区分开，旧回执不会被误当成它。
const AzureMainCardModel = "azure-main-clear-v1"

// 蔚蓝号翻牌的「随机装备」那一段，参数与月湖同口径（对照已跑通的沉月湖，见
// moon_rewards.go 的 MoonEquipmentPolicy）：件数区间 1..4、权重 {1,3,3,2}（把期望压到
// 2.67 件，避免"经常只出一件"），等级取副本的 MinimumLevel，怪物档位取 boss 档 3
// （装备率最高）。**品质不在这里指定** —— 每件独立走源里的掉落/品质判定表
// （Tables.Rarity / Tables.Probability），所以品质分布是游戏自己给的，本仓不另编概率。
const (
	azureMainEquipmentMin  = 1
	azureMainEquipmentMax  = 4
	azureMainEquipmentRank = 3
)

var azureMainEquipmentWeights = []uint32{1, 3, 3, 2}

// AzureMainEquipmentPolicy 给出蔚蓝号翻牌的随机装备口径。
func AzureMainEquipmentPolicy(level byte) MoonEquipmentPolicy {
	return MoonEquipmentPolicy{
		Min:     azureMainEquipmentMin,
		Max:     azureMainEquipmentMax,
		Weights: azureMainEquipmentWeights,
		Level:   level,
		Rank:    azureMainEquipmentRank,
	}
}

// AzureMainClearRewards 读副本脚本自己的 [difficulty dropitem group list]，取出**固定产出**。
//
// 蔚蓝号（100004131）的脚本带 `[disable clear reward]`，所以通用卡池发的必然是错的
// （实机 2026-10-04 发的是随机低阶装备 Old Street Armor Shoes + 610 Gold）。它把清单写在
// contents/2025/azuremain/dungeon/azuremain.dgn：
//
//	[gold card use] 0
//	[disable clear reward]
//	[difficulty dropitem group list]
//	  [group info]                        ← 无 [contents] 标签的这一块 = 固定产出
//	    [item index] 10326880 10326884 10403423 10419054
//	    [reward multiple info] 2 10 9     ← 下标 2 ×10（面板「1000% efficiency of the base reward」）
//	    [reward multiple info] 3 4 3      ← 下标 3 ×4 （面板「400% efficiency of the base reward」）
//	  [/group info]
//	  [custom group info] [contents] `equipment guide` / `normal` / `matching`   ← 组队界面各页签的展示组
//	[/difficulty dropitem group list]
//
// `[reward multiple info]` = `<item index 下标> <倍率> [<第三列，语义未解，不读>]`；
// 倍率就是面板上那句「N00% efficiency of the base reward」的 N（10 → 1000%、4 → 400%）。
func AzureMainClearRewards(def catalog.DungeonDefinition) ([]Award, error) {
	blocks, err := catalog.ParseDungeonDropBlocks(def.Script.Cells)
	if err != nil {
		return nil, fmt.Errorf("蔚蓝号清关奖励：%w", err)
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("蔚蓝号清关奖励：副本脚本里没有 [difficulty dropitem group list]")
	}
	// 无 [contents] 标签的那一块才是清关固定产出；带标签的三块是组队界面各页签的展示组，
	// 三者的 [item index] 后两项互不相同，按标签挑会挑错。
	block := blocks[0]
	for _, b := range blocks {
		if b.Kind == "[group info]" {
			block = b
			break
		}
	}
	var index []uint32
	multiples := map[int]uint32{}
	for _, sec := range block.Sections {
		switch sec.Header {
		case "[item index]":
			for _, v := range sec.Values {
				if v <= 0 {
					return nil, fmt.Errorf("蔚蓝号清关奖励：非正的 item index %d", v)
				}
				index = append(index, uint32(v))
			}
		case "[reward multiple info]":
			// 第三列（本副本是 9 / 3）语义未解，只消费前两列。
			if len(sec.Values) < 2 {
				return nil, fmt.Errorf("蔚蓝号清关奖励：[reward multiple info] 只有 %d 列", len(sec.Values))
			}
			at, mul := int(sec.Values[0]), sec.Values[1]
			if at < 0 || at >= len(index) {
				return nil, fmt.Errorf("蔚蓝号清关奖励：倍率下标 %d 越界（共 %d 项）", at, len(index))
			}
			if mul <= 0 {
				return nil, fmt.Errorf("蔚蓝号清关奖励：倍率 %d 非正", mul)
			}
			if _, dup := multiples[at]; dup {
				return nil, fmt.Errorf("蔚蓝号清关奖励：下标 %d 的倍率重复声明", at)
			}
			multiples[at] = uint32(mul)
		}
	}
	if len(index) == 0 {
		return nil, fmt.Errorf("蔚蓝号清关奖励：没有 [item index]")
	}
	// 奖单 8 格：前 4 格留给固定产出，后 4 格留给随机装备（与月湖一致：固定票券 + 1..4 件装备）。
	if len(index) > 4 {
		return nil, fmt.Errorf("蔚蓝号清关奖励：固定产出 %d 项超过奖单前 4 格", len(index))
	}
	out := make([]Award, 0, len(index))
	for i, template := range index {
		amount := uint32(1)
		if mul, ok := multiples[i]; ok {
			amount = mul
		}
		out = append(out, Award{Template: template, Amount: amount})
	}
	return out, nil
}

// azureMainRewardCatalog 把脚本里那份「产物目录」读出来**只作来源校验**。
//
// `[item index]` 的 4 个模板**不是可发放的物品** —— 客户端把它们渲染成目录标记
// （"Expectable Rewards" / "Azure Main X 10" / "Azure Main X 4"），它们描述的是
// 这一趟可以从哪些东西里抽，本身不该进奖单。实机 2026-10-04：把它们放进奖单后，
// 翻牌面板上多出三行这种标记（业主截图）。所以这里只用来确认「脚本还是我们认得的那一份」，
// 真正的产物由装备掷骰从源里的池子抽（见 PlanAzureMainCards）。
func azureMainRewardCatalog(def catalog.DungeonDefinition) error {
	if _, err := AzureMainClearRewards(def); err != nil {
		return err
	}
	return nil
}

// PlanAzureMainCards 组蔚蓝号的翻牌奖单：**只放抽出来的装备**。
//
// 与通用 PlanCards 的差别：Gold 恒 0（脚本声明 `[gold card use] 0`，通用路径的
// `Gold != 0` 自检会把它当来源冲突拒掉）；装备那一段复用月湖同一套掷骰
// （rollMoonEquipment：稳定种子 + 逐件走源表），只是奖单载体换成翻牌用的 CardPlan。
//
// 奖单里**不放**脚本 `[item index]` 那几项 —— 它们是产物目录的目录项，不是产物本身。
func (s *Service) PlanAzureMainCards(role Role, d *dungeon.Session, equipment MoonEquipmentPolicy) (CardPlan, error) {
	var p CardPlan
	if d == nil || !d.Completed() {
		return p, fmt.Errorf("蔚蓝号奖单：副本尚未通关")
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return p, fmt.Errorf("蔚蓝号奖单：存档身份与当前源不一致")
	}
	if err := azureMainRewardCatalog(d.Definition); err != nil {
		return CardPlan{}, err
	}
	p = CardPlan{Run: d.RunID, Source: s.Catalog.Source.SaveIdentity(), Model: AzureMainCardModel}
	if p.Gold != 0 {
		return CardPlan{}, fmt.Errorf("蔚蓝号脚本声明 [gold card use] 0，奖单不该带金币")
	}
	// 装备段：种子由「装备口径 + 本局 RunID + 账号 + 角色」派生，
	// 同一局重放结果一致，不同局之间随机。
	raw, err := json.Marshal(equipment)
	if err != nil {
		return CardPlan{}, err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	rolled, err := s.rollMoonEquipment(digest, d, role, equipment)
	if err != nil {
		return CardPlan{}, fmt.Errorf("蔚蓝号奖单：随机装备 %w", err)
	}
	for i, g := range rolled {
		if i >= len(p.Items) {
			break
		}
		p.Items[i] = Award{Template: g.Template, Amount: g.Count}
	}
	return p, nil
}
