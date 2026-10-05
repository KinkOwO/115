package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

type BagRules struct {
	Model             string               `json:"model"`
	Source            string               `json:"source"`
	Slots             map[string][2]uint16 `json:"slots"`
	MissingStackLimit uint32               `json:"missing_stack_limit"`
	EquipmentSlots    [2]uint16            `json:"equipment_slots,omitempty"`
	// QuickSlots is the belt the client drags consumables onto, below the
	// equipment range. Live capture 20260912T004320 shows CMD19 asking to put
	// the 6003 stack from slot 65 into slot 3; every earlier build refused it
	// because list 0 modelled nothing but the equipment range, and the client
	// then showed its generic "target inventory is full" notice. Slot 0 is a
	// legal belt slot, so this range is not subject to the nonzero check the
	// type ranges use.
	QuickSlots [2]uint16 `json:"quick_slots,omitempty"`
}

// Quick reports whether a slot is part of the quick-use belt.
func (r BagRules) Quick(slot uint16) bool {
	return r.QuickSlots != [2]uint16{} && slot >= r.QuickSlots[0] && slot <= r.QuickSlots[1]
}

// LoadBagRules 读取背包槽位策略。
//
// 可选 source 参数（2026-10-01，next146）：直读模式下调用方传入当次内层 checksum。
//
// 与 LoadWearRules 的差别，以及为什么这里用**覆盖**而不是拒绝：背包规则表的
// `source` 不是"某个 PVF 的实时指纹"，而是**整批 configs/*.json 导出族的批次标记** ——
// loot.next25 / inventory.next29 / compat90 / equipment.current37 等全部写着同一个
// 历史值，`Bag.Disjoint` 只拿它断言"规则表和掉落目录来自同一批导出"。
// 直读模式下服务端是**现场从内层 PVF 推导**目录的，调用方传进来的 checksum 就是权威身份，
// 文件里那个历史批次值必须被它取代；若在这里硬拒，内层一重建（哈希必变）启动就又断了。
//
// 覆盖/回填是**必需的**：BagRules.Source 同时是运行时不变量（amplify / enchant /
// inherit / refine / reinforcement / vault_transfer / stack_request / pet_move 都拿它比
// role.ConfigVersion，main.go 也拿它比掉落目录的 checksum），只在文件里留空而不回填
// 会让这些操作在运行时全被拒。
//
// 不传 source（cmd/admin、cmd/gmtool、cmd/charactercheck 等旧调用方）→ 保持旧契约：
// 文件里必须写死 64 位哈希。
func LoadBagRules(p string, source ...string) (BagRules, error) {
	var r BagRules
	b, e := os.ReadFile(p)
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return r, e
	}
	derived := ""
	if len(source) > 0 {
		derived = source[0]
	}
	if r.Model != "reference90-bag-v1" || r.MissingStackLimit == 0 {
		return r, fmt.Errorf("invalid bag policy")
	}
	if derived != "" {
		// 直读模式：调用方的实时 checksum 权威，覆盖文件里的历史批次标记（含留空）。
		if len(derived) != 64 {
			return r, fmt.Errorf("bag rules source mismatch")
		}
		r.Source = derived
	} else if len(r.Source) != 64 {
		// 旧调用方：文件必须自带 64 位批次标记。
		return r, fmt.Errorf("invalid bag policy")
	}
	seen := map[uint16]bool{}
	if r.QuickSlots != [2]uint16{} {
		v := r.QuickSlots
		if v[0] > v[1] || v[1] > 65534 {
			return r, fmt.Errorf("invalid quick slot range")
		}
		if r.EquipmentSlots != [2]uint16{} && v[1] >= r.EquipmentSlots[0] {
			return r, fmt.Errorf("quick slots overlap the equipment bag")
		}
		for n := uint32(v[0]); n <= uint32(v[1]); n++ {
			seen[uint16(n)] = true
		}
	}
	if r.EquipmentSlots != [2]uint16{} {
		v := r.EquipmentSlots
		if v[0] == 0 || v[0] > v[1] || v[1] > 65534 {
			return r, fmt.Errorf("invalid equipment bag range")
		}
		for n := uint32(v[0]); n <= uint32(v[1]); n++ {
			seen[uint16(n)] = true
		}
	}
	for _, v := range r.Slots {
		if v[0] == 0 || v[0] > v[1] || v[1] > 65534 {
			return r, fmt.Errorf("invalid bag slot range")
		}
		for n := uint32(v[0]); n <= uint32(v[1]); n++ {
			if seen[uint16(n)] {
				return r, fmt.Errorf("overlapping bag ranges")
			}
			seen[uint16(n)] = true
		}
	}
	return r, nil
}

type BagItem struct {
	Slot             uint16 `json:"slot"`
	Template, Amount uint32
	ExpireTime       uint32 `json:"expire_time,omitempty"`
}
type Bag struct {
	// Missing in legacy saves; fences atomic emblem insertion/replacement.
	EmblemInlaySeq uint64 `json:"emblem_inlay_seq,omitempty"`

	// Old inventories omit the sequence and start at zero. It distinguishes
	// repeated compounds using stacks that keep the same slots and templates.
	EmblemCompoundSeq uint64 `json:"emblem_compound_seq,omitempty"`

	// Legacy inventories omit this field and keep their original avatar capacity.
	AvatarExpansion byte                    `json:"avatar_expansion,omitempty"`
	Expansion       byte                    `json:"expansion,omitempty"`
	Version         string                  `json:"version"`
	Gold            uint32                  `json:"gold"`
	Coin            uint32                  `json:"coin,omitempty"`
	Items           []BagItem               `json:"items"`
	PetItems        []BagItem               `json:"pet_items,omitempty"`
	Equipment       []BagEquipment          `json:"equipment,omitempty"`
	Worn            []BagEquipment          `json:"worn,omitempty"`
	Special         map[byte][]BagEquipment `json:"special_equipment,omitempty"`
	// Missing in legacy saves. Socket zero is projected from Worn slot 24;
	// the other four entries are the client's Reserved shield positions.
	KnightShieldDeck []uint32 `json:"knight_shield_deck,omitempty"`
	// CreatureExperience is keyed by the creature instance key stored in its
	// equipment record. Older saves omit it and start at zero experience.
	CreatureExperience map[uint32]uint32 `json:"creature_experience,omitempty"`
	// Missing values in old saves mean a fully fed creature.
	CreatureSatiety map[uint32]byte `json:"creature_satiety,omitempty"`
	// Loyalty accrues in 1/360 point units: town/unequipped +1 per second,
	// equipped in a dungeon -6 per second. Old saves start at login time.
	CreatureLoyaltyUpdatedAt  int64            `json:"creature_loyalty_updated_at,omitempty"`
	CreatureLoyaltyDungeonKey uint32           `json:"creature_loyalty_dungeon_key,omitempty"`
	CreatureLoyaltyFraction   map[uint32]int64 `json:"creature_loyalty_fraction,omitempty"`
	// ExpandEquipFlags carries the extended equipment-slot unlock bits the
	// armoury draws its padlocks from: support 1<<0, magic stone 1<<1 and
	// earring 1<<4. Quests 649/650/2636 award one bit each and every award
	// must accumulate, so this field is only ever OR-ed - assigning it would
	// relock whatever an earlier quest opened. The client reads the byte from
	// the USERINFO1 unlock slot (protocol entry_addition, native 14563d692)
	// and gates equipment slots 22/23/25 on bits 1/2/16. Absent in saves
	// written before 2026-09-22, which reads back as 0 (nothing unlocked).
	ExpandEquipFlags byte `json:"expand_equip_flags,omitempty"`
	// WeaponSkin 是幻化仓库（Skin Storage）里应用的武器外观：皮肤 id，实机就是被
	// 幻化那把武器的模板。0 表示没有应用，装备外观投影照旧使用穿戴中的武器模板，
	// 因此缺席（老存档）与 0 同义。
	//
	// 它只改外观：穿戴容器（list 3）里的武器对象、金库与背包里的物品都不动，
	// 所以脱装/换装的身份校验（stale equipment identity）不受影响。
	WeaponSkin uint32 `json:"weapon_skin,omitempty"`
	// WeaponSkins 是幻化仓库武器外观页签（NOTI1545 subtype 4）里已复制的皮肤 id，
	// 按复制顺序排列。
	//
	// 客户端把这个容器当会话态：NOTI1545 只在复制（CMD1592）时推过一次，重登后
	// 容器是空的，所以列表必须落库，入场时再按它重推一次。WeaponSkin（已应用的那
	// 一个）正常是它的成员；老存档没有本字段，见 WeaponSkinStorage 的兜底。
	WeaponSkins []uint32 `json:"weapon_skins,omitempty"`
	// WeaponSkinSeq 是幻化应用/解除的单调序号，每真正改一次外观加一。
	//
	// 它只服务于幂等键：CMD1565 的 Apply 与「浏览选中」是同一个帧，服务端按
	// 「角色 + 上一个皮肤 + 新皮肤」落事务，于是 A→B→A 的第三次和第一次键完全相同，
	// CommitCharacterEvent 会当成重放命中旧 receipt、闭包根本不执行，
	// 玩家看到的就是「替换不生效、状态停在 B」（实机 2026-09-27）。
	// 把序号一起编进键，每次真实切换都是新键；序号本身不参与任何投影。
	WeaponSkinSeq uint32 `json:"weapon_skin_seq,omitempty"`
	// AvatarDisjointSeq separates successive instances of the same avatar in
	// the same slot. Old saves begin at zero; it is not sent to the client.
	AvatarDisjointSeq uint64 `json:"avatar_disjoint_seq,omitempty"`
	// Old saves start at zero. Successive opening operations get distinct receipts.
	AvatarSocketSeq uint64 `json:"avatar_socket_seq,omitempty"`
	AvatarRecastSeq uint64 `json:"avatar_recast_seq,omitempty"`

	// tutorialActive 由 ReadBag 从角色 state 现算（不落库、不序列化）：当前角色是否还在
	// Starter Boost 训练轨道里。封存门禁 = 行的来源标记 TutorialLocked && 本字段为真，
	// 所以出关后同一件装备自动恢复出售/丢弃/寄件/入仓。
	tutorialActive bool
}

// inBoostTraining 是训练轨道探测谓词，由装配层（cmd/wireprobe/main.go）注入，
// 避免 inventory 反向依赖 boostup 包。nil = 活动关闭，一切照旧。
var inBoostTraining func(state json.RawMessage) bool

func SetInBoostTraining(fn func(json.RawMessage) bool) { inBoostTraining = fn }

// TutorialSealed 判定行是否处于封存中：来源在训练期内 **且** 当前仍在训练轨道。
// 包内三处门禁（出售/寄件/入仓）与装配层（CMD18 丢弃、开箱直落）共用这一个口径。
func (b Bag) TutorialSealed(row BagEquipment) bool {
	return row.TutorialLocked && b.tutorialActive
}

// TutorialMode 报告该背包读出时角色是否仍在训练轨道（给不经 AddEquipment 的建装路径盖同一个锁）。
func (b Bag) TutorialMode() bool { return b.tutorialActive }

// WeaponSlot 是穿戴容器（list 3）里的武器槽。[equipment type] 的序号空间里
// weapon = 12，装备外观块（0x145639840）与主副手互换都用同一个槽号。
const WeaponSlot uint16 = 12

// WeaponSkinStorage 给出幻化仓库武器页签要显示的皮肤 id 列表。
//
// 已应用的 WeaponSkin 一定也在里面（它本来就是从这个仓库里选出来应用的），所以
// 即使列表里没有它也要补进去 —— 老存档更是只有这一个可用。重复 id 只保留第一次：
// 客户端仓库行的身份就是皮肤 id，重复行会让选择/应用指到同一格。
func (b Bag) WeaponSkinStorage() []uint32 {
	seen := make(map[uint32]bool, len(b.WeaponSkins)+1)
	out := make([]uint32, 0, len(b.WeaponSkins)+1)
	for _, id := range b.WeaponSkins {
		if id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if b.WeaponSkin != 0 && !seen[b.WeaponSkin] {
		out = append(out, b.WeaponSkin)
	}
	return out
}

func ReadBag(state json.RawMessage) (Bag, error) {
	var fields map[string]json.RawMessage
	var b Bag
	if e := json.Unmarshal(state, &fields); e != nil {
		return b, e
	}
	if fields == nil {
		return b, fmt.Errorf("character state is not an object")
	}
	if inBoostTraining != nil {
		b.tutorialActive = inBoostTraining(state)
	}
	if p, ok := fields["inventory"]; ok {
		if e := json.Unmarshal(p, &b); e != nil {
			return b, e
		}
		if b.Version != "ordinary-bag-v1" {
			return b, fmt.Errorf("unsupported saved inventory")
		}
	} else {
		b.Version = "ordinary-bag-v1"
	}
	filtered := make([]BagItem, 0, len(b.Items))
	if b.Expansion > 2 {
		return b, fmt.Errorf("背包扩展档位超出客户端范围")
	}
	if b.AvatarExpansion > protocol.MaxAvatarInventoryExpansion {
		return b, fmt.Errorf("时装栏扩展档位超出客户端范围")
	}
	for _, i := range b.Items {
		if i.Template == 1 {
			if uint64(b.Coin)+uint64(i.Amount) <= math.MaxUint32 {
				b.Coin += i.Amount
			}
			continue
		}
		filtered = append(filtered, i)
	}
	b.Items = filtered
	petSeen := map[uint16]bool{}
	for _, i := range b.PetItems {
		if i.Slot < 376 || i.Slot > 431 || i.Template < 2 || i.Amount == 0 || petSeen[i.Slot] {
			return b, fmt.Errorf("invalid saved pet consumable")
		}
		petSeen[i.Slot] = true
	}
	seen := map[uint16]bool{}
	for _, i := range b.Items {
		if i.Slot == 0 || i.Slot == 1 || i.Template == 0 || i.Amount == 0 || seen[i.Slot] {
			return b, fmt.Errorf("invalid saved inventory row")
		}
		seen[i.Slot] = true
	}
	for _, i := range b.Equipment {
		if e := i.ValidateRecord(); e != nil {
			return b, e
		}
		if i.Slot == 0 || i.Template == 0 || seen[i.Slot] {
			return b, fmt.Errorf("invalid saved equipment")
		}
		seen[i.Slot] = true
	}
	wornSeen := map[uint32]bool{}
	for idx, i := range b.Worn {
		if e := i.ValidateRecord(); e != nil {
			return b, e
		}
		if !EquipmentBodySlot(i.Slot) || i.Template == 0 {
			return b, fmt.Errorf("invalid saved worn equipment")
		}
		if i.Group > 1 || (i.Group == 1 && i.Slot > 11) {
			return b, fmt.Errorf("invalid worn equipment group")
		}
		key := (uint32(i.Group) << 16) | uint32(i.Slot)
		if wornSeen[key] {
			return b, fmt.Errorf("invalid saved worn equipment")
		}
		wornSeen[key] = true
		if i.Slot == 26 {
			if hatched, ok := EggHatchOutputs[i.Template]; ok {
				b.Worn[idx].Template = hatched
			}
		}
	}
	for space, rows := range b.Special {
		if space != 1 && space != 7 {
			return b, fmt.Errorf("unsupported equipment inventory")
		}
		seen = map[uint16]bool{}
		for _, i := range rows {
			if e := i.ValidateRecord(); e != nil {
				return b, e
			}
			if i.Template == 0 || seen[i.Slot] || (space == 7 && petSeen[i.Slot]) {
				return b, fmt.Errorf("invalid special equipment")
			}
			seen[i.Slot] = true
		}
	}
	return b, nil
}

// durabilityLimit 由启动期注入：给定模板返回源 `.equ` 里的 `[durability]` 上限。
// nil 时不做任何修正（行为与修复前一致）。
var durabilityLimit func(template uint32) (uint16, bool)

// SetDurabilityLimit 安装"装备耐久上限"查询源（cmd/wireprobe 装载完装备目录后调用）。
//
// 为什么需要它：每件装备的**耐久上限**写在源 `.equ` 的 `[durability]`（例如 11701025x
// 武器是 48），而耐久是**服务端权威**的实例字段。一旦有别的写入方把它写成超过上限的值
// （实机 2026-09-30：存档里出现 `100/48`），客户端会判定这件装备**非法** —— 表现正是
// "不能在装备库里登记 / 分解点不动 / 装备变换界面卡死"。落库前统一 clamp 是最省事也最
// 彻底的堵口：无论耐久从哪来（GM 工具、外部脚本、旧数据），写进存档时都合法。
func SetDurabilityLimit(fn func(template uint32) (uint16, bool)) { durabilityLimit = fn }

// clampDurability 把超出源上限的耐久压回上限。上限查不到、或为 0（该部位本来就无耐久
// 上限，如首饰/称号）时**不动**，避免把这类部位误改成 0。
func clampDurability(items []BagEquipment) {
	if durabilityLimit == nil {
		return
	}
	for i := range items {
		if items[i].Template == 0 {
			continue
		}
		if max, ok := durabilityLimit(items[i].Template); ok && max > 0 && items[i].Durability > max {
			items[i].Durability = max
		}
	}
}

func SaveBag(state json.RawMessage, b Bag) (json.RawMessage, error) {
	if b.AvatarExpansion > protocol.MaxAvatarInventoryExpansion {
		return nil, fmt.Errorf("时装栏扩展档位超出客户端范围")
	}
	if b.Expansion > 2 {
		return nil, fmt.Errorf("背包扩展档位超出客户端范围")
	}

	// [ALIGN-20260930-DURABILITY] 落库前 clamp 耐久（见 SetDurabilityLimit 的说明）。
	// 先拷贝切片，避免就地改到调用方那份 Bag。
	b.Worn = append([]BagEquipment(nil), b.Worn...)
	b.Equipment = append([]BagEquipment(nil), b.Equipment...)
	if len(b.Special) > 0 {
		m := make(map[byte][]BagEquipment, len(b.Special))
		for space, rows := range b.Special {
			m[space] = append([]BagEquipment(nil), rows...)
		}
		b.Special = m
	}
	clampDurability(b.Worn)
	clampDurability(b.Equipment)
	for _, rows := range b.Special {
		clampDurability(rows)
	}
	if len(b.KnightShieldDeck) > 0 {
		if len(b.KnightShieldDeck) > protocol.KnightDeckSize {
			return nil, fmt.Errorf("saved knight deck exceeds five slots")
		}
		deck := b.KnightDeck()
		b.KnightShieldDeck = append([]uint32(nil), deck[:]...)
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(state, &fields); e != nil {
		return nil, e
	}
	if fields == nil {
		return nil, fmt.Errorf("character state is not an object")
	}
	v, e := json.Marshal(b)
	if e != nil {
		return nil, e
	}
	fields["inventory"] = v
	return json.Marshal(fields)
}
func (b Bag) Rows() [][protocol.CurrentItemRecordSize]byte {
	items := append([]BagItem(nil), b.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].Slot < items[j].Slot })
	rows := [][protocol.CurrentItemRecordSize]byte{
		protocol.OrdinaryItem(0, 0, b.Gold),
		protocol.OrdinaryItem(1, 1, b.Coin),
	}
	for _, i := range items {
		rows = append(rows, protocol.OrdinaryItem(i.Slot, i.Template, i.Amount, i.ExpireTime))
	}
	for _, i := range b.Equipment {
		rows = append(rows, EquipmentRow(i))
	}
	sort.Slice(rows, func(i, j int) bool {
		return uint16(rows[i][0])|uint16(rows[i][1])<<8 < uint16(rows[j][0])|uint16(rows[j][1])<<8
	})
	return rows
}

// RowAt returns the protocol row for one slot, matching the Rows() layout for
// that slot. It lets callers publish an incremental NOTI14 update so the
// client only marks the changed slot as newly obtained instead of flashing
// the whole bag's new-item highlight on every pickup.
func (b Bag) RowAt(slot uint16) ([protocol.CurrentItemRecordSize]byte, bool) {
	switch slot {
	case 0:
		return protocol.OrdinaryItem(0, 0, b.Gold), true
	case 1:
		return protocol.OrdinaryItem(1, 1, b.Coin), true
	}
	for _, i := range b.Items {
		if i.Slot == slot {
			return protocol.OrdinaryItem(i.Slot, i.Template, i.Amount, i.ExpireTime), true
		}
	}
	for _, e := range b.Equipment {
		if e.Slot == slot {
			return EquipmentRow(e), true
		}
	}
	return [protocol.CurrentItemRecordSize]byte{}, false
}

// Add updates the whole bag in the caller's character transaction. It does
// not silently spill, drop or partially grant a stack when the bag is full.
func (b Bag) Add(c catalog.LootCatalog, r BagRules, id, amount uint32, expireTime ...uint32) (Bag, uint16, error) {
	if amount == 0 {
		return b, 0, fmt.Errorf("invalid inventory award/source")
	}
	var exp uint32
	if len(expireTime) > 0 {
		exp = expireTime[0]
	}
	b.Items = append([]BagItem(nil), b.Items...)
	if id == 0 {
		if uint64(b.Gold)+uint64(amount) > math.MaxUint32 {
			return b, 0, fmt.Errorf("gold overflow")
		}
		b.Gold += amount
		return b, 0, nil
	}
	if id == 1 {
		if uint64(b.Coin)+uint64(amount) > math.MaxUint32 {
			return b, 0, fmt.Errorf("coin overflow")
		}
		b.Coin += amount
		return b, 1, nil
	}
	item, ok := c.Items[id]
	if !ok || item.Kind != "stackable" {
		return b, 0, fmt.Errorf("unsupported source item")
	}
	if c.HasRuntimeDetails() {
		if _, err := c.ItemScript(id); err != nil {
			return b, 0, err
		}
	}
	if IsPetConsumable(item.StackableType) {
		return b.addPetStack(r, id, amount, exp, item.StackLimit)
	}
	slots := stackableSlotRange(r, item.StackableType)
	limit := stackLimitFor(r, item.StackableType, item.StackLimit)
	if amount > limit {
		return b, 0, fmt.Errorf("award exceeds stack limit")
	}
	original := b
	occupied := map[uint16]bool{}
	for _, i := range b.Equipment {
		occupied[i.Slot] = true
	}
	for i, row := range b.Items {
		occupied[row.Slot] = true
		// 新发放的限时物品不能继承另一堆的非零期限（尤其是已过期的旧堆）。
		if exp != 0 && row.ExpireTime != 0 && row.ExpireTime != exp {
			continue
		}
		// 快捷栏上的同模板堆就是这堆本身（next79 2026-10-03）：发放必须并进
		// 它，否则段内另起一叠、客户端陷入 findItemSlot "multiple slot issues"
		// 死循环。belt 行作合并目标与其余守卫（期限、上限）完全同规则。
		if row.Template == id && (r.Quick(row.Slot) || row.Slot >= slots[0] && row.Slot <= slots[1]) && row.Amount < limit {
			added := min(amount, limit-row.Amount)
			b.Items[i].Amount += added
			amount -= added
			if exp != 0 && b.Items[i].ExpireTime == 0 {
				b.Items[i].ExpireTime = exp
			}
			if amount == 0 {
				return b, row.Slot, nil
			}
		}
	}
	for n := uint32(slots[0]); n <= uint32(slots[1]); n++ {
		slot := uint16(n)
		if !occupied[slot] {
			b.Items = append(b.Items, BagItem{Slot: slot, Template: id, Amount: amount, ExpireTime: exp})
			return b, slot, nil
		}
	}
	return original, 0, fmt.Errorf("bag category is full")
}

// SweepStackSlots relocates saved stackables with known categories. It leaves
// unknown types alone, and returns the original bag on any placement error so
// a failed migration cannot lose an item.
//
// 快捷栏行（slot<=8）不再整体豁免（2026-10-03，next79 待机区闪退）：客户端把
// 堆拖上快捷栏后，那一行就是这堆本身（实机 20260912T004320，CMD19 65→3）。
// 同模板在 belt 和类型段各有一行时，客户端 findItemSlot 陷入 "multiple slot
// issues" 死循环（032305 会话 79374 次告警，待机区场景永不完成、约 11 分钟后
// 崩）。修复方向：belt 行并入段内行后删除；belt 上唯一的堆仍受保护。
func SweepStackSlots(b Bag, c catalog.LootCatalog, r BagRules) (Bag, bool, error) {
	next := b
	next.Items = append([]BagItem(nil), b.Items...)
	moved := false
	for i := 0; i < len(next.Items); {
		row := next.Items[i]
		definition, ok := c.Items[row.Template]
		if !ok || definition.Kind != "stackable" || IsPetConsumable(definition.StackableType) {
			i++
			continue
		}
		if row.Slot <= 8 {
			// 同模板在类型段有行且能吸收时并入；期限不同的堆不并（继承
			// 另一堆的期限会改变到期语义）。段内行已满时 belt 保留剩余
			// （满堆分堆是官服合法状态）。
			slots := stackableSlotRange(r, definition.StackableType)
			limit := stackLimitFor(r, definition.StackableType, definition.StackLimit)
			target := -1
			for j, other := range next.Items {
				if other.Slot >= slots[0] && other.Slot <= slots[1] &&
					other.Template == row.Template && other.ExpireTime == row.ExpireTime &&
					other.Amount < limit {
					target = j
					break
				}
			}
			if target < 0 {
				i++
				continue
			}
			added := min(row.Amount, limit-next.Items[target].Amount)
			next.Items[target].Amount += added
			next.Items[i].Amount -= added
			if next.Items[i].Amount == 0 {
				next.Items = append(next.Items[:i], next.Items[i+1:]...)
			} else {
				i++
			}
			moved = true
			continue
		}
		slots, known := classifyStackableSlot(r, definition.StackableType)
		if !known || row.Slot >= slots[0] && row.Slot <= slots[1] {
			i++
			continue
		}
		next.Items = append(next.Items[:i], next.Items[i+1:]...)
		placed, err := next.AddMailItem(c, r, nil, MailItem{Stack: &row})
		if err != nil {
			return b, false, fmt.Errorf("relocate stackable %d from slot %d: %w", row.Template, row.Slot, err)
		}
		next = placed
		moved = true
		// The relocated row is appended in its correct range; examine the
		// remaining original row at this index next.
	}
	return next, moved, nil
}

// WornBaseItems filters b.Worn down to at most one item per slot (0..47),
// preferring Group 0 (Clone or base equipment) over Group 1 (Appearance).
// This is used for NOTI 13/14 packets which require unique slot entries.
func (b Bag) WornBaseItems() []BagEquipment {
	bySlot := make(map[uint16]BagEquipment)
	for _, item := range b.Worn {
		if item.Group == 1 {
			bySlot[item.Slot] = item
		}
	}
	for _, item := range b.Worn {
		if item.Group == 0 {
			bySlot[item.Slot] = item
		}
	}
	res := make([]BagEquipment, 0, len(bySlot))
	for _, item := range bySlot {
		res = append(res, item)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Slot < res[j].Slot })
	return res
}
