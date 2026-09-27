package inventory

// 本文件把「角色实际穿戴的誓约/引子装备」换算成 noti 2838 要下发的机制档位。
//
// 为什么要这么做（依据见 docs/protocol/endkeeper-of-order-primer-20260926.md §4b/§10）：
// 客户端 `getPrimerGrade()` / `getOathGrade()` 读的是单例 `qword_14E6388B8` 的
// +88/+92，而该单例只由 noti 2838 写入（构造器里是 72 兜底）⇒ **档位完全由服务端
// 决定**。曾经恒发 45：`primer_00_normal_loop.act` 的状态机在 `oath_now == 44`
// 时必然走 `summon_orthaire`，于是隐藏 BOSS（它代表必出太初）场场登场 —— 那正是
// 本文件要消除的。
//
// 档位的真正来源是誓约/引子装备的 `[rarity]`。依据：oathsystemscript.cos 的
// `[base rarity section]`（rare / unique / legendary / epic / primeval）
// 与机制档位 41..45 逐位对位；而实测这类装备的 `[rarity]` 依次是 **2/3/6/4/8**
// —— 注意 **数值序不是机制序**（legendary=6 排在 epic=4 之前），所以必须查表，
// 不能写成 `40 + rarity`。
//
// 表由 `cmd/oathgradeimport` 从内层 PVF 生成（`configs/oath-grades.json`），禁手写。

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// OathGradeNormal 是没穿对应装备时的档位（normal）。它不出隐藏 BOSS。
const OathGradeNormal uint16 = 40

// oathGradeByRarity 把装备脚本的 `[rarity]` 映射成机制档位。
//
// 两边都是已证事实的合成：`[base rarity section]` 的书面顺序
// rare < unique < legendary < epic < primeval 对应档位 41..45（§10.2），
// 而这五类装备的 `[rarity]` 实测是 2 / 3 / 6 / 4 / 8（§10.1 的
// `[seasonlevel oath item]` 表 + 生成器输出的分布）。
var oathGradeByRarity = map[int32]uint16{
	2: 41, // rare
	3: 42, // unique
	6: 43, // legendary
	4: 44, // epic
	8: 45, // primeval —— 唯一会召唤隐藏 BOSS 的档
}

// OathGradeEntry 是表里的一件装备。
type OathGradeEntry struct {
	Family string `json:"family"` // "oath" 或 "primer"
	Rarity int32  `json:"rarity"`
	Grade  int32  `json:"grade,omitempty"`
	Type   string `json:"type"` // 脚本里 [equipment type] 的原文，用来交叉验证
	Path   string `json:"path"`
}

// OathGradeTable 是誓约/引子装备的稀有度表。
type OathGradeTable struct {
	Source  json.RawMessage           `json:"source"`
	Entries map[string]OathGradeEntry `json:"entries"`
	byID    map[uint32]OathGradeEntry
}

// LoadOathGradeTable 读入并校验表。任何一条无法映射的条目都会让加载失败：
// 一张读不动的表会让档位悄悄退回 normal，那比启动就报错更难查。
func LoadOathGradeTable(path string) (*OathGradeTable, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t OathGradeTable
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(t.Entries) == 0 {
		return nil, fmt.Errorf("%s: table has no entries", path)
	}
	t.byID = make(map[uint32]OathGradeEntry, len(t.Entries))
	for k, e := range t.Entries {
		id, err := strconv.ParseUint(k, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("%s: entry key %q is not a decimal item id", path, k)
		}
		if e.Family != "oath" && e.Family != "primer" {
			return nil, fmt.Errorf("%s: id %s has unknown family %q", path, k, e.Family)
		}
		if _, ok := oathGradeByRarity[e.Rarity]; !ok {
			return nil, fmt.Errorf("%s: id %s has rarity %d, which maps to no tier", path, k, e.Rarity)
		}
		t.byID[uint32(id)] = e
	}
	return &t, nil
}

// Len 是表里的装备件数。
func (t *OathGradeTable) Len() int {
	if t == nil {
		return 0
	}
	return len(t.byID)
}

// Grades 返回 (primer, oath) 两个机制档位。
//
// 取**已穿戴装备里最高的那一档**：同一家族可能同时穿多件，玩家的实际等级应当按
// 最好的那件算。一件都没穿时给 OathGradeNormal（不出隐藏 BOSS）—— 这正是
// 「隐藏 BOSS 稀有」的落点。
func (t *OathGradeTable) Grades(worn []BagEquipment) (primer, oath uint16) {
	primer, oath = OathGradeNormal, OathGradeNormal
	if t == nil {
		return primer, oath
	}
	for _, it := range worn {
		e, ok := t.byID[it.Template]
		if !ok {
			continue
		}
		g := oathGradeByRarity[e.Rarity]
		switch e.Family {
		case "oath":
			if g > oath {
				oath = g
			}
		case "primer":
			if g > primer {
				primer = g
			}
		}
	}
	return primer, oath
}
