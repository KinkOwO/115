// Package main 是 115us 单机服的 GM 工具。
//
// 设计原则与仓库里既有的 cmd/admin 一致：不凭空造物品，所有发放都走
// 同一套背包/装备校验与同一个事务，并且每条发放都有 grant-id 幂等键和审计。
// 额外提供的一个便利是：物品按中文名（客户端 PVF 文本表，见 names.go）搜索。
//
// 物品索引有两个来源：
//   - 首选 configs/items.index.json（全量 386230 件，堆叠物 + 装备，覆盖商城/任务/
//     符文/配方等掉落池外物品）；
//   - 回落到 configs/loot.next25.json + configs/equipment.current37.json（原有行为）。
//
// 名字与部位/等级/稀有度这三类"展示与筛选元数据"的来源见 names.go 的包注释。
package main

import (
	"bufio"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// NameTable 是 key -> 文本 的显示名表。
//
// 有两份，优先级不同：
//   - names.client.json：从客户端内层 PVF 的 string/*.uv.str 文本表导出（names.go），
//     这就是游戏里真正显示的文本，**优先**；
//   - runtime/l10n/names.zh.json：work-cn 汉化流水线的中间产物，只作为回落。
type NameTable map[string]string

// 名字表里 GM 工具真正会用到的三类键。names.zh.json / names.en.json 各有 78 万条，
// 其余是技能/任务/界面文本。加载时就地裁剪，避免 66MB 全量常驻。
const (
	namePrefix       = "name_"          // name_<物品ID> 物品名
	growNamePrefix   = "growtype_name_" // growtype_name_<职业号> 职业名，见 main.go 的 classOf
	rarityNamePrefix = "common_rarity_" // common_rarity_<n> 品级名（客户端 string/ui.uv.str）
)

// defaultRarityLabels 是品级（稀有度）中文名的最后兜底。
//
// 依据：客户端内层 PVF 的 string/ui.uv.str 文本表原文（本机实测）：
//
//	common_rarity_0>普通  ..._1>高级  ..._2>稀有  ..._3>神器  ..._4>史诗
//	..._5>勇者  ..._6>传说  ..._7>神话  ..._8>太初
//
// 注意：runtime/l10n/names.zh.json 里同一批键是「..._5=纪事、..._8=原始的」，
// 那是汉化流水线的译文，和客户端不一致 —— 这正好也是「原始的星辰」对不上
// 游戏里「太初之星」的同一个根因。正常运行时这两个键会由 names.client.json 覆盖，
// 这里只是没有该文件时的兜底。
var defaultRarityLabels = []string{
	"普通", "高级", "稀有", "神器", "史诗", "勇者", "传说", "神话", "太初",
}

// loadNames 读取 key -> 文本 的显示名表，并只保留 GM 工具用得到的三类前缀键。
// 裁剪后原表由 GC 回收，不会同时常驻两份。
func loadNames(path string) NameTable {
	raw := NameTable{}
	b, err := os.ReadFile(path)
	if err != nil {
		return raw
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return NameTable{}
	}
	kept := make(NameTable, len(raw)/2)
	for k, v := range raw {
		if strings.HasPrefix(k, namePrefix) || strings.HasPrefix(k, growNamePrefix) ||
			strings.HasPrefix(k, rarityNamePrefix) {
			kept[k] = v
		}
	}
	return kept
}

// 分类的稳定英文 key。中文标签是界面展示用的，key 用于 API 过滤和前端选中态。
const (
	typeConsumable = "consumable"
	typeMaterial   = "material"
	typeQuest      = "quest"
	typeProfession = "profession"
	typeEmblem     = "emblem"
	typeEquipment  = "equipment"
	typeOther      = "other"
)

// ItemType 是一个客户端页签分类及其数量。
type ItemType struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
	// Children 只对装备类出现：各「部位」。
	// 子分类的 key 可以直接当 /api/items 的 type= 或 slot= 用。
	Children []ItemType `json:"children,omitempty"`
}

// orderedTypes 是分类的固定展示顺序（与客户端页签顺序一致，见分类文档 §3）。
var orderedTypes = []ItemType{
	{Key: typeConsumable, Label: "消耗品"},
	{Key: typeMaterial, Label: "材料"},
	{Key: typeQuest, Label: "任务"},
	{Key: typeProfession, Label: "副职业"},
	{Key: typeEmblem, Label: "徽章"},
	{Key: typeEquipment, Label: "装备"},
	{Key: typeOther, Label: "其它"},
}

// stackableTypeKeys 把 PVF 的 [stackable type] 文本映射到客户端页签分类。
//
// 依据：docs/背包物品分类迁移实现报告.md §3「权威槽位区间表」的
// 「对应 PVF [stackable type]」列，以及 §5.1 的分类→槽位映射表。
// 只有该文档明确列出映射关系的 stack_type 才会出现在这里；文档未提及的一律
// 走 §5.1/§3 的兜底规则（默认落入消耗品栏 65~120），不做任何猜测性归类。
var stackableTypeKeys = map[string]string{
	// §3 / §5.1：消耗品栏 65~120
	"[waste]":               typeConsumable, // 药水/消耗品
	"[throw]":               typeConsumable, // 投掷物
	"[booster]":             typeConsumable, // 礼盒
	"[booster selection]":   typeConsumable,
	"[booster random]":      typeConsumable,
	"[cera package]":        typeConsumable, // 福袋/券包
	"[usable cera package]": typeConsumable,
	"[etc]":                 typeConsumable,
	"[contract]":            typeConsumable,
	"[recipe]":              typeConsumable,
	// §3 / §5.1：材料栏 121~176
	"[material]":           typeMaterial,
	"[upgrade limit cube]": typeMaterial,
	// §3：任务栏 177~232
	"[quest]":         typeQuest,
	"[quest receive]": typeQuest,
	// §3：副职业栏 233~288
	"[material expert job]": typeProfession,
	// §3：徽章栏 289~344
	"[avatar emblem]": typeEmblem,
}

// ---------------------------------------------------------------------------
// 装备部位
// ---------------------------------------------------------------------------

// equipmentSlotOrder 是「装备位」的固定顺序，也是游戏内装备栏的排列顺序。
//
// 依据（权威）：configs/equipment-wear.current35.json 的 slots —— 服务端自己
// 定义并校验的装备位表（native_evidence = "current equipment type map initializer"，
// 由 internal/inventory 的 LoadWearRules 加载，并把槽位号限制在 12..25）：
//
//	[weapon]=12 [title name]=13 [coat]=14 [shoulder]=15 [pants]=16 [shoes]=17
//	[waist]=18 [amulet]=19 [wrist]=20 [ring]=21 [support]=22 [magic stone]=23
//	[support weapon]=24 [earring]=25
//
// 这 14 个 cell 就是本工具的部位维度，一个不多一个不少。
// 交叉印证：腾讯 DNF 官方新手引导页「功能型装备共有 13 个装备位置」列出的是同一批
// 位置（武器/称号/肩部/上衣/护腿/腰带/鞋/手镯/项链/戒指/辅助装备/魔法石/耳环），
// 115 版在此之上多出了 24 号「辅助武器」。
//
// 表中没有出现的 cell（护石 [talisman]、融合石 [amalgamation stone]、誓约 [oath]、
// 刻印 [primer]、神器/圣物 [artifact red|blue|green]、旗帜 [flag]、宠物 [creature]、
// 护符 [charm] 等）无法归入上述 14 个装备位，统一落入「其它装备」，不做猜测性归类。
var equipmentSlotOrder = []string{
	"武器", "称号", "上衣", "头肩", "下装", "鞋", "腰带",
	"项链", "手镯", "戒指", "辅助装备", "魔法石", "辅助武器", "耳环",
}

// equipmentSlotCell 是一条 [equipment type] cell -> 装备位 的映射。
type equipmentSlotCell struct {
	cell     string // [equipment type] 的真实文本
	slot     string // 装备位（界面/API 用的中文标签）
	group    string // 大类，仅用于兼容旧的 type=防具/首饰/特殊装备 过滤
	wearSlot int    // 服务端槽位号 12..25；非现役装备位为 0
}

// equipmentSlotCells 逐条枚举装备目录里出现过的全部 [equipment type] 取值。
//
// 依据一（现役 14 个装备位）：configs/equipment-wear.current35.json 的 slots。
//
// 依据二（其余 cell 的完整枚举）：对 configs/equipment.current37.json 全部 19,955 行
// 逐行统计 [equipment type] 的第一个 text cell，实测只出现下面这 22 种取值
// （括号内为该取值在现役目录里的行数）：
//
//	[weapon]6890 [coat]1751 [pants]1632 [shoes]1590 [shoulder]1513 [waist]1505
//	[talisman]910 [amalgamation stone]750 [wrist]502 [amulet]496 [ring]492
//	[magic stone]479 [support]444 [earring]433 [title name]311 [oath]113
//	[primer]76 [artifact red]20 [artifact blue]20 [artifact green]20 [flag]6
//	[creature]2
//
// 22 种全部出现在下表里或走「其它装备」兜底，没有遗漏；19,955 行里
// 没有一行缺少 [equipment type]，也没有一行有第二个 text cell。
//
// 依据三（表的补充枚举）：另外一份全量遍历目录（equipment-full.json，121,726 行）
// 还出现 [support weapon]27 以及一批装扮 cell（[hat avatar] 等），本表一并列出，
// 前者对应服务端 24 号装备位，后者归入「时装」。
var equipmentSlotCells = []equipmentSlotCell{
	// 12..25：服务端现役装备位
	{"[weapon]", "武器", "武器", 12},
	{"[title name]", "称号", "称号", 13},
	{"[coat]", "上衣", "防具", 14},
	{"[shoulder]", "头肩", "防具", 15},
	{"[pants]", "下装", "防具", 16},
	{"[shoes]", "鞋", "防具", 17},
	{"[waist]", "腰带", "防具", 18},
	{"[amulet]", "项链", "首饰", 19},
	{"[wrist]", "手镯", "首饰", 20},
	{"[ring]", "戒指", "首饰", 21},
	{"[support]", "辅助装备", "特殊装备", 22},
	{"[magic stone]", "魔法石", "特殊装备", 23},
	{"[support weapon]", "辅助武器", "特殊装备", 24},
	{"[earring]", "耳环", "特殊装备", 25},
	// 非装备位：装饰/装扮（cell 文本自身就带 " avatar"，见 slotOfEquipmentCell）
	// 非装备位：其余系统（护石/融合石/誓约/刻印/神器/旗帜/宠物/护符）-> 其它装备
}

// 无法归入 14 个装备位时的兜底桶。
const (
	slotOther    = "其它装备"
	slotAvatar   = "时装" // [xxx avatar] 装扮类
	groupOther   = "其它"
	groupAvatar  = "时装"
	avatarSuffix = " avatar]"
)

// slotOfEquipmentCell 把 [equipment type] 的真实 cell 文本映射到装备位与大类。
//
// 现役 14 个装备位走 equipmentSlotCells；形如 "[hat avatar]" 的装扮 cell 归「时装」
// （cell 文本自身就带 " avatar" 后缀，不需要枚举全部 9 种装扮部位）；
// 其余一律「其它装备」，绝不猜测。
func slotOfEquipmentCell(cell string) (slot, group string, wearSlot int) {
	for _, e := range equipmentSlotCells {
		if e.cell == cell {
			return e.slot, e.group, e.wearSlot
		}
	}
	if strings.HasSuffix(cell, avatarSuffix) {
		return slotAvatar, groupAvatar, 0
	}
	return slotOther, groupOther, 0
}

// slotGroupOf 返回某个装备位所属的大类，用于兼容旧的 type=防具/首饰/特殊装备。
func slotGroupOf(slot string) string {
	for _, e := range equipmentSlotCells {
		if e.slot == slot {
			return e.group
		}
	}
	switch slot {
	case slotAvatar:
		return groupAvatar
	case slotOther:
		return groupOther
	}
	return groupOther
}

// ---------------------------------------------------------------------------
// 等级分档
// ---------------------------------------------------------------------------

// LevelBand 是等级筛选的一个可选区间。Max=0 表示没有上限。
//
// 分档依据：configs/equipment.current37.json 全部 19,955 行的 [minimum level]
// 实测范围是 1..115；115 是本客户端（DFO 115 版）的等级上限，因此档位按
// 20 / 50 / 80 / 100 / 110 / 115 切开，并保留 116+ 档（当前无物品，
// 但接口与界面按规格固定给出，方便将来等级上限提高时不必改协议）。
type LevelBand struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Min   int    `json:"min"`
	Max   int    `json:"max"` // 0 = 无上限
}

var levelBands = []LevelBand{
	{"~20", "≤20", 0, 20},
	{"21-50", "21-50", 21, 50},
	{"51-80", "51-80", 51, 80},
	{"81-100", "81-100", 81, 100},
	{"101-110", "101-110", 101, 110},
	{"111-115", "111-115", 111, 115},
	{"116+", "116+", 116, 0},
}

// ---------------------------------------------------------------------------
// 条目与索引
// ---------------------------------------------------------------------------

// ItemEntry 是 GM 界面里可搜索/可发放的一件物品。
type ItemEntry struct {
	ID        uint32 `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	NameEN    string `json:"name_en,omitempty"`
	Grade     int32  `json:"grade"`
	Rarity    int32  `json:"rarity"`
	Level     int32  `json:"level"`
	Type      string `json:"type,omitempty"`       // 原始类型文本：stack_type 或 [equipment type]
	TypeKey   string `json:"type_key,omitempty"`   // 分类稳定 key
	TypeLabel string `json:"type_label,omitempty"` // 分类中文标签
	// Slot 是装备部位（武器/上衣/.../其它装备），只对 kind=equipment 有意义。
	// 值同 TypeLabel，单独留一个字段是为了让接口自解释（slot= 就是筛它）。
	Slot string `json:"slot,omitempty"`
	// Group 是部位所属大类（武器/称号/防具/首饰/特殊装备/时装/其它）。
	Group string `json:"group,omitempty"`
	// RarityLabel 是 rarity 的中文名（客户端 string/ui.uv.str 的 common_rarity_<n>）。
	RarityLabel string `json:"rarity_label,omitempty"`
	Stackable   bool   `json:"stackable"`
	Grantable   bool   `json:"grantable"`
}

// ItemIndex 汇总可发放物品，并提供按 ID / 名称 / 分类 / 部位 / 等级 / 稀有度的检索。
type ItemIndex struct {
	items       []ItemEntry
	byID        map[uint32]int
	namesClient NameTable // 客户端 PVF 文本表，第一优先
	namesZH     NameTable // 汉化流水线产物，回落
	namesEN     NameTable
	slotMap     map[string]slotMapEntry // id 字符串 -> [部位cell, 最低等级]
	source      string                  // "items.index.json" 或 "loot+equipment"
	path        string                  // 实际读取的物品库路径（回落时为空）
	count       int                     // items.index.json 里声明的件数
}

// IndexItem 是 items.index.json 里的一件物品。
// 实测字段集合：id/name/kind/grade/rarity/stack_type/stack_limit/durability。
// 注意：该文件**没有** level 字段（装备的等级要靠装备目录补），也没有
// [equipment type]（部位同样靠装备目录补）。
// 该文件 43MB，用 json.Decoder 流式解码 items 数组，避免再复制一份字节。
type IndexItem struct {
	ID         uint32 `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Grade      int32  `json:"grade"`
	Rarity     int32  `json:"rarity"`
	StackType  string `json:"stack_type,omitempty"`
	StackLimit uint32 `json:"stack_limit,omitempty"`
	Durability uint32 `json:"durability,omitempty"`
	NameKey    string `json:"-"`
	NativeName bool   `json:"-"`
}

func nameKey(ref string) string {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimPrefix(ref, "<")
	ref = strings.TrimSuffix(ref, ">")
	if i := strings.Index(ref, "::"); i >= 0 {
		return ref[i+2:]
	}
	return ref
}

// display 按 客户端 PVF -> 汉化流水线 -> 英文 的顺序取显示名。
func (ix *ItemIndex) display(key string) (string, string) {
	if key == "" {
		return "", ""
	}
	en := ix.namesEN[key]
	zh := ix.namesClient[key]
	if zh == "" {
		zh = ix.namesZH[key]
	}
	if zh == "" {
		zh = en
	}
	return zh, en
}

// cellText 取出第一个带 text 的 cell 文本（PVF 枚举型 cell）。
func cellText(cells []pvf.Token) string {
	for _, c := range cells {
		if c.Text != "" {
			return c.Text
		}
	}
	return ""
}

// cellInt 取出第一个数值型 cell（PVF int cell 的 Type 为 0）。
func cellInt(cells []pvf.Token) int32 {
	for _, c := range cells {
		if c.Type == 0 {
			return c.Value
		}
	}
	return 0
}

// classifyStackable 把堆叠物的 stack_type 映射到分类 key。
// 未知 stack_type 按分类文档 §5.1 的兜底规则归入消耗品（默认槽位区间 65~120）。
func classifyStackable(stackType string) (key, label string) {
	if k, ok := stackableTypeKeys[stackType]; ok {
		return k, labelOf(k)
	}
	return typeConsumable, labelOf(typeConsumable)
}

func labelOf(key string) string {
	for _, t := range orderedTypes {
		if t.Key == key {
			return t.Label
		}
	}
	return key
}

// resolveStackableFallback 从背包规则里读消耗品兜底区间对应的分类 key（默认 [65,120]）。
// 依据：分类文档 §5.1「建议在 BagRules 中新增 DefaultStackableSlots，默认 [65, 120]」。
func resolveStackableFallback(rules inventory.BagRules) string {
	rng, ok := rules.Slots["[waste]"]
	if !ok {
		return typeConsumable
	}
	if rng[0] >= 121 && rng[1] <= 176 {
		return typeMaterial
	}
	return typeConsumable
}

// rarityLabel 返回品级的中文名：客户端名字表 -> 汉化名字表 -> 英文表 -> 内置兜底。
func (ix *ItemIndex) rarityLabel(v int32) string {
	if v < 0 {
		return ""
	}
	key := rarityNamePrefix + strconv.Itoa(int(v))
	if s := ix.namesClient[key]; s != "" {
		return s
	}
	if s := ix.namesZH[key]; s != "" {
		return s
	}
	if s := ix.namesEN[key]; s != "" {
		return s
	}
	if int(v) < len(defaultRarityLabels) {
		return defaultRarityLabels[v]
	}
	return ""
}

// RarityLabel 是给外部（main.go 的接口层）用的品级名查询。
func (ix *ItemIndex) RarityLabel(v int32) string { return ix.rarityLabel(v) }

// RarityIndex 把用户给的稀有度过滤值转成品级序号。
//
// 接受三种写法，返回 ok=false 表示无法识别（调用方应当报 400，而不是静默给出空结果）：
//  1. 序号，0..8；
//  2. 客户端品级名（普通/高级/稀有/神器/史诗/勇者/传说/神话/太初）；
//  3. 别名：汉化流水线 names.zh.json / names.en.json 里的同一批键 —— 那里
//     common_rarity_5 是「纪事」、_8 是「原始的」，和客户端不一致。保留别名是为了
//     不让"以前记的名字"直接报错，但**显示**只用客户端那份。
func (ix *ItemIndex) RarityIndex(token string) (int, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return -1, false
	}
	if n, err := strconv.Atoi(token); err == nil {
		// 客户端 string/ui.uv.str 只定义 common_rarity_0..8，超出的都是写错的值，
		// 直接判为非法让接口返回 400，而不是悄悄给出 0 条结果。
		if n < 0 || n >= len(defaultRarityLabels) {
			return -1, false
		}
		return n, true
	}
	// 先按客户端品级名匹配（界面显示的就是这些），再退到别名。
	for i := 0; i < len(defaultRarityLabels); i++ {
		if ix.rarityLabel(int32(i)) == token {
			return i, true
		}
	}
	for _, table := range []NameTable{ix.namesEN, ix.namesZH} {
		for i := 0; i < len(defaultRarityLabels); i++ {
			if s := table[rarityNamePrefix+strconv.Itoa(i)]; s != "" && s == token {
				return i, true
			}
		}
	}
	return -1, false
}

// IndexOptions 汇总建立索引需要的全部输入，避免 LoadItemIndex 长成一串位置参数。
type IndexOptions struct {
	Configs     string
	IndexPath   string
	Gear        *inventory.EquipmentCatalog
	SlotMap     map[string]slotMapEntry
	NamesClient NameTable
	NamesZH     NameTable
	NamesEN     NameTable
}

// LoadItemIndex 建立物品索引。
//
// 优先使用 IndexPath 指向的 items.index.json；文件不存在时回落到
// configs/loot.next25.json + configs/equipment.current37.json，
// 并通过 note 返回提示文本（由调用方打印）。
func LoadItemIndex(o IndexOptions) (*ItemIndex, string, error) {
	ix := &ItemIndex{
		byID:        map[uint32]int{},
		namesClient: o.NamesClient,
		namesZH:     o.NamesZH,
		namesEN:     o.NamesEN,
		slotMap:     o.SlotMap,
	}
	if o.IndexPath != "" {
		if _, err := os.Stat(o.IndexPath); err == nil {
			if err := ix.loadItemsIndexFile(o.IndexPath, o.Gear); err != nil {
				return nil, "", fmt.Errorf("载入物品库 %s: %w", o.IndexPath, err)
			}
			ix.finish()
			return ix, "", nil
		}
	}
	note := fmt.Sprintf("未找到物品库 %s，回落到 loot.next25.json + equipment.current37.json（物品数会远少于 386230）", o.IndexPath)
	if err := ix.loadFromCatalogs(o.Configs); err != nil {
		return nil, note, err
	}
	ix.finish()
	return ix, note, nil
}

// loadItemsIndexFile 读取全量物品库。43MB，只解析一次并常驻。
func (ix *ItemIndex) loadItemsIndexFile(path string, gear *inventory.EquipmentCatalog) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	dec := json.NewDecoder(bufio.NewReaderSize(f, 1<<20))
	// 顶层对象开括号
	if _, err := dec.Token(); err != nil {
		return err
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return err
		}
		key, _ := keyTok.(string)
		switch key {
		case "count":
			if err := dec.Decode(&ix.count); err != nil {
				return err
			}
		case "items":
			var batch []IndexItem
			if err := dec.Decode(&batch); err != nil {
				return err
			}
			ix.absorb(batch)
		default:
			// 跳过不认识的顶层字段
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return err
			}
		}
	}

	// items 数组的条目没有装备目录细化信息，这里补齐 [equipment type]（部位）
	// 与 [minimum level]（最低等级）。
	ix.refineEquipment(gear)
	ix.source, ix.path = "items.index.json", path
	return nil
}

// absorb 把一批物品并入索引。ID 重复时保留先出现的一条（实测 386230 件无重复）。
func (ix *ItemIndex) absorb(batch []IndexItem) {
	for _, it := range batch {
		if _, ok := ix.byID[it.ID]; ok {
			continue
		}
		ix.push(it)
	}
}

func (ix *ItemIndex) push(it IndexItem) {
	e := ItemEntry{
		ID: it.ID, Kind: it.Kind, NameEN: it.Name,
		Grade: it.Grade, Rarity: it.Rarity,
		RarityLabel: ix.rarityLabel(it.Rarity),
		Grantable:   true, Stackable: it.Kind == "stackable",
	}
	key := it.NameKey
	if !it.NativeName {
		key = ix.nameKeyFor(it.ID)
	}
	if zh, _ := ix.display(key); zh != "" {
		e.Name = zh
	} else {
		e.Name = it.Name
	}
	switch it.Kind {
	case "stackable":
		e.Type = it.StackType
		e.TypeKey, e.TypeLabel = classifyStackable(it.StackType)
	case "equipment":
		// 装备库文件里没有 [equipment type]，先按「其它装备」占位；
		// refineEquipment 会用装备目录/部位表的真实 cell 文本覆盖。
		e.TypeKey, e.TypeLabel = slotOther, slotOther
		e.Slot, e.Group = slotOther, groupOther
	default:
		e.Type = it.Kind
		e.TypeKey, e.TypeLabel = typeOther, labelOf(typeOther)
	}
	ix.byID[it.ID] = len(ix.items)
	ix.items = append(ix.items, e)
}

// refineEquipment 给装备类物品补上部位（来自 [equipment type] 的真实 cell 文本）
// 与最低等级（来自 [minimum level]）。
//
// 两个来源，按可用性取：
//  1. 离线生成的装备部位表 equipment.slots.json（names.go 的 -build-data 产出，
//     来自 configs/equipment.current37.json 加一份全量遍历目录），覆盖率高；
//  2. 服务端现役装备目录 gear（main.go 已加载，internal/inventory 校验过 source），
//     覆盖 19,955 行，作为没有部位表时的回落。
func (ix *ItemIndex) refineEquipment(gear *inventory.EquipmentCatalog) {
	fromMap, fromGear := 0, 0
	for i := range ix.items {
		it := &ix.items[i]
		if it.Kind != "equipment" {
			continue
		}
		if e, ok := ix.slotMap[idKey(it.ID)]; ok {
			ix.applySlot(it, e.Cell, e.Level)
			fromMap++
		}
	}
	if gear != nil {
		for _, row := range gear.Rows {
			i, ok := ix.byID[row.ID]
			if !ok {
				continue
			}
			it := &ix.items[i]
			if it.Kind != "equipment" {
				continue
			}
			cell := cellText(row.Fields["[equipment type]"])
			if cell == "" {
				continue
			}
			// 部位表已经给过的就不覆盖（两张表对同一 id 的部位文本实测一致）。
			if it.Slot != slotOther {
				continue
			}
			ix.applySlot(it, cell, cellInt(row.Fields["[minimum level]"]))
			fromGear++
		}
	} else {
		fmt.Fprintln(os.Stderr, "提示：装备目录不可用，装备类只按部位表分部位")
	}
	fmt.Fprintf(os.Stderr, "物品库装备细分：部位表命中 %d 件，现役装备目录补 %d 件\n", fromMap, fromGear)
}

// applySlot 把 [equipment type] cell 文本与最低等级写到条目上。
func (ix *ItemIndex) applySlot(it *ItemEntry, cell string, level int32) {
	slot, group, _ := slotOfEquipmentCell(cell)
	it.Slot, it.Group = slot, group
	it.TypeKey, it.TypeLabel = slot, slot
	if it.Type == "" {
		it.Type = cell
	}
	if it.Level == 0 && level > 0 {
		it.Level = level
	}
}

// loadFromCatalogs 是原有行为：从掉落目录 + 装备目录建立索引。
// equipCatalog 缺失时只索引普通消耗品/材料。
func (ix *ItemIndex) loadFromCatalogs(configs string) error {
	lootPath := filepath.Join(configs, "loot.next25.json")
	lootCatalog, err := catalog.LoadLoot(lootPath)
	if err != nil {
		return fmt.Errorf("载入掉落目录 %s: %w", lootPath, err)
	}

	// 兜底分类需要知道背包规则里消耗品区间的归属；取不到就按文档默认 [65,120]。
	var rules inventory.BagRules
	if r, err := inventory.LoadBagRules(filepath.Join(configs, "inventory.next29.json")); err == nil {
		rules = r
	}
	fallbackKey := resolveStackableFallback(rules)
	fallbackLabel := labelOf(fallbackKey)

	for id, item := range lootCatalog.Items {
		key := ""
		for _, c := range item.Script.Cells {
			if c.Type == 8 {
				key = nameKey(c.Reference)
				break
			}
		}
		zh, en := ix.display(key)
		typeKey, typeLabel := classifyStackable(item.StackableType)
		if _, known := stackableTypeKeys[item.StackableType]; !known {
			typeKey, typeLabel = fallbackKey, fallbackLabel
		}
		ix.byID[id] = len(ix.items)
		ix.items = append(ix.items, ItemEntry{
			ID: id, Kind: "stackable", Name: zh, NameEN: en,
			Grade: item.Grade, Rarity: item.Rarity, RarityLabel: ix.rarityLabel(item.Rarity),
			Type: item.StackableType, TypeKey: typeKey, TypeLabel: typeLabel,
			Stackable: true, Grantable: true,
		})
	}

	eqPath := filepath.Join(configs, "equipment.current37.json")
	if gear, err := inventory.LoadEquipmentCatalog(eqPath, lootCatalog.Source.Checksum); err == nil {
		for _, row := range gear.Rows {
			key := ""
			if cells, ok := row.Fields["[name]"]; ok {
				for _, c := range cells {
					if c.Type == 8 {
						key = nameKey(c.Reference)
						break
					}
				}
			}
			zh, en := ix.display(key)
			var grade, rarity, level int32
			if cells, ok := row.Fields["[grade]"]; ok {
				grade = cellInt(cells)
			}
			if cells, ok := row.Fields["[rarity]"]; ok {
				rarity = cellInt(cells)
			}
			if cells, ok := row.Fields["[minimum level]"]; ok {
				level = cellInt(cells)
			}
			if _, dup := ix.byID[row.ID]; dup {
				continue
			}
			// 部位必须来自目录里的真实 cell 文本。
			slot, group, _ := slotOfEquipmentCell(cellText(row.Fields["[equipment type]"]))
			ix.byID[row.ID] = len(ix.items)
			ix.items = append(ix.items, ItemEntry{
				ID: row.ID, Kind: "equipment", Name: zh, NameEN: en,
				Grade: grade, Rarity: rarity, RarityLabel: ix.rarityLabel(rarity),
				Level: level, Type: cellText(row.Fields["[equipment type]"]),
				TypeKey: slot, TypeLabel: slot, Slot: slot, Group: group,
				Grantable: true,
			})
		}
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "警告：装备目录不可用（%v），只能发放普通物品\n", err)
	}

	ix.source = "loot+equipment"
	return nil
}

// finish 排序并按 ID 重建下标。
func (ix *ItemIndex) finish() {
	sort.Slice(ix.items, func(i, j int) bool { return ix.items[i].ID < ix.items[j].ID })
	for i := range ix.items {
		ix.byID[ix.items[i].ID] = i
	}
}

func (ix *ItemIndex) Count() int { return len(ix.items) }

// Source 说明索引来自哪里，供 /api/overview 展示。
func (ix *ItemIndex) Source() string {
	if ix.source == "" {
		return "unknown"
	}
	if ix.path != "" && ix.count > 0 && ix.count != len(ix.items) {
		return fmt.Sprintf("%s（声明 %d 件，实际载入 %d 件）", ix.source, ix.count, len(ix.items))
	}
	return ix.source
}

func (ix *ItemIndex) Get(id uint32) (ItemEntry, bool) {
	if i, ok := ix.byID[id]; ok {
		return ix.items[i], true
	}
	return ItemEntry{}, false
}

// itemTypeKey 归一化物品的分类 key 用于统计与过滤。
func itemTypeKey(it *ItemEntry) string {
	if it.Kind == "equipment" {
		// 装备的 TypeKey 是部位；顶层「装备」是聚合桶。
		return typeEquipment
	}
	if it.TypeKey == "" {
		return typeOther
	}
	return it.TypeKey
}

// Types 返回分类列表与每类数量，顺序固定为 orderedTypes。
// 装备类额外带上各「部位」的数量与稳定 key。
func (ix *ItemIndex) Types() []ItemType {
	counts := map[string]int{}
	subs := map[string]int{}
	for i := range ix.items {
		it := &ix.items[i]
		counts[itemTypeKey(it)]++
		if it.Kind == "equipment" {
			subs[it.Slot]++
		}
	}
	out := make([]ItemType, 0, len(orderedTypes))
	for _, t := range orderedTypes {
		t.Count = counts[t.Key]
		if t.Key == typeEquipment {
			t.Children = subItems(subs)
		}
		out = append(out, t)
	}
	return out
}

// subItems 按装备位固定顺序把数量表铺成子分类，只保留数量大于 0 的。
func subItems(counts map[string]int) []ItemType {
	var out []ItemType
	for _, label := range equipmentSlotOrder {
		if n := counts[label]; n > 0 {
			out = append(out, ItemType{Key: label, Label: label, Count: n})
		}
	}
	for _, label := range []string{slotAvatar, slotOther} {
		if n := counts[label]; n > 0 {
			out = append(out, ItemType{Key: label, Label: label, Count: n})
		}
	}
	return out
}

// FilterOption 是 /api/filters 返回的一个可选项（部位 / 等级档 / 品级）。
type FilterOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
	// 下面三个只对部位有意义。
	Cell     string `json:"cell,omitempty"`      // 原始 [equipment type] 文本
	WearSlot int    `json:"wear_slot,omitempty"` // 服务端装备位号 12..25，非现役位为 0
	Group    string `json:"group,omitempty"`     // 所属大类
	// 下面两个只对等级档有意义。
	Min int `json:"min,omitempty"`
	Max int `json:"max,omitempty"` // 0 = 无上限
}

// Filters 是 /api/filters 的返回体：界面三个新筛选控件的可选值与数量。
type Filters struct {
	Slots    []FilterOption `json:"slots"`
	Levels   []FilterOption `json:"levels"`
	Rarities []FilterOption `json:"rarities"`
	// RaritySource 说明品级中文名取自哪里，界面直接显示，避免"名字对不上"再被当成 bug。
	RaritySource string `json:"rarity_source"`
	// Equipment/SlotCovered/SlotUnknown 说明部位覆盖率：
	// items.index.json 里 38 万件物品中只有装备有部位，且装备里也只有
	// 装备目录覆盖到的那些能给出部位，其余落到「其它装备」。
	Equipment    int `json:"equipment"`
	SlotCovered  int `json:"slot_covered"`
	SlotUnknown  int `json:"slot_unknown"`
	LevelKnown   int `json:"level_known"`
	LevelUnknown int `json:"level_unknown"`
}

// Filters 统计三个筛选维度的可选值与数量。
func (ix *ItemIndex) Filters() Filters {
	slotCount := map[string]int{}
	levelCount := make([]int, len(levelBands))
	rarityCount := map[int32]int{}
	var f Filters
	for i := range ix.items {
		it := &ix.items[i]
		if it.Kind == "equipment" {
			f.Equipment++
			slotCount[it.Slot]++
			if it.Slot == slotOther {
				f.SlotUnknown++
			} else {
				f.SlotCovered++
			}
			if it.Level > 0 {
				f.LevelKnown++
				for bi, band := range levelBands {
					if int(it.Level) >= band.Min && (band.Max == 0 || int(it.Level) <= band.Max) {
						levelCount[bi]++
						break
					}
				}
			} else {
				f.LevelUnknown++
			}
		}
		rarityCount[it.Rarity]++
	}

	for _, label := range equipmentSlotOrder {
		cell, wear, group := "", 0, slotGroupOf(label)
		for _, e := range equipmentSlotCells {
			if e.slot == label {
				cell, wear = e.cell, e.wearSlot
				break
			}
		}
		f.Slots = append(f.Slots, FilterOption{
			Key: label, Label: label, Count: slotCount[label], Cell: cell, WearSlot: wear, Group: group,
		})
	}
	for _, label := range []string{slotAvatar, slotOther} {
		f.Slots = append(f.Slots, FilterOption{
			Key: label, Label: label, Count: slotCount[label], Group: slotGroupOf(label),
		})
	}

	for bi, band := range levelBands {
		f.Levels = append(f.Levels, FilterOption{
			Key: band.Key, Label: band.Label, Count: levelCount[bi], Min: band.Min, Max: band.Max,
		})
	}

	for r := int32(0); r < int32(len(defaultRarityLabels)); r++ {
		f.Rarities = append(f.Rarities, FilterOption{
			Key: strconv.Itoa(int(r)), Label: ix.rarityLabel(r), Count: rarityCount[r],
		})
	}

	switch {
	case ix.namesClient[rarityNamePrefix+"8"] != "":
		f.RaritySource = "客户端 PVF string/ui.uv.str（与游戏一致）"
	case ix.namesZH[rarityNamePrefix+"8"] != "":
		f.RaritySource = "汉化流水线 names.zh.json（与游戏可能不一致）"
	default:
		f.RaritySource = "内置兜底（取自客户端 string/ui.uv.str 原文）"
	}
	return f
}

// ItemFilter 是一次物品查询的全部条件，字段之间是「与」的关系。
type ItemFilter struct {
	Q        string
	Type     string // 客户端页签分类 / 装备大类 / 装备位
	Slot     string // 装备位（中文标签或 [cell] 文本）
	LevelMin int    // 0 表示不限（等级未知的物品不会被返回）
	LevelMax int    // 0 表示不限
	Rarity   int    // 品级序号，仅当 RaritySet 为真时生效
	// RaritySet 单独用一个布尔量，是因为 0（普通）本身是合法品级，
	// 不能拿零值当"不限"，否则「不筛品级」会退化成「只看普通」。
	RaritySet bool
	Limit     int
}

// Empty 判断是否一个条件都没给。没给条件时按"低等级一批"列出，
// 给了任意条件就老老实实按条件列，避免筛选结果被默认门槛吃掉。
func (f ItemFilter) Empty() bool {
	return strings.TrimSpace(f.Q) == "" && f.Type == "" && f.Slot == "" &&
		f.LevelMin == 0 && f.LevelMax == 0 && !f.RaritySet
}

// Search 按条件检索物品。
//
// 规则（与界面联动一致）：
//   - Type 命中客户端页签分类、装备大类或装备位（见 inType）；
//   - Slot 单独筛装备位，接受中文标签（"武器"）或原始 cell（"[weapon]"）；
//   - LevelMin/LevelMax 按最低等级筛；等级未知（0）的物品在给了等级条件时被排除；
//   - RaritySet 为真时按品级序号筛；
//   - Q 为空或纯数字时按 ID 前缀匹配，否则按中英文名做前缀优先的子串匹配；
//   - 一个条件都没给时只列低等级的一批，避免把 38 万件一次吐出去。
func (ix *ItemIndex) Search(f ItemFilter) []ItemEntry {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 60
	}
	f.Q = strings.TrimSpace(f.Q)
	f.Type = strings.TrimSpace(f.Type)
	f.Slot = strings.TrimSpace(f.Slot)
	defaultGate := f.Empty()

	out := []ItemEntry{}
	if f.Q == "" || isDigits(f.Q) {
		for i := range ix.items {
			it := &ix.items[i]
			if !ix.match(it, f) {
				continue
			}
			if defaultGate {
				// 没给关键字时给一批低等级、最可能想发的物品
				if it.Level > 10 || it.Grade > 3 {
					continue
				}
			} else if f.Q != "" && !strings.HasPrefix(strconv.FormatUint(uint64(it.ID), 10), f.Q) {
				continue
			}
			out = append(out, *it)
			if len(out) >= f.Limit {
				break
			}
		}
		return out
	}

	needle := strings.ToLower(f.Q)
	// 先精确前缀命中，再子串命中，让"力量"这类短词排在前面。
	hit := map[uint32]bool{}
	for _, pass := range []int{0, 1} {
		for i := range ix.items {
			it := &ix.items[i]
			if !ix.match(it, f) || hit[it.ID] {
				continue
			}
			n := strings.ToLower(it.Name)
			ne := strings.ToLower(it.NameEN)
			var ok bool
			if pass == 0 {
				ok = strings.HasPrefix(n, needle) || strings.HasPrefix(ne, needle)
			} else {
				ok = strings.Contains(n, needle) || strings.Contains(ne, needle)
			}
			if ok {
				hit[it.ID] = true
				out = append(out, *it)
				if len(out) >= f.Limit {
					return out
				}
			}
		}
	}
	return out
}

// match 判断一件物品是否满足过滤条件（不含关键字）。
func (ix *ItemIndex) match(it *ItemEntry, f ItemFilter) bool {
	if !ix.inType(it, f.Type) {
		return false
	}
	if f.Slot != "" {
		if it.Kind != "equipment" {
			return false
		}
		want := f.Slot
		if strings.HasPrefix(want, "[") {
			// 允许直接传原始 cell 文本
			if s, _, _ := slotOfEquipmentCell(want); s != "" {
				want = s
			}
		}
		if it.Slot != want {
			return false
		}
	}
	if f.LevelMin > 0 || f.LevelMax > 0 {
		// 等级未知（0）在给了等级条件时按"不匹配"处理，避免把 38 万件
		// 没有等级信息的物品混进"101-110"这种结果里。
		if it.Level <= 0 {
			return false
		}
		if f.LevelMin > 0 && int(it.Level) < f.LevelMin {
			return false
		}
		if f.LevelMax > 0 && int(it.Level) > f.LevelMax {
			return false
		}
	}
	if f.RaritySet && int(it.Rarity) != f.Rarity {
		return false
	}
	return true
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// inType 判断物品是否属于指定分类；typeKey 为空表示不过滤。
//
// 兼容三层语义：
//   - type=equipment 匹配所有装备；
//   - type=武器/上衣/…/其它装备 匹配具体装备位（stackable 侧则是页签分类）；
//   - type=防具/首饰/特殊装备/时装 等旧大类仍然可用，等价于该大类下所有装备位。
func (ix *ItemIndex) inType(it *ItemEntry, typeKey string) bool {
	if typeKey == "" {
		return true
	}
	if it.Kind == "equipment" {
		if typeKey == typeEquipment {
			return true
		}
		if it.Slot == typeKey || it.TypeKey == typeKey {
			return true
		}
		return slotGroupOf(it.Slot) == typeKey
	}
	key := it.TypeKey
	if key == "" {
		key = typeOther
	}
	return key == typeKey
}

// nameKeyFor 拼出物品名的名字表键 name_<id>。
// 用 AppendUint 直接拼字节，避免对 38 万件物品各做一次字符串分配。
func (ix *ItemIndex) nameKeyFor(id uint32) string {
	buf := make([]byte, 0, len(namePrefix)+10)
	buf = append(buf, namePrefix...)
	buf = strconv.AppendUint(buf, uint64(id), 10)
	return string(buf)
}

// idKey 拼出物品 id 的十进制字符串键（部位表的键）。
func idKey(id uint32) string {
	buf := make([]byte, 0, 10)
	return string(strconv.AppendUint(buf, uint64(id), 10))
}
