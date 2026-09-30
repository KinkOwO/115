package adventure

import (
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
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
	SourceChecksum string            `json:"pvf_sha256"`
	MaxLevel       uint32            `json:"max_level"`
	ExpRate        float32           `json:"exp_rate"`
	Experience     map[uint32]uint64 `json:"experience"`
	Shops          map[byte]Shop     `json:"shops"`
	Items          map[uint32]Item   `json:"items"`
	Sources        map[string]string `json:"sources"`
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
	return NewRules(r)
})

func NewRules(r Rules) (*Rules, error) {
	if b, err := hex.DecodeString(r.SourceChecksum); err != nil || len(b) != 32 {
		return nil, fmt.Errorf("invalid adventure source identity")
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
}

var currentRules atomic.Pointer[Rules]

// EmbeddedRules is the legacy projection used only for fallback or an explicit
// migration audit. Current does not read it after native startup installation.
func EmbeddedRules() (*Rules, error) { return load() }

func Current() (*Rules, error) {
	if r := currentRules.Load(); r != nil {
		return r, nil
	}
	return load()
}

// InstallRules is called before storage opens and before serving requests.
// The returned restoration function allows isolated startup verification.
func InstallRules(source *Rules) (func(), error) {
	if source == nil {
		return nil, fmt.Errorf("nil adventure rules")
	}
	r := *source
	r.Experience = make(map[uint32]uint64, len(source.Experience))
	for k, v := range source.Experience {
		r.Experience[k] = v
	}
	r.Shops = make(map[byte]Shop, len(source.Shops))
	for k, v := range source.Shops {
		v.ExpPoints = append([]uint64(nil), v.ExpPoints...)
		v.Items = append([]ShopItem(nil), v.Items...)
		r.Shops[k] = v
	}
	r.Items = make(map[uint32]Item, len(source.Items))
	for k, v := range source.Items {
		v.UsagePeriod = append([]int64(nil), v.UsagePeriod...)
		r.Items[k] = v
	}
	r.Sources = make(map[string]string, len(source.Sources))
	for k, v := range source.Sources {
		r.Sources[k] = v
	}
	validated, err := NewRules(r)
	if err != nil {
		return nil, err
	}
	previous := currentRules.Swap(validated)
	return func() { currentRules.Store(previous) }, nil
}
