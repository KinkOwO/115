package inventory

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

type EquipmentDefinition struct {
	ID           uint32
	Path, SHA256 string
	Fields       map[string][]pvf.Token
	fameFields   map[string][]pvf.Token
	fameLevels   []equipmentFameLevel
}
type EquipmentCatalog struct {
	Full   *FullEquipmentCatalog `json:"-"`
	Source pvf.ArchiveSnapshot   `json:"source"`
	Rows   []EquipmentDefinition `json:"rows"`
	// Separate from the historical selection used by deferred special modes.
	OrdinaryPool  []EquipmentDrop `json:"ordinary_pool,omitempty"`
	HellPartyPool []EquipmentDrop `json:"-"`
	index         map[uint32]EquipmentDefinition
	pool          []EquipmentDrop
}

// EquipmentDrop is one piece of gear this build can actually place in a bag,
// with the source fields a drop roll selects on.
type EquipmentDrop struct {
	ID            uint32
	Grade, Rarity int32
	Durability    uint16
	Weight        uint32 `json:",omitempty"`
}

// DropPool is the catalog's bag-usable gear, projected once at load. Basic
// decides membership, so anything the pool offers can always be granted.
func (c *EquipmentCatalog) DropPool() []EquipmentDrop {
	if c == nil {
		return nil
	}
	return c.pool
}

func (c *EquipmentCatalog) Durability(id uint32) (uint16, bool) {
	if c == nil {
		return 0, false
	}
	for _, d := range c.pool {
		if d.ID == id {
			return d.Durability, true
		}
	}
	return 0, false
}

func LoadEquipmentCatalog(path, source string) (*EquipmentCatalog, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c EquipmentCatalog
	if e = json.Unmarshal(b, &c); e != nil {
		return nil, e
	}
	return NewEquipmentCatalog(c, source)
}

func NewEquipmentCatalog(c EquipmentCatalog, source string) (*EquipmentCatalog, error) {
	c.pool = nil
	if c.Source.Checksum != source || len(c.Rows) == 0 {
		return nil, fmt.Errorf("equipment source mismatch")
	}
	c.index = map[uint32]EquipmentDefinition{}
	for _, r := range c.Rows {
		if r.ID == 0 || len(r.SHA256) != 64 || r.Path == "" {
			return nil, fmt.Errorf("invalid equipment source")
		}
		c.index[r.ID] = r
	}
	// Project the drop pool once. Basic is the same acceptance test the award
	// path uses, so nothing enters the pool that could later fail to be
	// granted; gear it rejects stays undroppable rather than being guessed at.
	for _, r := range c.Rows {
		durability, err := c.Basic(r.ID)
		if err != nil {
			continue
		}
		grade, rarity := r.Fields["[grade]"], r.Fields["[rarity]"]
		if len(grade) != 1 || grade[0].Type != 0 || grade[0].Value <= 0 {
			continue
		}
		if len(rarity) != 1 || rarity[0].Type != 0 || rarity[0].Value < 0 {
			continue
		}
		c.pool = append(c.pool, EquipmentDrop{ID: r.ID, Grade: grade[0].Value, Rarity: rarity[0].Value, Durability: durability})
	}
	sort.Slice(c.pool, func(i, j int) bool {
		if c.pool[i].Grade != c.pool[j].Grade {
			return c.pool[i].Grade < c.pool[j].Grade
		}
		return c.pool[i].ID < c.pool[j].ID
	})
	return &c, nil
}

type BagEquipment struct {
	// Future/private instance fields survive moves and save upgrades.
	ExtraFields map[string]json.RawMessage `json:"-"`
	// CloneSource references the physical look instance in the avatar bag.
	// Missing in old saves; old dual-Worn rows are migrated transactionally.
	CloneSource   *CloneAvatarSource `json:"clone_source,omitempty"`
	Slot          uint16             `json:"slot"`
	Template      uint32             `json:"template"`
	Group         byte               `json:"group,omitempty"`
	Durability    uint16             `json:"durability"`
	Record        []byte             `json:"record,omitempty"`
	AvatarOptions []byte             `json:"avatar_options,omitempty"`
	AvatarSockets []byte             `json:"avatar_sockets,omitempty"`
	Period        uint32             `json:"period,omitempty"`
	// Refine 是锻造（Refine / CMD430）等级，服务端权威状态。
	//
	// 为什么要单独存一格而不是只读行内字节：装备行里锻造等级那一格（configs/refine.json
	// 的 record_offset）还没实机校准到位，等级必须以服务端状态为准，否则一旦改偏移，
	// 玩家已经锻出来的等级就会读成 0（或者读到上一次写歪的残留值）。
	// 行内那格只是给客户端渲染用的镜像，不影响判定。
	Refine byte `json:"refine,omitempty"`
	// TutorialLocked 是**来源标记**：教学模式（Starter Boost 训练轨道）内新建的装备行
	// 盖上它，出关后仍然存在，只用来配合「当前是否仍在训练轨道」做门禁。
	// 老存档没有该字段 → false，向前兼容。分解刻意不受它约束（第九关任务就是拆训练装备）。
	TutorialLocked bool `json:"tutorial_locked,omitempty"`
}

// IsCloneAvatar reports whether this equipment has PVF category "clear avatar".
func (d EquipmentDefinition) IsCloneAvatar() bool {
	for _, t := range d.Fields["[item category]"] {
		if t.Text == "clear avatar" {
			return true
		}
	}
	return false
}

// IsAvatar reports whether this equipment is an avatar piece (slot 0..11).
func (d EquipmentDefinition) IsAvatar() bool {
	kind := d.Fields["[equipment type]"]
	return len(kind) > 0 && kind[0].Type == 6 && strings.HasSuffix(kind[0].Text, " avatar]")
}

// durabilityOptional 列出**源文件本来就不带 [durability] 段**的部位。
//
// 依据：对 `cmd/equipcatalog` 从客户端内层 PVF 全量生成的装备目录（67,874 行）逐部位统计，
// 下列 17 个 cell 的条目 **100% 没有 [durability]**，括号内是实测条数：
//
//	[support]3354 [magic stone]2748 [ring]2512 [wrist]2445 [amulet]2387
//	[title name]2134 [creature]1775 [talisman]1561 [earring]1387
//	[amalgamation stone]750 [artifact red]129 [oath]113 [artifact blue]112
//	[primer]76 [artifact green]46 [charm]41 [support weapon]26
//
// 而武器/防具是带耐久的，它们里少数缺耐久的条目（实测：weapon 31/24981、
// coat 22/4508、waist 15/4083、pants 7/4355、shoulder 5/4091、shoes 5/4253，
// 合计 85 行）属于源数据不规整，**仍然按缺失拒绝**。
// [flag] 带耐久（7 行都有），因此不在本表里。
//
// 原来这里只硬编码了 [amulet]/[wrist]/[ring] 三个，导致上述 17 个部位里另外 14 个
// 共 **14,337 条**（21,681 缺耐久行 - 三个首饰部位 7,344 行）装备发不出去，
// 报 "missing source equipment durability"，玩家侧也穿不上 ——
// 称号、宠物、护石、辅助装备、魔法石、耳环等整类受害。
var durabilityOptional = map[string]bool{
	"[amulet]": true, "[wrist]": true, "[ring]": true,
	"[earring]": true, "[support]": true, "[magic stone]": true, "[support weapon]": true,
	"[title name]": true, "[talisman]": true, "[creature]": true,
	"[amalgamation stone]": true, "[oath]": true, "[primer]": true, "[charm]": true,
	"[artifact red]": true, "[artifact blue]": true, "[artifact green]": true,
}

// poolJewelry 是"没有耐久但原本就能进掉落池"的三个首饰部位。
// 2026-09-18 放宽耐久规则之前，Reward 只给这三个部位免了耐久，所以它们本来就在池子里。
var poolJewelry = map[string]bool{"[amulet]": true, "[wrist]": true, "[ring]": true}

// Reward accepts the gear a quest or an operator hands out. It keeps every
// structural check Basic makes - a readable rarity, a known equipment kind, a
// usable durability - but not Basic's two drop-pool rules: a reward may be
// account- or character-bound, and it may be rarer than rare. Live capture
// 20260912T011904 shows quest 21650 refused four times as "special equipment
// reward requires additional source state" after its template was imported,
// because 100261068 is bound gear; the reward is ordinary, it simply is not
// pool gear.
//
// 2026-09-18 放宽两处（发放路径）：
//  1. **不再要求恰好一个 [attach type]**：源里有 817 件装备（如 100051124 上衣）
//     的 .equ 里根本没有 [attach type] 段；发放行（EquipmentRow）并不使用该字段，
//     绑定与否只影响"能不能掉"，所以发放不再据此拒绝（掉落侧见 Basic）。
//  2. **[durability] 缺失只对 durabilityOptional 里的部位放行**（见上表）。
func (c *EquipmentCatalog) Reward(id uint32) (uint16, error) {
	r, err := c.definitionResolved(id, 0)
	if err != nil {
		return 0, err
	}
	rarity, kind := r.Fields["[rarity]"], r.Fields["[equipment type]"]
	if len(rarity) != 1 || rarity[0].Type != 0 || rarity[0].Value < 0 || len(kind) == 0 {
		return 0, fmt.Errorf("special equipment reward requires additional source state")
	}
	d := r.Fields["[durability]"]
	if len(d) == 0 {
		if durabilityOptional[kind[0].Text] {
			return 0, nil
		}
		return 0, fmt.Errorf("missing source equipment durability")
	}
	if len(d) != 1 || d[0].Type != 0 || d[0].Value < 0 || d[0].Value > 65535 {
		return 0, fmt.Errorf("invalid equipment durability")
	}
	return uint16(d[0].Value), nil
}

// RewardType resolves the numeric PVF equipment type through the same import
// chain as Reward. CMD27's client reader uses this value to decide whether
// an extra u32 follows the 181-byte equipment record.
func (c *EquipmentCatalog) RewardType(id uint32) (int32, error) {
	r, err := c.definitionResolved(id, 0)
	if err != nil {
		return 0, err
	}
	kind := r.Fields["[equipment type]"]
	if len(kind) < 2 || kind[0].Type != 6 || kind[1].Type != 0 || kind[1].Value < 0 {
		return 0, fmt.Errorf("numeric equipment type missing for %d", id)
	}
	return kind[1].Value, nil
}

func (c *EquipmentCatalog) EquipmentKind(id uint32) (string, error) {
	r, err := c.definitionResolved(id, 0)
	if err != nil {
		return "", err
	}
	kind := r.Fields["[equipment type]"]
	if len(kind) == 0 || kind[0].Type != 6 {
		return "", fmt.Errorf("equipment type missing for %d", id)
	}
	return kind[0].Text, nil
}

// definitionResolved 跟随 [import script] 链补全"薄壳"装备。
//
// 真源里大量装备只是引用另一件的基础定义 —— equipment/character/common/jacket/cloth/
// 100050791.equ 的内容只有 [name]/[attach type]/[usable period]/[value]/[move wav]/
// [import script] `character/common/jacket/cloth/100050666.equ`，自身不带
// [rarity]/[equipment type]/[durability]。不跟随这条链，发放就会以
// "special equipment reward requires additional source state" 或
// "missing source equipment durability" 失败；实机诊断
// （cmd/wireprobe/selection_box_audit_test.go）显示 78 个自选盒 / 543 件装备因此发不出去。
//
// 自身显式给出的字段优先；基础取不到（目标不在目录里、或链太深）时退化为自身，
// 保持原有报错行为而不是悄悄放行。
func (c *EquipmentCatalog) definitionResolved(id uint32, depth int) (EquipmentDefinition, error) {
	d, err := c.Definition(id)
	if err != nil {
		return d, err
	}
	if depth >= 8 {
		return d, fmt.Errorf("equipment import chain too deep at %d", id)
	}
	target := importTarget(d.Fields["[import script]"])
	if target == 0 || target == id {
		return d, nil
	}
	base, err := c.definitionResolved(target, depth+1)
	if err != nil {
		return d, nil
	}
	// 补齐的字段只增不减，读它们的调用方各取所需：Reward / EquipmentKind 只读前三个，
	// 武器幻化的可用性判定要读后面的等级与职业段。薄壳武器自身不带这些段，不追这条链
	// 就会把一件本职业明明能用的武器判成"非本职业可用"而拒绝幻化。
	for _, key := range []string{
		"[rarity]", "[equipment type]", "[durability]",
		"[minimum level]", correctionEquippedLevelKey, "[usable job]", "[usable grow type]",
		"[required job skill]",
	} {
		if len(d.Fields[key]) > 0 || len(base.Fields[key]) == 0 {
			continue
		}
		if d.Fields == nil {
			d.Fields = map[string][]pvf.Token{}
		}
		d.Fields[key] = base.Fields[key]
	}
	return d, nil
}

// importTarget 从 [import script] 的路径里取出目标模板号（文件名就是模板 id）。
func importTarget(cells []pvf.Token) uint32 {
	for _, t := range cells {
		if t.Type != 6 {
			continue
		}
		name := t.Text
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		name = strings.TrimSuffix(name, ".equ")
		if n, err := strconv.ParseUint(name, 10, 32); err == nil {
			return uint32(n)
		}
	}
	return 0
}

// Basic is Reward plus the rules that decide what a monster may drop: the
// piece has to be unbound, and no rarer than the source rarity roll can name.
// The drop pool is built from this, so anything a drop offers is also
// grantable.
//
// 注意：这里**比 Reward 更严**，两处：
//  1. 必须恰好有一个 [attach type] 且为 [free]（没有绑定信息的条目不能确定可自由掉落）；
//  2. 掉落池只收"带耐久的部位"与戒指/手镯/项链（poolJewelry）。放宽耐久规则后
//     新变得可发放的称号/辅助装备/魔法石/耳环/护石/宠物/融合石/… **不进入掉落池**，
//     以免改变既有掉落分布。它们仍然可以被 GM 与任务正常发放。
//
// 稀有度上限 2026-09-23 从 1 放宽到 2，理由（实测，不是推测）：
//   - [basis of rarity dicision] 首行给 rarity 0/1/2 的概率是 70%/26.5%/2.5%，
//     第 3 档及以上恒为 0 ⇒ 源表本来就预期掉到 rarity 2；
//   - 装备目录里 rarity=0 的 744 件**全在 grade<=20**，grade>=22 的 2430 件
//     rarity 全 >=1，而 rarity=2 占其中绝大多数。原来的 `> 1` 把这些整批挡在
//     池外，掉落池只剩 grade<=20 的 1534 件；
//   - 后果：等级 >=22 的副本**一件装备都掉不出来** —— 实机日志里
//     `drop_rules_pending / equipment_grade_window_empty` 就是这条路径。
//
// 发放走的仍是 Reward（见 Bag.AddEquipment），放宽此处不影响任务/GM 发放。
func (c *EquipmentCatalog) Basic(id uint32) (uint16, error) {
	d, e := c.Reward(id)
	if e != nil {
		return 0, e
	}
	r, e := c.Definition(id)
	if e != nil {
		return 0, e
	}
	attach, rarity, kind := r.Fields["[attach type]"], r.Fields["[rarity]"], r.Fields["[equipment type]"]
	if len(attach) != 1 || !acceptableAttach(attach[0].Text) || rarity[0].Value > 2 {
		return 0, fmt.Errorf("special equipment reward requires additional source state")
	}
	if len(r.Fields["[durability]"]) == 0 && !poolJewelryFor(kind[0].Text) {
		return 0, fmt.Errorf("special equipment reward requires additional source state")
	}
	return d, nil
}

// allowTradeEquipment 报告是否把 `[trade]` / `[trade delete]` 也算作可掉落。
//
// 默认**关**：交易属性是否还需要额外的绑定状态尚未证实（见 Basic 的原始注释
// "requires additional source state"），所以做成可一键切换的实验开关。
func allowTradeEquipment() bool {
	return os.Getenv("DFO_ALLOW_TRADE_EQUIPMENT") == "1"
}

// acceptableAttach 决定一件装备的 `[attach type]` 能否进掉落池。
//
// 原实现只接受 `[free]`，把 `[trade]` / `[trade delete]` 一并拒绝。后果（外部包实测，
// 2026-09-28）：115 深渊的掉落装备（`100345985` `[support]`、`100391038` `[earring]`、
// `100354160` `[magic stone]`、`100401592` `[primer]`）**全部是 `[trade]`**，于是
//
//	Basic() 拒绝 → 不进 pool → Durability() 查不到 → 掉落记录 [11] Durability = 0
//	→ 客户端拿不到耐久/扩展字段 → 粉以上装备落地没有闪光特效。
//
// 三类在我们自己的 `configs/equipment.current37.json` 里的分布是
// `[free]` 2805 / `[trade]` 170 / `[trade delete]` 199（与作者侧逐项一致），
// 都来自同一份 PVF，本来就该能发。
func acceptableAttach(t string) bool {
	switch t {
	case "[free]":
		return true
	case "[trade]", "[trade delete]":
		return allowTradeEquipment()
	}
	return false
}

// poolJewelryFor 决定「没有 [durability]」的部位能否进掉落池。
//
// 原实现硬查 `poolJewelry`（只有 `[amulet]` `[wrist]` `[ring]` 三个），而 `Reward` 那侧的
// `durabilityOptional` 早已放宽到 17 个部位。两侧不同步的直接后果：深渊装备的部位
// （`[support]` / `[earring]` / `[magic stone]` / `[primer]`）虽然 `Reward` 认，`Basic` 却拒
// ⇒ 永远进不了掉落池。开关打开时改用 `durabilityOptional` 与发放路径对齐；
// 默认仍走旧表，行为与原先逐字节一致。
func poolJewelryFor(kind string) bool {
	if allowTradeEquipment() {
		return durabilityOptional[kind]
	}
	return poolJewelry[kind]
}

func EquipmentRow(i BagEquipment) [protocol.CurrentItemRecordSize]byte {
	// Current NOTI13 logs name slot+0, template+2, Data+6, ext_data1+10,
	// Durability+11, isSealed+13. Fresh free basic gear has zero extensions.
	r := protocol.OrdinaryItem(i.Slot, i.Template, 0)
	if len(i.Record) == protocol.CurrentItemRecordSize {
		copy(r[:], i.Record)
	}
	binary.LittleEndian.PutUint16(r[:], i.Slot)
	binary.LittleEndian.PutUint32(r[2:], i.Template)
	binary.LittleEndian.PutUint32(r[56:], protocol.ItemPeriodForWire(i.Template, binary.LittleEndian.Uint32(r[56:60])))
	// Worn slot 26 is the equipped creature. Its Data field is the
	// creature instance key consumed by both NOTI13/14 and NOTI105.
	if i.Slot == 26 && i.Template != 0 && binary.LittleEndian.Uint32(r[6:10]) == 0 {
		binary.LittleEndian.PutUint32(r[6:10], 1)
	}
	binary.LittleEndian.PutUint16(r[11:], i.Durability)
	return r
}
func (b Bag) AddEquipment(c *EquipmentCatalog, slots [2]uint16, id, count uint32) (Bag, []uint16, error) {
	if c == nil || slots[0] == 0 || slots[0] > slots[1] || count == 0 || count > uint32(slots[1]-slots[0]+1) {
		return b, nil, fmt.Errorf("invalid equipment award")
	}
	// Granting uses the reward rule, not the drop-pool rule: a quest reward is
	// routinely bound and may be above rare. Drops are unaffected, because the
	// pool they choose from is built from Basic.
	d, e := c.Reward(id)
	if e != nil {
		return b, nil, e
	}
	occupied := map[uint16]bool{}
	for _, i := range b.Items {
		occupied[i.Slot] = true
	}
	for _, i := range b.Equipment {
		occupied[i.Slot] = true
	}
	var available []uint16
	for n := uint32(slots[0]); n <= uint32(slots[1]) && len(available) < int(count); n++ {
		if !occupied[uint16(n)] {
			available = append(available, uint16(n))
		}
	}
	if len(available) != int(count) {
		return b, nil, fmt.Errorf("equipment bag is full")
	}
	b.Equipment = append([]BagEquipment(nil), b.Equipment...)
	for _, n := range available {
		b.Equipment = append(b.Equipment, BagEquipment{Slot: n, Template: id, Durability: d, TutorialLocked: b.tutorialActive})
	}
	return b, available, nil
}
