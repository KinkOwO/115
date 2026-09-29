package loot

import (
	"crypto/rand"
	"dfolan/internal/catalog"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"slices"
)

// 原版矿区 CTP、booster info、int data 和智能掉落组的完整导出。
// 阶段表只展开一层，保留奖励袋里的材料箱和可合成装备罐。
type BleedingMineRewards struct {
	Source     string                        `json:"source"`
	StageBoxes []uint32                      `json:"stage_boxes"`
	BossBoxes  map[uint32]uint32             `json:"boss_boxes"`
	GroupBoxes []uint32                      `json:"group_boxes"`
	Boxes      map[uint32][]BleedingMineDraw `json:"boxes"`
	Items      map[uint32]catalog.LootItem   `json:"items"`
	Combine    struct {
		Chances   []uint32            `json:"chances"`
		Maximum   uint32              `json:"maximum"`
		Equipment map[string][]uint32 `json:"equipment"`
		Stackable map[string][]uint32 `json:"stackable"`
	} `json:"combine"`
	Undefined []uint32 `json:"undefined_items"`
	NoDrop    []uint32 `json:"no_drop_items"`
}

type BleedingMineDraw struct {
	Draws      uint32               `json:"draws"`
	Candidates []BleedingMineChoice `json:"candidates"`
}

type BleedingMineChoice struct {
	Template int64  `json:"template"`
	Weight   uint32 `json:"weight"`
	Count    uint32 `json:"count"`
}

func LoadBleedingMineRewards(path string) (*BleedingMineRewards, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r BleedingMineRewards
	if err = json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	hash, err := hex.DecodeString(r.Source)
	if err != nil || len(hash) != 32 || len(r.StageBoxes) != 12 || len(r.GroupBoxes) != 3 || len(r.BossBoxes) != 12 || len(r.Undefined) != 0 || len(r.Combine.Chances) != 3 || r.Combine.Maximum != 5 {
		return nil, fmt.Errorf("矿区奖励源不完整")
	}
	// 先检查整个有向图，拒绝循环、占位物品和无法落到真实物品的容器。
	visited := map[uint32]byte{}
	var visit func(uint32) error
	visit = func(id uint32) error {
		if visited[id] == 1 {
			return fmt.Errorf("矿区奖励容器循环：%d", id)
		}
		if visited[id] == 2 {
			return nil
		}
		item, ok := r.Items[id]
		if !ok || item.ID != id || id < 2 || id == 490000001 {
			return fmt.Errorf("矿区奖励物品未定义：%d", id)
		}
		visited[id] = 1
		groups, box := r.Boxes[id]
		if box && len(groups) == 0 {
			return fmt.Errorf("矿区奖励容器为空：%d", id)
		}
		for _, g := range groups {
			if g.Draws == 0 || g.Draws > 100 || len(g.Candidates) == 0 {
				return fmt.Errorf("矿区奖励抽取次数无效：%d", id)
			}
			var total uint64
			for _, c := range g.Candidates {
				if c.Template == 0 || c.Template > int64(^uint32(0)) || c.Count == 0 || c.Count > 1000000 {
					return fmt.Errorf("矿区奖励条目无效：%d", id)
				}
				total += uint64(c.Weight)
				if c.Template > 0 {
					if err := visit(uint32(c.Template)); err != nil {
						return err
					}
				}
			}
			if total == 0 {
				return fmt.Errorf("矿区奖励权重为空：%d", id)
			}
		}
		visited[id] = 2
		return nil
	}
	for id := range r.Boxes {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	for _, ids := range [][]uint32{r.StageBoxes, r.GroupBoxes} {
		for _, id := range ids {
			if _, ok := r.Boxes[id]; !ok {
				return nil, fmt.Errorf("矿区奖励入口缺失：%d", id)
			}
		}
	}
	for _, id := range r.BossBoxes {
		if _, ok := r.Boxes[id]; !ok {
			return nil, fmt.Errorf("矿区领主奖励入口缺失：%d", id)
		}
	}
	return &r, nil
}

// 随机结果由调用方在奖励事务中固定；重发请求不能再次调用抽取。
func (r *BleedingMineRewards) Draw(id uint32) ([]Award, error) {
	groups, ok := r.Boxes[id]
	if !ok {
		return nil, fmt.Errorf("矿区奖励容器未定义：%d", id)
	}
	var out []Award
	for _, g := range groups {
		var total uint64
		for _, c := range g.Candidates {
			total += uint64(c.Weight)
		}
		if total == 0 {
			return nil, fmt.Errorf("矿区奖励权重为空")
		}
		for n := uint32(0); n < g.Draws; n++ {
			value, err := rand.Int(rand.Reader, new(big.Int).SetUint64(total))
			if err != nil {
				return nil, err
			}
			roll := value.Uint64()
			for _, c := range g.Candidates {
				if roll < uint64(c.Weight) {
					// -1、-2 是源空奖签，绝不能取绝对值变成真实物品。
					if c.Template > 0 && !slices.Contains(r.NoDrop, uint32(c.Template)) {
						out = append(out, Award{Template: uint32(c.Template), Amount: c.Count})
					}
					break
				}
				roll -= uint64(c.Weight)
			}
		}
	}
	return out, nil
}

// 奖励袋显示的是装备罐代理；其 int data 和 virtual 智能组必须解析到
// 可发放物品。普通材料箱、卡册和宠物箱保留盒子，沿用正常开箱链路。
func (r *BleedingMineRewards) ResolveSelection(id uint32) ([]Award, error) {
	var out []Award
	var walk func(uint32, uint32, int) error
	walk = func(id, count uint32, depth int) error {
		item, ok := r.Items[id]
		if !ok || count == 0 || depth > 16 || len(out) > 100 {
			return fmt.Errorf("矿区选奖展开超出源边界")
		}
		if item.StackableType != "[upgradable legacy]" && item.StackableType != "[virtual]" {
			out = append(out, Award{Template: id, Amount: count})
			return nil
		}
		if count > 100 {
			return fmt.Errorf("矿区奖励代理数量超限")
		}
		for i := uint32(0); i < count; i++ {
			awards, err := r.Draw(id)
			if err != nil {
				return err
			}
			for _, a := range awards {
				if err = walk(a.Template, a.Amount, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(id, 1, 0); err != nil {
		return nil, err
	}
	return out, nil
}

// 两张同类型卡各贡献50%的源权重，不能把品质档位当成等概率。
func (r *BleedingMineRewards) Compose(first, second uint32) (uint32, error) {
	names := []string{"rare", "unique", "legendary", "epic"}
	rates := []string{"rare rate", "unipue rate", "legendary rate", "epic rate"}
	for _, family := range []map[string][]uint32{r.Combine.Equipment, r.Combine.Stackable} {
		indices := [2]int{-1, -1}
		for n, id := range []uint32{first, second} {
			for tier, name := range names {
				for _, candidate := range family[name+" list"] {
					if id == candidate {
						indices[n] = tier
					}
				}
			}
		}
		if indices[0] < 0 || indices[1] < 0 {
			continue
		}
		var weights [4]uint32
		for _, tier := range indices {
			row := family[rates[tier]]
			if len(row) != 4 {
				return 0, fmt.Errorf("该品质不能继续合成")
			}
			var total uint32
			for i, v := range row {
				total += v
				weights[i] += v
			}
			if total != 50 {
				return 0, fmt.Errorf("矿区合成源权重应为单卡50%%")
			}
		}
		value, err := rand.Int(rand.Reader, big.NewInt(100))
		if err != nil {
			return 0, err
		}
		roll := uint32(value.Uint64())
		for tier, weight := range weights {
			if roll < weight {
				ids := family[names[tier]+" list"]
				if len(ids) == 0 {
					return 0, fmt.Errorf("矿区合成结果目录缺失")
				}
				pick, err := rand.Int(rand.Reader, big.NewInt(int64(len(ids))))
				if err != nil {
					return 0, err
				}
				return ids[pick.Int64()], nil
			}
			roll -= weight
		}
	}
	return 0, fmt.Errorf("矿区合成必须选择两张同类型的有效卡片")
}
