package catalog

import "dfolan/internal/catalog/pvf"

// parseSelection walks one script's typed cells. It returns the box's category
// blocks and whether the script carries a fixed [booster info] block, which is
// how a mislabeled fixed box is recognised.
func ParseSelectionCells(cells []pvf.Token) ([]SelectionCategory, bool) {
	var out []SelectionCategory
	fixed := false
	for i := 0; i < len(cells); i++ {
		if cells[i].Type != 3 {
			continue
		}
		switch cells[i].Text {
		case "[booster info]":
			fixed = true
		case "[booster select category]":
			cat, next, ok := parseSelectionCategory(cells, i+1)
			if ok {
				out = append(out, cat)
			}
			i = next
		}
	}
	return out, fixed
}

// parseSelectionCategory reads one [booster select category] block starting at
// the (job, growtype) pair. The source omits the closing tag on a few scripts
// and leaves a stray [equipment] where [/equipment] was intended; the client
// reads the block by its header/end boundary, so a new header or the end of the
// script closes the block, and an [equipment] scan never crosses into the next
// category. Both boundaries are source facts, not a guess between candidates.
func parseSelectionCategory(cells []pvf.Token, i int) (SelectionCategory, int, bool) {
	var cat SelectionCategory
	nums := make([]int32, 0, 2)
	for i < len(cells) && len(nums) < 2 {
		if cells[i].Type == 3 {
			return cat, i, false
		}
		if cells[i].Type == 0 {
			nums = append(nums, cells[i].Value)
		}
		i++
	}
	if len(nums) != 2 {
		return cat, len(cells), false
	}
	cat.Category = [2]byte{byte(nums[0]), byte(nums[1])}
	for i < len(cells) {
		c := cells[i]
		if c.Type != 3 {
			i++
			continue
		}
		if c.Text == "[/booster select category]" {
			return cat, i, true
		}
		// 未闭合的块由下一个 [booster select category] 或脚本结尾收口；不消费
		// 下一个头，交给 ParseSelectionCells 继续解析。
		if c.Text == "[booster select category]" {
			return cat, i - 1, true
		}
		switch c.Text {
		case "[booster equipment grade]":
			i++
			if i < len(cells) && cells[i].Type == 0 {
				cat.Grade = uint32(cells[i].Value)
				i++
			}
		case "[booster equipment upgrade]", "[booster equipment separate]":
			// 成品礼盒在分类块里直接声明打造状态：upgrade=强化、separate=锻造
			// （实机源 590015875/876 的分类 [0 0] 写 12/8）。
			i++
			if i < len(cells) && cells[i].Type == 0 {
				if c.Text == "[booster equipment upgrade]" {
					cat.Reinforce = uint32(cells[i].Value)
				} else {
					cat.Refine = uint32(cells[i].Value)
				}
				i++
			}
		case "[recommend]":
			i++
			if i < len(cells) && cells[i].Type == 0 {
				n := int(cells[i].Value)
				i++
				for k := 0; k < n && i < len(cells) && cells[i].Type == 0; k++ {
					cat.Recommend = append(cat.Recommend, uint32(cells[i].Value))
					i++
				}
			}
		case "[equipment]":
			cat.Sections = append(cat.Sections, c.Text)
			i++
			for i < len(cells) {
				e := cells[i]
				if e.Type == 3 {
					if e.Text == "[/equipment]" {
						i++
						break
					}
					// 源里有个别脚本把 [/equipment] 写成了 [equipment]，或整块缺
					// 尾标签：以类别边界收口，绝不把下一个类别的 (job,growtype)
					// 和条目当成装备条目。
					if e.Text == "[booster select category]" || e.Text == "[/booster select category]" {
						break
					}
					i++
					continue
				}
				if e.Type != 0 {
					i++
					continue
				}
				id := uint32(e.Value)
				i++
				count := uint32(1)
				if i < len(cells) && cells[i].Type == 0 {
					count = uint32(cells[i].Value)
					i++
				}
				cat.Items = append(cat.Items, SelectionItem{Template: id, Count: count})
			}
		case "[stackable]":
			// 成对形态的 `[stackable]` 段按 (模板, 数量) 建模 —— 与 [equipment] 同一口径。
			//
			// ★★ 2026-10-11（「Royal Pass 每日登录礼包」开到的东西只给 1 个的根因）：
			// 590722589 的分类块内容是
			//
			//	[booster select category] 0 0
			//	[stackable] 10327284 90 10327285 45 10362432 50 [/stackable]
			//
			// 而这里原本把 `[stackable]` 和 [avatar]/[etc] 一起当「未建模段」跳过 ⇒
			// `Resolve` 的件数表为空 ⇒ 发放回落到**礼盒自身数量**（源 `[daily reward list]`
			// 写的是 1）⇒ 玩家选了 x45 只拿到 1 个（实机 `gateway.err`：
			// `selection box 590722589: category [0 0] has no exported item set`）。
			//
			// 全量实测（16757 个 `[booster selection]` 候选，2026-10-11）：带 `[stackable]`
			// 的盒子 3178 个、块 **成对 17660 / 奇数 33**。那 33 个是 5 个数字一条的记录
			// （`模板 数量 ? ? ?`，如 10404663 的 `10404771 1 2 115 108 …`），形状未定
			// ⇒ **整块不建模**，并且在 Sections 里记成已声明的 `selectionSectionStackableUnparsed`，
			// 保持旧的「未建模段」语义（`hasUnmodelledSection` 照样拦下重复类的去重）。
			// 宁可少一次校验，也不猜。
			i++
			start := i
			for i < len(cells) {
				e := cells[i]
				if e.Type == 3 {
					if e.Text == "[/stackable]" {
						break
					}
					// 下一个类别头、或源里缺尾标签的写法：以类别边界收口。
					if e.Text == "[booster select category]" || e.Text == "[/booster select category]" {
						break
					}
					i++
					continue
				}
				i++
			}
			if pairs, ok := selectionPairs(cells[start:i]); ok {
				cat.Sections = append(cat.Sections, c.Text)
				cat.Items = append(cat.Items, pairs...)
			} else {
				cat.Sections = append(cat.Sections, selectionSectionStackableUnparsed)
			}
			if i < len(cells) && cells[i].Type == 3 && cells[i].Text == "[/stackable]" {
				i++
			}
		case "[avatar]", "[creature]", "[etc]", "[cera]":
			// 尚未建模的内容段：只记名并跳过。Resolve 见到这些类别时不校验
			// （它们的条目结构各不相同，先把装备段做对再说）。
			cat.Sections = append(cat.Sections, c.Text)
			end := "[/" + c.Text[1:]
			i++
			for i < len(cells) && !(cells[i].Type == 3 && cells[i].Text == end) {
				i++
			}
		default:
			i++
		}
	}
	return cat, len(cells), true
}

// selectionSectionStackableUnparsed 记进 Sections，表示「这个类别带一个形状未定、
// 因此没有建模的 `[stackable]` 块」。`hasUnmodelledSection` 认这个值，
// 于是这类类别仍然走「未建模」的老路（重复类别照旧判冲突、Resolve 不校验）。
const selectionSectionStackableUnparsed = "[stackable] (unparsed)"

// selectionPairs 把内容块里的数字 token 按 (模板, 数量) 成对读出来。
//
// 数字个数为奇数、个数为 0、或出现非正数时返回 ok=false：那说明这个块不是成对形态
// （源里确有一批 5 个数字一条的 `[stackable]` 记录），**宁可不建模也不猜**。
func selectionPairs(tokens []pvf.Token) ([]SelectionItem, bool) {
	nums := make([]int32, 0, len(tokens))
	for _, t := range tokens {
		if t.Type == 0 {
			nums = append(nums, t.Value)
		}
	}
	if len(nums) == 0 || len(nums)%2 != 0 {
		return nil, false
	}
	out := make([]SelectionItem, 0, len(nums)/2)
	for i := 0; i < len(nums); i += 2 {
		if nums[i] <= 0 || nums[i+1] <= 0 {
			return nil, false
		}
		out = append(out, SelectionItem{Template: uint32(nums[i]), Count: uint32(nums[i+1])})
	}
	return out, true
}
