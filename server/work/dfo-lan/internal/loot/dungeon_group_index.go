package loot

import (
	"fmt"

	"dfolan/internal/catalog"
)

// [MERGE-20260928-DUNGEON-GROUP-INDEX] 读副本自己声明的 `[normal group index]`。
//
// 这是「对标客户端」的第二条组来源，与 dungeondropinfo.cos 并列：
//
//	etc/dungeondropinfo.cos            全服副本 → (品级, 类型, 组号, 率)   198 个副本
//	副本脚本 [normal group index]      单个副本 → 组号清单                 3200 个副本
//
// dungeondropinfo 是**全局索引**，所以只有 198 个副本有；`[normal group index]` 写在
// 每个副本自己的 `[difficulty dropitem group list]` 里，覆盖全部 3200 个副本。实机刷的
// 「深渊：最终调律者」100005014 **只在后者里**出现 —— 这就是服务端此前拿不到它的原因。
//
// 源里的形状（只读观察，逐条有出处）：
//
//	[group info]
//	    [item index]         10326880 10326884 10419088 10419090
//	    [normal group index] 1 21251 1 21476
//	    [fame info]          0 0 0 0
//
// `[normal group index]` 的读法是**重复的「个数 + 该个数组号」**：
//
//	1 21251            -> 1 个组：[21251]
//	1 21251 1 21476    -> 两段：1 个组 21251，1 个组 21476
//	2 10900 21030 5 10014 10015 10024 10021 11085
//	                   -> 两段：2 个组 10900/21030，5 个组 10014/10015/10024/10021/11085
//
// 这个读法在三个副本上自洽：100005014（4 个数 → 2 组）、100005068（4 个数 → 2 组）、
// 100003295（9 个数 → 2+5 组）。段与段之间没有分隔符，所以必须按「个数」推进游标；
// 读到越界或个数为 0 就停 —— 不猜。

// ExtractGroupIndices 把一段 `[normal group index]` 的数值读成组号清单。
//
// 返回 error 而不是宽松跳过：形状不认识时上层应当保留全局表结果，而不是拿半截清单一
// 发奖励。
func ExtractGroupIndices(values []int32) ([]uint32, error) {
	var out []uint32
	for i := 0; i < len(values); {
		n := int(values[i])
		i++
		if n < 0 {
			return nil, fmt.Errorf("negative group count %d", n)
		}
		if n == 0 {
			// 个数 0 是合法的空段；继续读下一段。
			continue
		}
		if i+n > len(values) {
			return nil, fmt.Errorf("group count %d exceeds %d remaining values", n, len(values)-i)
		}
		for k := 0; k < n; k++ {
			g := values[i]
			i++
			if g <= 0 {
				return nil, fmt.Errorf("non-positive group id %d", g)
			}
			out = append(out, uint32(g))
		}
	}
	return out, nil
}

// DungeonGroupIndices 取某个副本在指定难度下声明的全部掉落组号。
//
// `[difficulty dropitem group list]` 的每个 `[group info]` 块对应一个难度档，块在
// 源里的顺序就是难度顺序 —— 100005068 有三个块，`[normal group index]` 分别是
// 21600 / 21601 / 21602；100005014 只有一个块（单难度）。
//
// 返回 ok=false 表示这个副本没有声明组索引，调用方应当完整保留全局表的结果。
func DungeonGroupIndices(d catalog.DungeonDefinition, difficulty int) ([]uint32, bool, error) {
	blocks, e := catalog.ParseDungeonDropBlocks(d.Script.Cells)
	if e != nil {
		return nil, false, e
	}
	if len(blocks) == 0 {
		return nil, false, nil
	}
	if difficulty < 0 || difficulty >= len(blocks) {
		// 难度超出块数：不做取模回绕，交给调用方保留全局表结果。
		return nil, false, nil
	}
	for _, s := range blocks[difficulty].Sections {
		if s.Header != "[normal group index]" {
			continue
		}
		gs, e := ExtractGroupIndices(s.Values)
		if e != nil {
			return nil, false, e
		}
		if len(gs) == 0 {
			return nil, false, nil
		}
		return gs, true, nil
	}
	return nil, false, nil
}
