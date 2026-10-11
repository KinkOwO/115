package reward

import "context"

type EventType string

const (
	EventLevelUp         EventType = "level_up"
	EventQuestComplete   EventType = "quest_complete"
	EventCharacterCreate EventType = "character_create"
)

// Recipient identifies the character an event/reward belongs to.
type Recipient struct {
	AccountID     int64
	CharacterID   int64
	Name          string
	Level         byte
	ConfigVersion string

	// Profession / Advancement 是角色的职业信息（基础职业号 / 转职号），给 Lua 规则用
	// （`ctx.profession` / `ctx.advancement`）。
	//
	//   - 只有**知道职业**的事件才填：当前只有 character_create（那时职业与转职就在建号
	//     请求与初始 state 里）。level_up / quest_complete 一律不填 —— 与其塞一个猜的值，
	//     不如让规则自己看到"没有"并跳过。
	//   - 另用 HasProfession 而不是用 0 表示"不知道"：基础职业 0（鬼剑士）是合法值，
	//     和"拿不到"不能共用一个数（脚本里 `if not ctx.profession then return end` 依赖这一点：
	//     字段不存在时 Lua 侧是 nil，存在时哪怕 0 也是数）。
	Profession    int32
	Advancement   int32
	HasProfession bool
}

type Event struct {
	Type      EventType
	Recipient Recipient
	QuestID   uint16
}

type ItemGrant struct {
	ID    uint32
	Count uint32
}

type MailReward struct {
	Subject string
	Body    string
	// Attachments optionally deliver items/equipment with the mail. The mail
	// claim path restores each one into the bag, so a full bag leaves the
	// attachments waiting in the mailbox instead of failing the reward.
	Attachments []ItemGrant
}

// Notifier is the only interface the domains depend on. Domains call these
// after a business success; they never read reward rules themselves.
type Notifier interface {
	LevelUp(ctx context.Context, r Recipient)
	QuestComplete(ctx context.Context, r Recipient, questID uint16)
	CharacterCreate(ctx context.Context, r Recipient)
}

// Entitlements 是一个 handler 收集到的**角色待遇**（存档字段，不是物品）。
//
// 为什么单独一类：装备栏挂锁、背包/时装栏扩容档位、复活币这些东西**不是物品**，
// `grant_item` / `send_mail` 碰不到它们 —— 客户端读的是角色 state 里的字段。
// 服务端把"改这些字段"做成一条**能力**，至于改成多少、给不给，由 mod 的脚本决定
// （业主 2026-10-06 口径：服务端只提供能力，内容归 mod）。
//
// 每个字段都是"零值/缺省即不改"的语义：脚本没调那条函数，就**不动**存档里原有的值 ——
// 绝不能用零值把玩家已有的待遇抹掉。
type Entitlements struct {
	// EquipSlotMask 是扩展装备栏挂锁位，按位或进 expand_equip_flags。0 = 不改。
	// 位定义见 inventory/expand_slots.go（support=1、magic stone=2、aura skin=8、
	// earring=16、creature skin=32；全开 = 59）。
	EquipSlotMask byte
	// BagTier / AvatarTier 是背包 / 时装栏扩容**档位**；nil = 不改。
	// 只在比现档更高时才写（扩容不可逆，往低改等于把玩家的格子抹掉）。
	BagTier    *byte
	AvatarTier *byte
	// ReviveCoins 是复活币增量。0 = 不改。
	ReviveCoins uint32

	// —— 2026-10-06 第二批：宠物 / 金库 / 账号材料仓 / 皮肤仓库 ——
	//
	// 说明：这批里有的是**角色 state 字段**（宠物本体、宠物用品），有的是**独立表**
	// （个人金库容量、账号金库容量、账号材料仓、账号皮肤仓库）。本结构只负责"收集意图"，
	// 落到哪张表由服务端的 EntitlementFunc 决定（applier 会分派到各自的事务入口）。
	// 同样遵守"零值/缺省即不改"。
	Pets     []ItemGrant `json:"pets,omitempty"`      // 宠物本体（模板，count 恒为 1）
	PetItems []ItemGrant `json:"petItems,omitempty"`  // 宠物用品（模板 + 数量）
	// Vault1Slots / Vault2Slots 是个人金库两层容量的目标值；AccountVaultSlots 是账号金库。
	// nil = 不改；只会往大改（只升不降）。
	Vault1Slots       *uint16 `json:"vault1Slots,omitempty"`
	Vault2Slots       *uint16 `json:"vault2Slots,omitempty"`
	AccountVaultSlots *uint16 `json:"accountVaultSlots,omitempty"`
	// AccountMaterialByTemplate 按**模板**聚合账号材料增量。
	//
	// 为什么按模板而不是按槽位：账号材料仓内部按固定**槽位**计数（inventory.AccountMaterialSlot
	// 只认 19 个模板 → 槽位映射），但脚本作者手里只有模板号 —— 让脚本去背槽位号是第二真源。
	// 所以脚本按模板累加，落库时由服务端映射到槽位，不在映射里的模板会被拒绝并点名。
	AccountMaterialByTemplate map[uint32]uint32 `json:"accountMaterialByTemplate,omitempty"`
	// UnlockSkins = true 表示把这批皮肤模板全部解锁（账号级）。
	UnlockSkins bool `json:"unlockSkins,omitempty"`
	// SkinTemplates 是要解锁的皮肤**来源模板**（缺省由服务端用运行期目录补全 = 全解锁）。
	SkinTemplates []uint32 `json:"skinTemplates,omitempty"`
}

// Empty 报告这批待遇什么都没改（调用方据此跳过整笔事务）。
func (e Entitlements) Empty() bool {
	return e.EquipSlotMask == 0 && e.BagTier == nil && e.AvatarTier == nil && e.ReviveCoins == 0 &&
		len(e.Pets) == 0 && len(e.PetItems) == 0 &&
		e.Vault1Slots == nil && e.Vault2Slots == nil && e.AccountVaultSlots == nil &&
		len(e.AccountMaterialByTemplate) == 0 && !e.UnlockSkins && len(e.SkinTemplates) == 0
}

// EntitlementFunc persists one batch of character entitlements under a stable key.
type EntitlementFunc func(ctx context.Context, r Recipient, key string, e Entitlements) error
