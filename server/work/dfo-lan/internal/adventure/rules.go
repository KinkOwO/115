package adventure

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed rules.json
var assets embed.FS

type ShopItem struct {
	Template uint32 `json:"template"`
	Level    uint32 `json:"level"`
	Price    uint32 `json:"price"`
	Limit    uint32 `json:"limit"`
	Reset    int    `json:"reset"`
}
type Shop struct {
	MaxPoints   uint32     `json:"max_points"`
	ExpPoints   []uint64   `json:"exp_points"`
	ResetPoints bool       `json:"reset_points"`
	Items       []ShopItem `json:"items"`
}
type Item struct {
	Path        string  `json:"path"`
	SHA256      string  `json:"sha256"`
	Type        string  `json:"type"`
	Limit       uint32  `json:"limit"`
	UsagePeriod []int64 `json:"usage_period"`
}
type Rules struct {
	MaxLevel   uint32            `json:"max_level"`
	ExpRate    float32           `json:"exp_rate"`
	Experience map[uint32]uint64 `json:"experience"`
	Shops      map[byte]Shop     `json:"shops"`
	Items      map[uint32]Item   `json:"items"`
	Sources    map[string]string `json:"sources"`
}

var load = sync.OnceValues(func() (*Rules, error) {
	b, e := assets.ReadFile("rules.json")
	if e != nil {
		return nil, e
	}
	var r Rules
	if e = json.Unmarshal(b, &r); e != nil {
		return nil, e
	}
	if r.MaxLevel != 50 || r.ExpRate <= 0 || len(r.Shops) == 0 {
		return nil, fmt.Errorf("冒险团源规则不完整")
	}
	for n := uint32(2); n <= r.MaxLevel; n++ {
		if r.Experience[n] == 0 {
			return nil, fmt.Errorf("冒险团等级 %d 缺少经验规则", n)
		}
	}
	for category, shop := range r.Shops {
		if category >= 5 || shop.MaxPoints == 0 {
			return nil, fmt.Errorf("冒险团商店类别无效")
		}
		for _, item := range shop.Items {
			if item.Price == 0 || item.Limit == 0 || item.Level < 1 || r.Items[item.Template].Path == "" {
				return nil, fmt.Errorf("冒险团商品 %d 数据不完整", item.Template)
			}
		}
	}
	return &r, nil
})

// 内嵌的是当前 PVF 的只读导出，不依赖运行目录中额外的候选配置。
func Current() (*Rules, error) { return load() }
