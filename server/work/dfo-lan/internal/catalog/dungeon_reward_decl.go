package catalog

import (
	"dfolan/internal/catalog/pvf"
)

// 本文件解析副本脚本自己声明的**通关翻牌内容**（[difficulty dropitem group list] 里
// 每个难度块的 [custom group info]）。
//
// 源里的形状（2026-10-09 从 100004137 / 100004136 / 100004131 直读，逐条有出处）：
//
//	[difficulty dropitem group list]
//	  [custom group info]
//	    [contents]           equipment guide        ← 产物列表页的名字（type-6 字符串）
//	    [item index]         10326880 10326884 10403422 10419051
//	    [normal group index] 2 21251 3 1 21291      ← 「个数+组号」交替，解码归 internal/loot
//	    [fame info]          0 0
//	    [special setinfo reward]  <2::SetEquipmentReward> 320 10326880 1 21279
//	    …
//	    [special custom reward info] <2::MonsterCard_ExpectationReward_0> 320 10326884 1 21292
//	    [reward multiple info] 2 7 6                 ← 类别 装备，普通 7 倍 / 匹配 6 倍
//	    [reward multiple info] 3 3 2                 ← 类别 誓约·星蕴石，普通 3 倍 / 匹配 2 倍
//	  [/custom group info]
//	  [custom group info] … 下一个难度块 …
//
// 为什么要把这些**原样**读进目录：
//
//   - `[reward multiple info]` 就是玩家看到的「产出效率」——普通难度 700%（装备）/ 300%
//     （誓约·星蕴石）、匹配难度 600% / 200%，与游戏内 tooltip 逐字吻合；
//   - `[special setinfo reward]` / `[special custom reward info]` 是**基础产物**的声明，
//     每条带自己的率（实测 320 / 138）与掉落组号，组号指向
//     `etc/dungeondroptablebygroup.etc`（本仓已能读）；
//   - `[contents]` + `[item index]` 是客户端那两个「产物列表」图标点开后的内容页。
//
// ⚠️ **形状不符一律保持空**：不猜、不补默认值，别的副本的行为一个字节都不变。
// 率的量纲（千分比 / 万分比 / 组间权重）**尚未定案** —— 目录只记录原值，
// 判定归上层；`Rate`/`Normal`/`Matching` 都是源里写的数，不是本仓换算过的。

// DungeonRewardMultiple 是源 [reward multiple info] 的一条声明。
//
// 源里是「奖励类别 + 各难度倍数」：实测 `2 7 6` 与 `3 3 2`（类别 2 = 装备、
// 类别 3 = 誓约·星蕴石）。第三格在某些块里缺省（单难度块只给一个倍数），此时
// Matching 保持 0 —— 0 表示「源里没写」，不是「0 倍」。
type DungeonRewardMultiple struct {
	Kind     uint32 `json:"kind"`
	Normal   uint32 `json:"normal"`
	Matching uint32 `json:"matching,omitempty"`
}

// DungeonSpecialReward 是源 [special setinfo reward] / [special custom reward info] 的一条声明。
//
// 实测（100004137）：
//
//	<2::SetEquipmentReward>         320 10326880 1 21279
//	<2::SetOathPrimerReward>        320 10326880 1 21468
//	<2::RareEquipmentReward>        320 10326880 1 21470
//	<2::WeaponEquipmentReward>      138 10326880 1 21310
//	<2::MonsterCard_ExpectationReward_0> 320 10326884 1 21292   ← 来自 [special custom reward info]
//
// 各格的判读：Name = list/n_string.lst 的引用名（`<表::键>` 里的键，本仓只留键）；
// Rate = 第二个数；List = 第三个数（**归属哪张「产物列表」**，实测恒为
// 10326880 / 10326884 这两件 "Expectable Rewards"）；Count = 第四个数；
// Group = 第五个数（`etc/dungeondroptablebygroup.etc` 的组号）。
type DungeonSpecialReward struct {
	Name   string `json:"name"`
	Rate   uint32 `json:"rate"`
	List   uint32 `json:"list,omitempty"`
	Count  uint32 `json:"count,omitempty"`
	Group  uint32 `json:"group,omitempty"`
	Custom bool   `json:"custom,omitempty"` // 来自 [special custom reward info]（而非 [special setinfo reward]）
}

// DungeonRewardBlock 是一个难度块的翻牌声明。
//
// Contents 为空表示源里那块没有 [contents]（老副本的 [group info] 就没有），
// 此时其它字段仍然照读 —— 不因为缺一个页名就整块丢弃。
type DungeonRewardBlock struct {
	Contents string                  `json:"contents,omitempty"`
	Item     []uint32                `json:"item_index,omitempty"`
	Groups   []int32                 `json:"normal_group_index,omitempty"`
	Multiple []DungeonRewardMultiple `json:"reward_multiple,omitempty"`
	Special  []DungeonSpecialReward  `json:"special_reward,omitempty"`
}

const (
	rewardGroupListOpen  = "[difficulty dropitem group list]"
	rewardGroupListClose = "[/difficulty dropitem group list]"
	rewardBlockOpen      = "[group info]"
	rewardBlockOpenAlt   = "[custom group info]"
	rewardBlockClose     = "[/group info]"
	rewardBlockCloseAlt  = "[/custom group info]"
	rewardContentsTag    = "[contents]"
	rewardItemIndexTag   = "[item index]"
	rewardGroupTag       = "[normal group index]"
	rewardMultipleTag    = "[reward multiple info]"
	rewardSpecialTag     = "[special setinfo reward]"
	rewardSpecialCustom  = "[special custom reward info]"
)

// rewardDeclarations 读 [difficulty dropitem group list] 的每个难度块。
//
// 块顺序**就是难度顺序**（与 internal/loot 的 DungeonGroupIndices 同一约定）。
// 只读数值/引用类型；遇到没见过的标签一律跳过，不做任何推断。
func rewardDeclarations(cells []pvf.Token) []DungeonRewardBlock {
	start := -1
	for i, c := range cells {
		if c.Type == 3 && c.Text == rewardGroupListOpen {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}
	var out []DungeonRewardBlock
	cur := -1
	for i := start; i < len(cells); {
		c := cells[i]
		if c.Type != 3 {
			i++
			continue
		}
		switch c.Text {
		case rewardGroupListClose:
			return out
		case rewardBlockOpen, rewardBlockOpenAlt:
			out = append(out, DungeonRewardBlock{})
			cur = len(out) - 1
			i++
		case rewardBlockClose, rewardBlockCloseAlt:
			cur = -1
			i++
		case rewardContentsTag:
			i++
			if cur >= 0 && i < len(cells) && cells[i].Type == 6 {
				out[cur].Contents = cells[i].Text
				i++
			}
		case rewardItemIndexTag:
			i++
			var vals []uint32
			for i < len(cells) && cells[i].Type == 0 {
				if cells[i].Value > 0 {
					vals = append(vals, uint32(cells[i].Value))
				}
				i++
			}
			if cur >= 0 {
				out[cur].Item = vals
			}
		case rewardGroupTag:
			i++
			var raw []int32
			for i < len(cells) && cells[i].Type == 0 {
				raw = append(raw, cells[i].Value)
				i++
			}
			if cur >= 0 {
				// 原样保存：「个数+组号」交替的解码归 internal/loot（它已有
				// ExtractGroupIndices），这里再写一份必然漂移。
				out[cur].Groups = raw
			}
		case rewardMultipleTag:
			i++
			var nums []uint32
			for i < len(cells) && cells[i].Type == 0 && len(nums) < 3 {
				nums = append(nums, uint32(cells[i].Value))
				i++
			}
			if cur >= 0 && len(nums) >= 2 {
				m := DungeonRewardMultiple{Kind: nums[0], Normal: nums[1]}
				if len(nums) == 3 {
					m.Matching = nums[2]
				}
				out[cur].Multiple = append(out[cur].Multiple, m)
			}
		case rewardSpecialTag, rewardSpecialCustom:
			custom := c.Text == rewardSpecialCustom
			i++
			var name string
			if i < len(cells) && cells[i].Type == 8 {
				name = cells[i].Reference
				if k := stringKey(name); k != "" {
					name = k
				}
				i++
			}
			var nums []uint32
			for i < len(cells) && cells[i].Type == 0 && len(nums) < 4 {
				nums = append(nums, uint32(cells[i].Value))
				i++
			}
			if cur >= 0 && name != "" && len(nums) == 4 {
				out[cur].Special = append(out[cur].Special, DungeonSpecialReward{
					Name: name, Rate: nums[0], List: nums[1], Count: nums[2], Group: nums[3], Custom: custom,
				})
			}
		default:
			i++
		}
	}
	return out
}

// stringKey 把 `<13::name_10326880>` 这样的引用压成键名 `name_10326880`。
//
// 引用形如 `<表序号::键>`；表序号本身没用（键在表里唯一），只留键便于比对与日志。
// 形状不认识时返回空串，调用方保留原样。
func stringKey(ref string) string {
	if len(ref) < 5 || ref[0] != '<' || ref[len(ref)-1] != '>' {
		return ""
	}
	body := ref[1 : len(ref)-1]
	for i := 0; i+1 < len(body); i++ {
		if body[i] == ':' && body[i+1] == ':' {
			return body[i+2:]
		}
	}
	return ""
}
