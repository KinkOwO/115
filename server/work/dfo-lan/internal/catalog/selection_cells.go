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
		return cat, i, false
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
		switch c.Text {
		case "[booster equipment grade]":
			i++
			if i < len(cells) && cells[i].Type == 0 {
				cat.Grade = uint32(cells[i].Value)
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
			for i < len(cells) && !(cells[i].Type == 3 && cells[i].Text == "[/equipment]") {
				if cells[i].Type != 0 {
					i++
					continue
				}
				id := uint32(cells[i].Value)
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
	return cat, i, false
}
