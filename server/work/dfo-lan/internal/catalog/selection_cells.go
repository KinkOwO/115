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
		case "[avatar]", "[creature]", "[etc]", "[stackable]", "[cera]":
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
